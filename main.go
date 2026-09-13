// EmbedLedger records and compares the assets selected by Go's embed resolver.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"
)

const version = "0.1.0"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(out, `EmbedLedger: review the files selected by go:embed.

Usage: embedledger <scan|snapshot|check> [flags] [./package ...]
       embedledger version

scan prints the current inventory; snapshot saves a baseline; check reports drift.
Default package selection: ./... in the selected module. Dependencies are excluded.
Run a subcommand with --help to see its flags.
Exit codes: 0 success, 1 inventory drift, 2 invalid input or incomplete scan.`)
		return 0
	}
	if args[0] == "version" && len(args) == 1 {
		fmt.Fprintln(out, "embedledger "+version)
		return 0
	}
	command := args[0]
	if command != "scan" && command != "snapshot" && command != "check" {
		fmt.Fprintf(errOut, "unknown command %q; use embedledger help\n", command)
		return 2
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	var opt options
	fs.StringVar(&opt.Dir, "dir", ".", "module directory (must contain go.mod)")
	fs.StringVar(&opt.GOOS, "goos", runtime.GOOS, "target operating system")
	fs.StringVar(&opt.GOARCH, "goarch", runtime.GOARCH, "target architecture")
	fs.StringVar(&opt.CGO, "cgo", "0", "CGO_ENABLED: 0 or 1")
	fs.StringVar(&opt.Tags, "tags", "", "comma separated build tags")
	fs.BoolVar(&opt.Tests, "tests", false, "also inventory internal and external test assets")
	fs.Int64Var(&opt.MaxBytes, "max-bytes", 1<<30, "maximum total asset bytes to read (default 1 GiB)")
	file := fs.String("file", "embedledger.json", "baseline path, relative to --dir")
	asJSON := fs.Bool("json", false, "print structured JSON")
	force := fs.Bool("force", false, "allow snapshot to replace an existing baseline")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *force && command != "snapshot" {
		fmt.Fprintln(errOut, "--force is only valid for snapshot")
		return 2
	}
	opt.Patterns = fs.Args()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var previous manifest
	if command == "check" {
		var err error
		previous, err = readManifest(baselinePath(opt.Dir, *file))
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
	}
	current, err := collect(ctx, opt)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if command != "scan" {
		if err = rejectEmbeddedBaseline(opt.Dir, *file, current); err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
	}
	switch command {
	case "snapshot":
		err = saveManifest(baselinePath(opt.Dir, *file), current, *force)
		if err == nil {
			if *asJSON {
				err = writeJSON(out, current)
			} else {
				_, err = fmt.Fprintf(out, "Saved %d assets (%d bytes) to %q\nReview this file before committing it.\n", len(current.Assets), current.TotalBytes, *file)
			}
		}
	case "scan":
		if *asJSON {
			err = writeJSON(out, current)
		} else {
			for _, a := range current.Assets {
				fmt.Fprintf(out, "%s  %8d  %s  %q\n", a.Kind, a.Bytes, a.SHA256[:12], a.Path)
			}
			_, err = fmt.Fprintf(out, "%d assets, %d bytes, %s/%s, CGO=%s\n", len(current.Assets), current.TotalBytes, current.Scope.GOOS, current.Scope.GOARCH, current.Scope.CGO)
		}
	case "check":
		if !sameScope(previous.Scope, current.Scope) {
			fmt.Fprintln(errOut, "baseline scope differs: use the same packages, target, tags and --tests setting, or review a new snapshot")
			return 2
		}
		diff := compare(previous, current)
		if *asJSON {
			err = writeJSON(out, diff)
		} else {
			err = printDiff(out, diff)
		}
		if err == nil && !diff.OK {
			return 1
		}
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	return 0
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
