package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const maxMetadata = 32 << 20
const maxAssets = 100000

type options struct {
	Dir, GOOS, GOARCH, CGO, Tags string
	Tests                        bool
	Patterns                     []string
	MaxBytes                     int64
}

type scope struct {
	GOOS     string   `json:"goos"`
	GOARCH   string   `json:"goarch"`
	CGO      string   `json:"cgo"`
	Tags     []string `json:"tags"`
	Tests    bool     `json:"tests"`
	Patterns []string `json:"packages"`
}

type asset struct {
	Package string `json:"package"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Bytes   int64  `json:"bytes"`
	SHA256  string `json:"sha256"`
}

type manifest struct {
	Schema     int     `json:"schema"`
	GoVersion  string  `json:"go_version"`
	Scope      scope   `json:"scope"`
	Assets     []asset `json:"assets"`
	TotalBytes int64   `json:"total_bytes"`
}

type goPackage struct {
	Dir, ImportPath                             string
	ForTest                                     string
	EmbedFiles, TestEmbedFiles, XTestEmbedFiles []string
	Error                                       *struct{ Err string }
}

// boundedBuffer stops excessive command output instead of retaining it in memory.
type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("Go output exceeded metadata limit")
	}
	return b.Buffer.Write(p)
}

func goCommand(ctx context.Context, dir string, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir, cmd.Env = dir, env
	out := &boundedBuffer{limit: maxMetadata}
	stderr := &boundedBuffer{limit: 64 << 10}
	cmd.Stdout, cmd.Stderr = out, stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("go %s interrupted: %w", args[0], ctx.Err())
		}
		return nil, fmt.Errorf("go %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	if args[0] == "list" && strings.Contains(stderr.String(), "matched no packages") {
		return nil, fmt.Errorf("incomplete package selection: %s", strings.TrimSpace(stderr.String()))
	}
	return out.Bytes(), nil
}

func controlledEnv(opt options) []string {
	overrides := map[string]string{
		"GOOS": opt.GOOS, "GOARCH": opt.GOARCH, "CGO_ENABLED": opt.CGO,
		"GOFLAGS": "", "GOWORK": "off", "GOTOOLCHAIN": "local",
		"GOPROXY": "off", "GOSUMDB": "off", "GO111MODULE": "on",
	}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, s := range os.Environ() {
		key, _, _ := strings.Cut(s, "=")
		if _, replace := overrides[strings.ToUpper(key)]; !replace {
			env = append(env, s)
		}
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}

func collect(ctx context.Context, opt options) (manifest, error) {
	m := manifest{Schema: 1, Assets: []asset{}}
	if opt.CGO != "0" && opt.CGO != "1" {
		return m, errors.New("--cgo must be 0 or 1")
	}
	if opt.MaxBytes <= 0 || opt.MaxBytes > 1<<50 {
		return m, errors.New("--max-bytes must be between 1 and 1125899906842624")
	}
	root, err := filepath.Abs(opt.Dir)
	if err != nil {
		return m, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return m, err
	}
	info, err := os.Stat(filepath.Join(root, "go.mod"))
	if err != nil || !info.Mode().IsRegular() {
		return m, errors.New("--dir must be a module root containing a regular go.mod file")
	}
	patterns := slices.Clone(opt.Patterns)
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	for _, p := range patterns {
		if p != "." && !strings.HasPrefix(p, "./") {
			return m, fmt.Errorf("package %q must be . or start with ./", p)
		}
		for _, component := range strings.Split(p, "/") {
			if component == ".." || strings.ContainsAny(component, "\\@") {
				return m, fmt.Errorf("invalid local package pattern %q", p)
			}
		}
	}
	slices.Sort(patterns)
	patterns = slices.Compact(patterns)
	tags := strings.FieldsFunc(opt.Tags, func(r rune) bool { return r == ',' || r == ' ' })
	slices.Sort(tags)
	tags = slices.Compact(tags)
	if tags == nil {
		tags = []string{}
	}
	m.Scope = scope{opt.GOOS, opt.GOARCH, opt.CGO, tags, opt.Tests, patterns}
	env := controlledEnv(opt)
	data, err := goCommand(ctx, root, env, "env", "GOVERSION")
	if err != nil {
		return m, err
	}
	m.GoVersion = strings.TrimSpace(string(data))
	args := []string{"list", "-mod=readonly", "-json", "-tags=" + strings.Join(tags, ",")}
	if opt.Tests {
		args = append(args, "-test")
	}
	args = append(args, patterns...)
	data, err = goCommand(ctx, root, env, args...)
	if err != nil {
		return m, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	seen := map[string]bool{}
	count := 0
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return m, err
	}
	defer rootFS.Close()
	for {
		var pkg goPackage
		if err := decoder.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return m, fmt.Errorf("decode go list: %w", err)
		}
		count++
		if pkg.Error != nil {
			return m, errors.New(pkg.Error.Err)
		}
		// With -test, Go emits synthetic variants as well as the original
		// package. Its original now contains the resolved test asset lists.
		if pkg.ForTest != "" {
			continue
		}
		// Go can report another filesystem alias for the same directory.
		// Resolve it, as we did the module root, before checking containment.
		packageDir, err := filepath.EvalSymlinks(pkg.Dir)
		if err != nil {
			return m, fmt.Errorf("resolve package %q directory: %w", pkg.ImportPath, err)
		}
		dir, err := filepath.Rel(root, packageDir)
		if err != nil || !filepath.IsLocal(dir) {
			return m, fmt.Errorf("package %q is outside the selected module", pkg.ImportPath)
		}
		groups := []struct {
			kind  string
			names []string
		}{{"production", pkg.EmbedFiles}}
		if opt.Tests {
			groups = append(groups, struct {
				kind  string
				names []string
			}{"test", pkg.TestEmbedFiles}, struct {
				kind  string
				names []string
			}{"external_test", pkg.XTestEmbedFiles})
		}
		for _, group := range groups {
			for _, name := range group.names {
				if !fs.ValidPath(name) || strings.Contains(name, "\\") {
					return m, fmt.Errorf("invalid embedded path %q", name)
				}
				rel := filepath.ToSlash(filepath.Join(dir, filepath.FromSlash(name)))
				a := asset{Package: pkg.ImportPath, Kind: group.kind, Path: rel}
				if seen[assetKey(a)] {
					continue
				}
				seen[assetKey(a)] = true
				if len(seen) > maxAssets {
					return m, errors.New("asset count exceeds 100000")
				}
				if err := ctx.Err(); err != nil {
					return m, err
				}
				a.Bytes, a.SHA256, err = hashAsset(ctx, rootFS, rel, opt.MaxBytes-m.TotalBytes)
				if err != nil {
					return m, fmt.Errorf("asset %q: %w", rel, err)
				}
				m.TotalBytes += a.Bytes
				m.Assets = append(m.Assets, a)
			}
		}
	}
	if count == 0 {
		return m, errors.New("no packages matched; refusing to report a successful empty scan")
	}
	slices.SortFunc(m.Assets, func(a, b asset) int { return strings.Compare(assetKey(a), assetKey(b)) })
	return m, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func hashAsset(ctx context.Context, root *os.Root, path string, remaining int64) (int64, string, error) {
	f, err := root.Open(filepath.FromSlash(path))
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return 0, "", err
	}
	if !before.Mode().IsRegular() {
		return 0, "", errors.New("not a regular file")
	}
	if before.Size() > remaining {
		return 0, "", errors.New("total asset bytes exceed --max-bytes")
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(contextReader{ctx, f}, remaining+1))
	if err != nil {
		return 0, "", err
	}
	if n > remaining {
		return 0, "", errors.New("total asset bytes exceed --max-bytes")
	}
	after, err := f.Stat()
	if err != nil {
		return 0, "", err
	}
	if n != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return 0, "", errors.New("file changed while scanning; retry on a stable tree")
	}
	return n, hex.EncodeToString(hash.Sum(nil)), nil
}

func assetKey(a asset) string { return a.Package + "\x00" + a.Kind + "\x00" + a.Path }
func sameScope(a, b scope) bool {
	return a.GOOS == b.GOOS && a.GOARCH == b.GOARCH && a.CGO == b.CGO && a.Tests == b.Tests && slices.Equal(a.Tags, b.Tags) && slices.Equal(a.Patterns, b.Patterns)
}
