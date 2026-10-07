// Package builds and verifies a native EmbedLedger distribution.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	out := flag.String("out", "dist", "output directory for the native ZIP and checksum")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("package takes only --out")
	}
	version, err := releaseVersion()
	if err != nil {
		return err
	}
	output, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(output, 0755); err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "embedledger-package-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	name := fmt.Sprintf("embedledger_%s_%s_%s", version, runtime.GOOS, runtime.GOARCH)
	executable := "embedledger"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	binary := filepath.Join(work, executable)
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", binary, ".")
	build.Env = buildEnvironment()
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err = build.Run(); err != nil {
		return fmt.Errorf("build native executable: %w", err)
	}
	files := []string{"LICENSE", "README.md", "CHANGELOG.md", "go.mod", "embedledger.json"}
	for _, directory := range []string{"docs", "examples"} {
		err = filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() {
				if !entry.Type().IsRegular() {
					return fmt.Errorf("distribution file is not regular: %s", path)
				}
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	slices.Sort(files)
	archive := filepath.Join(output, name+".zip")
	if err = writeArchive(archive, name, executable, binary, files); err != nil {
		return err
	}
	data, err := os.ReadFile(archive)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	checksum := hex.EncodeToString(digest[:]) + "  " + filepath.Base(archive) + "\n"
	if err = os.WriteFile(archive+".sha256", []byte(checksum), 0644); err != nil {
		return err
	}
	unpacked := filepath.Join(work, "unpacked")
	if err = unpackArchive(archive, unpacked, name, executable); err != nil {
		return err
	}
	packageRoot := filepath.Join(unpacked, name)
	program := filepath.Join(packageRoot, executable)
	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"version"}, "embedledger " + version + "\n"},
		{[]string{"help"}, "EmbedLedger: review the files selected by go:embed."},
		{[]string{"check", "--goos", "linux", "--goarch", "amd64"}, "Embedded assets match the baseline (112 bytes).\n"},
	} {
		cmd := exec.Command(program, check.args...)
		cmd.Dir = packageRoot
		result, runErr := cmd.CombinedOutput()
		if runErr != nil || !strings.Contains(string(result), check.want) {
			return fmt.Errorf("unpacked package %v: %v: %s", check.args, runErr, result)
		}
		fmt.Printf("Verified unpacked package: %s\n", strings.Join(check.args, " "))
	}
	// Verification only reads the archive. Check the bytes before handing it to CI.
	unchanged, err := os.ReadFile(archive)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, unchanged) {
		return errors.New("archive changed during verification")
	}
	fmt.Printf("Native package: %s/%s\n%s", runtime.GOOS, runtime.GOARCH, checksum)
	return nil
}

func releaseVersion() (string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		return "", err
	}
	var version string
	for _, declaration := range file.Decls {
		constants, ok := declaration.(*ast.GenDecl)
		if !ok || constants.Tok != token.CONST {
			continue
		}
		for _, specification := range constants.Specs {
			value := specification.(*ast.ValueSpec)
			for index, identifier := range value.Names {
				if identifier.Name != "version" {
					continue
				}
				if version != "" || index >= len(value.Values) {
					return "", errors.New("version must have one string constant value")
				}
				literal, ok := value.Values[index].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return "", errors.New("version must be a string literal")
				}
				version, err = strconv.Unquote(literal.Value)
				if err != nil {
					return "", err
				}
			}
		}
	}
	if !regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`).MatchString(version) {
		return "", errors.New("version must be stable semantic version MAJOR.MINOR.PATCH")
	}
	return version, nil
}

func buildEnvironment() []string {
	replacements := map[string]string{"GOOS": runtime.GOOS, "GOARCH": runtime.GOARCH, "CGO_ENABLED": "0", "GOFLAGS": "", "GOTOOLCHAIN": "local", "GOWORK": "off"}
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replace := replacements[strings.ToUpper(key)]; !replace {
			env = append(env, entry)
		}
	}
	for key, value := range replacements {
		env = append(env, key+"="+value)
	}
	return env
}

func writeArchive(path, name, executable, binary string, files []string) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	w := zip.NewWriter(f)
	add := func(source, destination string, mode fs.FileMode) error {
		header := &zip.FileHeader{Name: name + "/" + filepath.ToSlash(destination), Method: zip.Deflate}
		header.SetMode(mode)
		entry, err := w.CreateHeader(header)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		_, err = entry.Write(data)
		return err
	}
	if err = add(binary, executable, 0755); err != nil {
		w.Close()
		return err
	}
	for _, file := range files {
		if err = add(file, file, 0644); err != nil {
			w.Close()
			return err
		}
	}
	return w.Close()
}

func unpackArchive(path, destination, name, executable string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()
	executableFound := false
	for _, entry := range r.File {
		if !strings.HasPrefix(entry.Name, name+"/") || !fs.ValidPath(entry.Name) || strings.Contains(entry.Name, "\\") || !entry.Mode().IsRegular() {
			return fmt.Errorf("invalid distribution entry: %s", entry.Name)
		}
		if entry.Name == name+"/"+executable {
			executableFound = true
			if entry.Mode().Perm() != 0755 {
				return errors.New("ZIP did not preserve executable permissions")
			}
		}
		target := filepath.Join(destination, filepath.FromSlash(entry.Name))
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, entry.Mode().Perm())
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(output, source)
		err = errors.Join(copyErr, source.Close(), output.Close())
		if err != nil {
			return err
		}
		if err = os.Chmod(target, entry.Mode().Perm()); err != nil {
			return err
		}
	}
	if !executableFound {
		return errors.New("distribution has no executable")
	}
	return nil
}
