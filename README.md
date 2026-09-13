# EmbedLedger

[![CI](https://github.com/agammann/embedledger/actions/workflows/ci.yml/badge.svg)](https://github.com/agammann/embedledger/actions/workflows/ci.yml)

**Review what `go:embed` puts into your build.**

EmbedLedger saves the paths, sizes, and SHA256 hashes of embedded assets to a JSON baseline. It reports additions, removals, and content changes, and returns a failing exit code when that inventory changes.

Drop a forgotten export into an embedded directory and your Go build can still succeed. EmbedLedger makes that new file visible in CI.

```text
ADDED    production "examples/site/assets/export.csv" (1048576 bytes)
1 added, 0 removed, 0 changed; 112 -> 1048688 bytes.
```

Illustrative output. The automated tests exercise this behavior with disposable files.

One Go binary. Standard library only. No account, service, or configuration language.

## Install

Requires Go 1.27 or newer. The Go command must remain on PATH when running EmbedLedger because it resolves the actual embed inputs.

```sh
go install github.com/agammann/embedledger@v0.1.0
embedledger version
```

Version 0.1.0 is the first release. See the [release notes](https://github.com/agammann/embedledger/releases/tag/v0.1.0) and [changelog](CHANGELOG.md). Use `@latest` instead of `@v0.1.0` to install the newest published version.

Or build the checkout:

```sh
git clone https://github.com/agammann/embedledger.git
cd embedledger
go build -o dist/embedledger .
```

On Windows, use `go build -o dist/embedledger.exe .`.

## Start in your Go module

```sh
# Inspect the current files.
embedledger scan

# Save a baseline, then review and commit the JSON file.
embedledger snapshot

# Detect additions, removals, and changed bytes.
embedledger check

# After reviewing an intentional change, update the baseline explicitly.
embedledger snapshot --force
```

`snapshot` refuses to replace an existing file without `--force`. `check` never changes your baseline or assets. Store the baseline outside all embedded directories and patterns. If a newly created baseline gets picked up by a broad pattern, the next `check` rejects it and tells you to move it.

The default baseline is `embedledger.json` in the module root. JSON stores paths and hashes, never asset contents. Paths themselves may reveal information, so review the baseline before publishing it.

## Try the included example

From this repository, the committed baseline checks the example HTML asset for a fixed Linux target:

```sh
go run . scan --goos linux --goarch amd64
go run . check --goos linux --goarch amd64
```

Edit `examples/site/assets/index.html`, or add a file inside that asset directory. Run the second command again to see the drift. The compiled `embedledger check` command exits with status 1. When invoked through `go run`, Go wraps a failing program and prints its exit status; use the installed binary when scripts need to distinguish status 1 from status 2.

To see the embedded page's contents:

```sh
go run ./examples/site
```

## Commands and options

| Command | Result |
| :--- | :--- |
| `scan` | Print the resolved asset inventory |
| `snapshot` | Save the inventory as a baseline |
| `check` | Compare the current inventory to a baseline |
| `version` | Print the CLI version |

| Option | Default | Meaning |
| :--- | :--- | :--- |
| `--dir` | `.` | Module root containing `go.mod` |
| `--file` | `embedledger.json` | Baseline path relative to `--dir`, or an absolute path |
| `--json` | false | Structured inventory or comparison output |
| `--goos` | Host OS | Target operating system |
| `--goarch` | Host architecture | Target architecture |
| `--cgo` | `0` | Explicit CGO setting, `0` or `1` |
| `--tags` | Empty | Comma separated build tags |
| `--tests` | false | Include internal and external test assets |
| `--max-bytes` | `1073741824` | Maximum total asset bytes read per scan |
| `--force` | false | Permit replacing a baseline with `snapshot` |

Put flags before package arguments. Package arguments must be `.` or local patterns starting with `./`. The default is `./...`.

```sh
embedledger scan --json ./internal/web ./templates
embedledger snapshot --tags enterprise --goos linux --goarch arm64 --file embedledger.arm64.json
embedledger check --tags enterprise --goos linux --goarch arm64 --file embedledger.arm64.json
embedledger scan --tests --max-bytes 10485760
```

Use the same target, CGO setting, tags, package patterns, and test setting for snapshot and check. Different scopes return an error rather than a misleading diff. The byte limit is an execution budget and may differ between those commands.

| Exit code | Meaning |
| :--- | :--- |
| `0` | Inventory printed, baseline saved, or baseline matched |
| `1` | Added, removed, or changed assets |
| `2` | Invalid input, incompatible baseline, exceeded budget, or incomplete scan |

JSON diffs always contain `ok`, `added`, `removed`, `changed`, `before_bytes`, and `after_bytes`. Each changed item contains complete `before` and `after` records. Errors go to stderr; a failed scan does not emit a successful JSON report.

## Add to CI

Choose a target explicitly so developer machines and CI compare the same scope. Commit the reviewed baseline, install a pinned EmbedLedger revision, and run:

```sh
embedledger check --goos linux --goarch amd64 --max-bytes 10485760
```

The [repository workflow](.github/workflows/ci.yml) runs the real resolver and CLI tests on Linux, Windows, and macOS, plus the race detector on Linux. GitHub Actions dependencies are pinned to commit SHAs. Go is pinned to 1.27.1.

## What it measures

EmbedLedger reads `EmbedFiles` from `go list -json`. With `--tests`, it also resolves `TestEmbedFiles` and `XTestEmbedFiles` using `go list -test`, excluding synthetic package variants to avoid counting them again.

This follows Go's [documented embed rules](https://pkg.go.dev/embed), including hidden file behavior, `all:`, build constraints, and nested module exclusions. It does not implement its own glob matcher.

The inventory covers the selected packages in one module. It does not automatically include dependencies or sibling modules from a workspace. `./...` can include packages that a particular application never imports. Linker elimination can also remove embedded data. The total is the sum of selected input records, **not the size of a final binary**. A file selected for both production and tests is recorded in both contexts and contributes to both totals.

The tool fixes `GOWORK=off`, clears `GOFLAGS`, and defaults CGO to zero; pass the supported target and tag options explicitly. Other Go environment settings, including architecture tuning and experiments, remain inherited. Use a consistent toolchain and environment for repeatable results. Go version is recorded as informational metadata; a version change alone does not fail a comparison.

The tool requests readonly module resolution, disables automatic toolchain downloads, and sets `GOPROXY=off` and `GOSUMDB=off`. It never runs your program, tests, or generators. Go may write its own build cache, and required dependencies must already be available locally. Prepare dependencies with your usual Go workflow before scanning.

## Boundaries

This is an inventory review tool. It does not recognize secrets, decide whether a file is appropriate to ship, inspect a compiled binary, or prove that a release came from the reviewed inputs. A matching baseline says only that the selected paths and bytes match. Treat the baseline as reviewed source and run the check immediately before a build against a stable checkout.

Scans have a 60 second deadline, a 32 MiB Go metadata limit, a 100,000 record limit, and the configurable byte budget. Asset hashing streams data through a module rooted filesystem handle. Common concurrent changes are rejected, but the scan is not an atomic filesystem snapshot. Baseline creation uses a temporary file and a hard link to avoid overwrites; the destination filesystem must support hard links. Replacement uses rename and does not promise durability through every power failure or across every filesystem.

## Development

```sh
go test -count=1 ./...
go vet ./...
go test -race ./...
```

The race detector requires a supported platform and C compiler. Integration tests create temporary Go modules and launch the real Go resolver. A separate test builds and launches the CLI to verify process exit codes. No live services or credentials are needed.

See [research and related work](docs/research.md) for the search behind this project and [validation](docs/validation.md) for the recorded checks.

## License

A license has not yet been selected.
