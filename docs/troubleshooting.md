# Troubleshooting

[Back to the README](../README.md)

## Command not found

First run `go version`. If Go is missing or older than 1.27, [install a supported toolchain](https://go.dev/doc/install) and reopen your terminal.

If Go works but `embedledger` is missing after installation, add Go's executable installation directory to PATH. Go uses `GOBIN` when configured, otherwise the `bin` directory under the first `GOPATH` entry. These commands update only the current terminal session.

### Windows PowerShell

```powershell
$goBin = go env GOBIN
if (-not $goBin) {
    $goWorkspace = (go env GOPATH) -split [IO.Path]::PathSeparator
    $goBin = Join-Path $goWorkspace[0] 'bin'
}
$env:PATH = "$goBin;$env:PATH"
embedledger version
```

For future terminals, add the printed value of `$goBin` to your user Path through Windows Environment Variables, then open a new terminal.

### macOS and Linux

For Bash or Zsh:

```sh
go_bin="$(go env GOBIN)"
if [ -z "$go_bin" ]; then
  go_workspace="$(go env GOPATH)"
  go_bin="${go_workspace%%:*}/bin"
fi
export PATH="$go_bin:$PATH"
embedledger version
```

For future terminals, add the PATH configuration to your shell startup file. See Go's [installation directory guidance](https://go.dev/doc/tutorial/compile-install).

## Module root or missing dependencies

Run from the directory containing your project's `go.mod`, or pass `--dir` on each command:

```sh
embedledger scan --dir /path/to/your/module --goos linux --goarch amd64
```

Replace the example path with your module's actual path. EmbedLedger disables workspace mode and scans one module at a time.

Nested modules need separate scans from their own module roots. A parent module's `./...` does not enter a nested module, and Go also excludes directories such as `testdata` from that wildcard. Select those packages explicitly when needed, for example `embedledger scan --tests . ./testdata`. An empty inventory can be correct for the selected scope.

Prepare dependencies before scanning, usually with `go mod download` from that module. EmbedLedger uses readonly module resolution and disables downloads during scanning. If Go requests changes to `go.mod` or `go.sum`, resolve and review those changes through your normal build workflow first. Private dependencies may need your normal Go authentication setup.

## Baseline is missing or already exists

Create the first baseline with `snapshot`. The default file is `embedledger.json`, relative to the module directory. If you use `--file`, pass the same file when checking. The destination's parent directory must already exist.

When the baseline already exists, run `check` and review the reported changes. Use `snapshot --force` only when you intend to accept the new inventory; review and commit the resulting JSON diff.

## Baseline scope differs

Use the same `--goos`, `--goarch`, `--cgo`, `--tags`, package arguments, and `--tests` settings as the snapshot. The baseline's `scope` object records them. Host OS and architecture are the defaults, so a baseline created on Windows without target flags will not match a default Linux scan.

For separate build targets, keep separate baseline files. See the [target examples](reference.md#commands-and-options).

## Baseline is itself embedded

Move the baseline outside every selected embed pattern and pass its new location with `--file` on snapshot and check. This can happen with broad patterns such as `*` or `all:` even if the file is in the module root. If the baseline is newly created, it may be rejected on the next scan when Go first selects it.

## Exit status 1 or 2

Status `1` means the scan completed and found drift. Inspect the diff before accepting it. Status `2` means no complete comparison was produced; read stderr for invalid input, missing dependencies, target mismatch, or resource limits.

Deleting a file named by a literal `//go:embed config.yml` directive makes Go's embed pattern invalid and returns status `2`. A removed file is reported as drift only when Go can still resolve the package, such as a wildcard that continues to match other files.

In PowerShell, inspect `$LASTEXITCODE` immediately after the command; in Bash or Zsh, inspect `$?`. For JSON consumers, use stdout for the report and stderr for errors.

Use the installed or compiled executable in automation. `go run . check` wraps the program's exit status and does not preserve the CLI's distinct error statuses for your shell.

## File budget, timeout, or filesystem error

The default asset byte budget is 1 GiB. Set `--max-bytes` explicitly if your project needs a different budget; it may differ between snapshot and check.

The default deadline is 60 seconds. A cold Go build cache can require more time even after module downloads finish. If stderr reports `context deadline exceeded`, retry with a longer deadline:

```sh
embedledger scan --timeout 5m --goos linux --goarch amd64
```

`--timeout` accepts positive Go durations up to `1h`, applies to scan, snapshot, and check, and can differ from the timeout used to save a baseline. Metadata and record limits remain fixed; narrow package selection for very large modules.

Initial baseline publication requires a filesystem that supports hard links. If it fails, use a supported local filesystem. Scans should run against a stable checkout; retry after concurrent editors or generators finish. See the [full boundaries](reference.md#boundaries).
