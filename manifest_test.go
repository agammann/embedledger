package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validManifest() manifest {
	return manifest{Schema: 1, GoVersion: "go1.25.0", Scope: scope{GOOS: "linux", GOARCH: "amd64", CGO: "0", Patterns: []string{"./..."}, Tags: []string{}}, Assets: []asset{{Package: "example.com/a", Kind: "production", Path: "assets/a.txt", Bytes: 3, SHA256: strings.Repeat("a", 64)}}, TotalBytes: 3}
}

func TestManifestValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*manifest)
	}{
		{"schema", func(m *manifest) { m.Schema = 2 }},
		{"missing_assets", func(m *manifest) { m.Assets = nil }},
		{"duplicate", func(m *manifest) { m.Assets = append(m.Assets, m.Assets[0]); m.TotalBytes = 6 }},
		{"digest", func(m *manifest) { m.Assets[0].SHA256 = "oops" }},
		{"path", func(m *manifest) { m.Assets[0].Path = "../secret" }},
		{"kind", func(m *manifest) { m.Assets[0].Kind = "invalid" }},
		{"size", func(m *manifest) { m.Assets[0].Bytes = -1 }},
		{"total", func(m *manifest) { m.TotalBytes = 4 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			tc.edit(&m)
			data, _ := json.Marshal(m)
			p := filepath.Join(t.TempDir(), "baseline.json")
			if err := os.WriteFile(p, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := readManifest(p); err == nil {
				t.Fatal("invalid baseline accepted")
			}
		})
	}
	for _, suffix := range []string{"{}", "garbage"} {
		m := validManifest()
		data, _ := json.Marshal(m)
		p := filepath.Join(t.TempDir(), "baseline.json")
		if err := os.WriteFile(p, append(data, []byte(suffix)...), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readManifest(p); err == nil {
			t.Fatal("trailing data accepted")
		}
	}
}

func TestSavePreservesBaselineAndRoundTrips(t *testing.T) {
	p := filepath.Join(t.TempDir(), "baseline.json")
	m := validManifest()
	if err := saveManifest(p, m, false); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	m.TotalBytes = 7
	if err := saveManifest(p, m, false); err == nil {
		t.Fatal("overwrote existing baseline")
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("existing baseline changed")
	}
	loaded, err := readManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if !compare(validManifest(), loaded).OK {
		t.Fatal("round trip mismatch")
	}
	if err := saveManifest(filepath.Dir(p), m, true); err == nil {
		t.Fatal("directory accepted")
	}
}

func TestDeterministicDiffAndOutput(t *testing.T) {
	before := validManifest()
	after := validManifest()
	after.Assets[0].SHA256 = strings.Repeat("b", 64)
	d := compare(before, after)
	if d.OK || len(d.Changed) != 1 {
		t.Fatal(d)
	}
	var out bytes.Buffer
	if err := printDiff(&out, d); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "CHANGED") {
		t.Fatal(out.String())
	}
	for _, args := range [][]string{nil, {"help"}, {"version"}, {"scan", "--help"}} {
		if code := run(args, &out, &out); code != 0 {
			t.Fatal(args, code)
		}
	}
	for _, args := range [][]string{{"unknown"}, {"scan", "--unknown"}} {
		if code := run(args, &out, &out); code != 2 {
			t.Fatal(args, code)
		}
	}
}

func TestMetadataLimit(t *testing.T) {
	b := boundedBuffer{limit: 4}
	if _, err := b.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("5")); err == nil {
		t.Fatal("limit bypassed")
	}
	if b.String() != "1234" {
		t.Fatal("unexpected partial write")
	}
}
