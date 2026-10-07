package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
)

func writeFixture(t *testing.T, dir, path, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T, pattern string) (string, options) {
	t.Helper()
	dir := t.TempDir()
	writeFixture(t, dir, "go.mod", "module example.com/fixture\n\ngo 1.25.0\n")
	writeFixture(t, dir, "assets.go", "package fixture\nimport \"embed\"\n//go:embed "+pattern+"\nvar Assets embed.FS\n")
	writeFixture(t, dir, "assets/a.txt", "alpha")
	writeFixture(t, dir, "assets/nested/b.txt", "beta")
	writeFixture(t, dir, "assets/.hidden", "hidden")
	writeFixture(t, dir, "assets/nested/_private", "private")
	return dir, options{Dir: dir, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, CGO: "0", MaxBytes: 1 << 20}
}

func TestGoResolverSemantics(t *testing.T) {
	for _, tc := range []struct {
		pattern  string
		expected []string
	}{
		{"assets", []string{"assets/a.txt", "assets/nested/b.txt"}},
		{"assets/*", []string{"assets/.hidden", "assets/a.txt", "assets/nested/b.txt"}},
		{"all:assets", []string{"assets/.hidden", "assets/a.txt", "assets/nested/_private", "assets/nested/b.txt"}},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			_, opt := fixture(t, tc.pattern)
			m, err := collect(context.Background(), opt)
			if err != nil {
				t.Fatal(err)
			}
			var paths []string
			for _, a := range m.Assets {
				paths = append(paths, a.Path)
			}
			if !slices.Equal(paths, tc.expected) {
				t.Fatalf("got %v; want %v", paths, tc.expected)
			}
			again, err := collect(context.Background(), opt)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(m)
			b, _ := json.Marshal(again)
			if !bytes.Equal(a, b) {
				t.Fatal("inventory is not deterministic")
			}
		})
	}
}

func TestGoResolverThroughModuleAlias(t *testing.T) {
	dir, opt := fixture(t, "assets")
	alias := filepath.Join(t.TempDir(), "module")
	if err := os.Symlink(dir, alias); err != nil {
		// Windows ERROR_PRIVILEGE_NOT_HELD: symlink creation needs permission.
		if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(1314)) {
			t.Skipf("symlink privilege unavailable: %v", err)
		}
		t.Fatal(err)
	}
	opt.Dir = alias
	// Unix Go subprocesses may retain a valid PWD alias in package metadata.
	t.Setenv("PWD", alias)
	m, err := collect(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Assets) != 2 || m.TotalBytes != 9 || m.Assets[0].Path != "assets/a.txt" || m.Assets[1].Path != "assets/nested/b.txt" {
		t.Fatalf("unexpected inventory through module alias: %+v", m)
	}
	// A package alias that resolves outside the module must still fail.
	outside := t.TempDir()
	writeFixture(t, outside, "outside.go", "package outside\n")
	if err := os.Symlink(outside, filepath.Join(dir, "outside")); err != nil {
		t.Fatal(err)
	}
	opt.Patterns = []string{"./outside"}
	if _, err := collect(context.Background(), opt); err == nil || !strings.Contains(err.Error(), "outside the selected module") {
		t.Fatalf("expected outside-module rejection, got %v", err)
	}
}

func TestBuildContextsAndTestAssets(t *testing.T) {
	dir, opt := fixture(t, "assets")
	writeFixture(t, dir, "special.go", "//go:build special\n\npackage fixture\nimport _ \"embed\"\n//go:embed special.txt\nvar Special string\n")
	writeFixture(t, dir, "special.txt", "special")
	writeFixture(t, dir, "internal_test.go", "package fixture\nimport _ \"embed\"\n//go:embed internal.txt\nvar Internal string\n")
	writeFixture(t, dir, "internal.txt", "internal")
	writeFixture(t, dir, "external_test.go", "package fixture_test\nimport _ \"embed\"\n//go:embed external.txt\nvar External string\n")
	writeFixture(t, dir, "external.txt", "external")
	writeFixture(t, dir, "windows.go", "//go:build windows\n\npackage fixture\nimport _ \"embed\"\n//go:embed windows.txt\nvar Windows string\n")
	writeFixture(t, dir, "windows.txt", "windows")
	opt.GOOS = "linux"
	base, err := collect(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(base.Assets) != 2 {
		t.Fatalf("unexpected production assets: %+v", base.Assets)
	}
	opt.Tags, opt.Tests, opt.GOOS = "special,special", true, "windows"
	m, err := collect(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Assets) != 6 {
		t.Fatalf("wanted 6 assets, got %+v", m.Assets)
	}
	kinds := map[string]int{}
	for _, a := range m.Assets {
		kinds[a.Kind]++
	}
	if kinds["production"] != 4 || kinds["test"] != 1 || kinds["external_test"] != 1 {
		t.Fatal(kinds)
	}
	if !slices.Equal(m.Scope.Tags, []string{"special"}) {
		t.Fatal(m.Scope.Tags)
	}
}

func TestNoPartialSuccess(t *testing.T) {
	dir, opt := fixture(t, "assets")
	opt.MaxBytes = 3
	if _, err := collect(context.Background(), opt); err == nil {
		t.Fatal("budget must fail")
	}
	opt.MaxBytes = 1 << 20
	opt.Patterns = []string{"./missing/..."}
	if _, err := collect(context.Background(), opt); err == nil {
		t.Fatal("no packages must fail")
	}
	opt.Patterns = []string{".", "./missing/..."}
	if _, err := collect(context.Background(), opt); err == nil {
		t.Fatal("a valid package must not hide an unmatched package pattern")
	}
	opt.Patterns = []string{"./../..."}
	if _, err := collect(context.Background(), opt); err == nil {
		t.Fatal("parent traversal must fail")
	}
	opt.Patterns = nil
	writeFixture(t, dir, "bad/bad.go", "package bad\nimport _ \"embed\"\n//go:embed nonexistent\nvar Missing string\n")
	if _, err := collect(context.Background(), opt); err == nil {
		t.Fatal("invalid sibling package must fail the entire scan")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := collect(ctx, opt); err == nil {
		t.Fatal("cancellation must fail")
	}
}

func TestCLIWorkflow(t *testing.T) {
	dir, _ := fixture(t, "assets")
	call := func(command string, expected int, extra ...string) string {
		t.Helper()
		args := append([]string{command, "--dir", dir}, extra...)
		var out, stderr bytes.Buffer
		if got := run(args, &out, &stderr); got != expected {
			t.Fatalf("%v: exit %d, want %d\n%s\n%s", args, got, expected, &out, &stderr)
		}
		return out.String() + stderr.String()
	}
	call("snapshot", 0)
	baseline, err := os.ReadFile(filepath.Join(dir, "embedledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	call("snapshot", 2)
	call("check", 0)
	writeFixture(t, dir, "assets/a.txt", "ALPHA") // Same byte count must still detect content drift.
	writeFixture(t, dir, "assets/new.txt", "new")
	if err := os.Remove(filepath.Join(dir, "assets/nested/b.txt")); err != nil {
		t.Fatal(err)
	}
	result := call("check", 1, "--json")
	var d difference
	if err := json.Unmarshal([]byte(result), &d); err != nil {
		t.Fatal(err)
	}
	if d.OK || len(d.Added) != 1 || len(d.Removed) != 1 || len(d.Changed) != 1 {
		t.Fatalf("unexpected diff: %+v", d)
	}
	unchanged, _ := os.ReadFile(filepath.Join(dir, "embedledger.json"))
	if !bytes.Equal(baseline, unchanged) {
		t.Fatal("check mutated baseline")
	}
	if !strings.Contains(call("check", 2, "--tests"), "scope differs") {
		t.Fatal("context mismatch not reported")
	}
	call("snapshot", 0, "--force")
	call("check", 0)
	call("scan", 2, "--force")
	call("check", 2, "--max-bytes", "1")
	call("snapshot", 0, "--file", "assets/ledger.json")
	if !strings.Contains(call("check", 2, "--file", "assets/ledger.json"), "itself embedded") {
		t.Fatal("self inclusion was not caught")
	}
}

func TestRealBinaryExitCodes(t *testing.T) {
	dir, _ := fixture(t, "assets")
	binary := filepath.Join(t.TempDir(), "embedledger")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, ".")
	if data, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, data)
	}
	invoke := func(command string, want int) {
		t.Helper()
		cmd := exec.Command(binary, command, "--dir", dir)
		data, err := cmd.CombinedOutput()
		got := 0
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				got = exit.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if got != want {
			t.Fatalf("%s exit=%d want=%d: %s", command, got, want, data)
		}
	}
	invoke("snapshot", 0)
	invoke("check", 0)
	writeFixture(t, dir, "assets/extra.txt", "unreviewed")
	invoke("check", 1)
	if err := os.WriteFile(filepath.Join(dir, "embedledger.json"), []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}
	invoke("check", 2)
}

func TestEnvironmentAndScope(t *testing.T) {
	t.Setenv("GOFLAGS", "-tags=surprise")
	t.Setenv("GOWORK", "/nonexistent/go.work")
	_, opt := fixture(t, "assets")
	if _, err := collect(context.Background(), opt); err != nil {
		t.Fatal(err)
	}
	a := scope{GOOS: "linux", GOARCH: "amd64", CGO: "0", Patterns: []string{"./..."}}
	b := a
	b.GOARCH = "arm64"
	if sameScope(a, b) {
		t.Fatal("architectures must not compare equal")
	}
}

func TestRootRejectsEscapingSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.txt")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, _, err = hashAsset(context.Background(), root, "link.txt", 100); err == nil {
		t.Fatal("escaped module root")
	}
}
