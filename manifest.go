package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func baselinePath(dir, file string) string {
	if filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(dir, file)
}

func readManifest(path string) (manifest, error) {
	var m manifest
	f, err := os.Open(path)
	if err != nil {
		return m, fmt.Errorf("read baseline: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxMetadata+1))
	if err != nil {
		return m, err
	}
	if len(data) > maxMetadata {
		return m, errors.New("baseline exceeds 32 MiB")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&m); err != nil {
		return m, fmt.Errorf("invalid baseline: %w", err)
	}
	var extra any
	if err = dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return m, errors.New("baseline contains trailing JSON or invalid data")
	}
	if m.Schema != 1 || m.GoVersion == "" || m.Scope.GOOS == "" || m.Scope.GOARCH == "" || (m.Scope.CGO != "0" && m.Scope.CGO != "1") || len(m.Scope.Patterns) == 0 || m.Assets == nil || len(m.Assets) > maxAssets {
		return m, errors.New("unsupported or incomplete baseline; expected schema 1")
	}
	seen := map[string]bool{}
	var total int64
	for _, a := range m.Assets {
		hash, err := hex.DecodeString(a.SHA256)
		if err != nil || len(hash) != 32 || a.SHA256 != strings.ToLower(a.SHA256) || a.Bytes < 0 || a.Bytes > 1<<50 || a.Package == "" || strings.ContainsRune(a.Package, 0) || !fs.ValidPath(a.Path) || a.Path == "." || strings.ContainsAny(a.Path, "\\\x00") || (a.Kind != "production" && a.Kind != "test" && a.Kind != "external_test") {
			return m, errors.New("invalid asset record in baseline")
		}
		if seen[assetKey(a)] {
			return m, errors.New("duplicate asset record in baseline")
		}
		seen[assetKey(a)] = true
		if a.Bytes > (1<<50)-total {
			return m, errors.New("baseline total exceeds supported limit")
		}
		total += a.Bytes
	}
	if total != m.TotalBytes {
		return m, errors.New("baseline total_bytes does not match its assets")
	}
	return m, nil
}

func rejectEmbeddedBaseline(dir, file string, m manifest) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(baselinePath(dir, file))
	if err != nil {
		return err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = resolved
	}
	for _, a := range m.Assets {
		candidate := filepath.Join(root, filepath.FromSlash(a.Path))
		// EqualFold also catches aliases on the common case insensitive filesystems.
		if strings.EqualFold(candidate, path) {
			return errors.New("baseline is itself embedded; choose --file outside all embedded directories and patterns")
		}
	}
	return nil
}

func saveManifest(path string, m manifest, force bool) error {
	if info, err := os.Lstat(path); err == nil {
		if !force {
			return errors.New("baseline already exists; review changes with check, then use snapshot --force")
		}
		if !info.Mode().IsRegular() {
			return errors.New("baseline destination must be a regular file, not a symlink or directory")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var data bytes.Buffer
	if err := writeJSON(&data, m); err != nil {
		return err
	}
	if data.Len() > maxMetadata {
		return errors.New("baseline exceeds 32 MiB")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".embedledger-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if force {
		return os.Rename(tmp.Name(), path)
	}
	// Linking the completed temporary file prevents overwriting a destination
	// created between the existence check and publication.
	if err = os.Link(tmp.Name(), path); err != nil {
		return fmt.Errorf("publish baseline without overwrite: %w", err)
	}
	return nil
}

type change struct {
	Before asset `json:"before"`
	After  asset `json:"after"`
}

type difference struct {
	OK          bool     `json:"ok"`
	Added       []asset  `json:"added"`
	Removed     []asset  `json:"removed"`
	Changed     []change `json:"changed"`
	BeforeBytes int64    `json:"before_bytes"`
	AfterBytes  int64    `json:"after_bytes"`
}

func compare(before, after manifest) difference {
	d := difference{Added: []asset{}, Removed: []asset{}, Changed: []change{}, BeforeBytes: before.TotalBytes, AfterBytes: after.TotalBytes}
	old := map[string]asset{}
	for _, a := range before.Assets {
		old[assetKey(a)] = a
	}
	for _, a := range after.Assets {
		b, found := old[assetKey(a)]
		if !found {
			d.Added = append(d.Added, a)
		} else if a.Bytes != b.Bytes || a.SHA256 != b.SHA256 {
			d.Changed = append(d.Changed, change{b, a})
		}
		delete(old, assetKey(a))
	}
	for _, a := range old {
		d.Removed = append(d.Removed, a)
	}
	sortAssets := func(a, b asset) int { return strings.Compare(assetKey(a), assetKey(b)) }
	slices.SortFunc(d.Added, sortAssets)
	slices.SortFunc(d.Removed, sortAssets)
	slices.SortFunc(d.Changed, func(a, b change) int { return sortAssets(a.After, b.After) })
	d.OK = len(d.Added)+len(d.Removed)+len(d.Changed) == 0
	return d
}

func printDiff(out io.Writer, d difference) error {
	var b strings.Builder
	if d.OK {
		fmt.Fprintf(&b, "Embedded assets match the baseline (%d bytes).\n", d.AfterBytes)
	} else {
		for _, a := range d.Added {
			fmt.Fprintf(&b, "ADDED    %s %q (%d bytes)\n", a.Kind, a.Path, a.Bytes)
		}
		for _, a := range d.Removed {
			fmt.Fprintf(&b, "REMOVED  %s %q (%d bytes)\n", a.Kind, a.Path, a.Bytes)
		}
		for _, c := range d.Changed {
			fmt.Fprintf(&b, "CHANGED  %s %q (%d -> %d bytes)\n", c.After.Kind, c.After.Path, c.Before.Bytes, c.After.Bytes)
		}
		fmt.Fprintf(&b, "%d added, %d removed, %d changed; %d -> %d bytes.\n", len(d.Added), len(d.Removed), len(d.Changed), d.BeforeBytes, d.AfterBytes)
	}
	_, err := io.WriteString(out, b.String())
	return err
}
