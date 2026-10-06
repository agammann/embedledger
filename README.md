# EmbedLedger

[![CI](https://github.com/agammann/embedledger/actions/workflows/ci.yml/badge.svg)](https://github.com/agammann/embedledger/actions/workflows/ci.yml)

**Review changes to the files selected by `go:embed`.**

EmbedLedger records embedded file paths, sizes, and SHA256 hashes in a JSON baseline. It reports added, removed, and changed assets so you can review what your Go application includes before building it.

One Go CLI, standard library only. No account or service required.

[Install](#install) · [Quick start](#quick-start) · [Example](#try-the-example) · [CI setup](docs/github-actions.md) · [Command reference](docs/reference.md) · [Troubleshooting](docs/troubleshooting.md)

## Install

Install [Go 1.27 or newer](https://go.dev/doc/install), then check that `go version` works in your terminal. Go must remain on PATH because EmbedLedger uses its resolver when scanning your project.

```sh
go install github.com/agammann/embedledger@v0.1.1
embedledger version
```

Expected output: `embedledger 0.1.1`.

If your terminal cannot find `embedledger`, follow the [PATH setup instructions](docs/troubleshooting.md#command-not-found). This release is installed from source through Go; it does not include prebuilt binary downloads.

See the [0.1.1 release](https://github.com/agammann/embedledger/releases/tag/v0.1.1) and [changelog](CHANGELOG.md). Use `@latest` instead of `@v0.1.1` when you want the newest tagged version.

## Quick start

Open a terminal in **your project's module root**, the directory containing `go.mod`. Prepare its dependencies with your usual Go workflow, such as `go mod download`, before scanning. EmbedLedger resolves packages without downloading missing modules.

These examples consistently select **Linux amd64**, matching the CI guide. Change both target flags throughout if you build for another platform.

1. Inspect the embedded files, then save a baseline:

   ```sh
   embedledger scan --goos linux --goarch amd64
   embedledger snapshot --goos linux --goarch amd64
   ```

2. Review `embedledger.json` and commit it alongside your source. Keep it outside embedded directories and patterns. It contains paths and hashes, not asset contents.

3. Compare future changes against that baseline:

   ```sh
   embedledger check --goos linux --goarch amd64
   ```

   A match exits with status `0`. Added, removed, or changed assets exit with `1`. Invalid input or an incomplete scan exits with `2`.

4. After reviewing an intentional asset change, update and review the baseline:

   ```sh
   embedledger snapshot --force --goos linux --goarch amd64
   embedledger check --goos linux --goarch amd64
   ```

`check` never rewrites the baseline. `snapshot` requires `--force` to replace an existing file. Use identical target, tags, CGO, package selection, and test settings when saving and checking a baseline.

By default, the tool inspects `./...` in one module and excludes test assets. See the [command reference](docs/reference.md) for package selection, build tags, JSON output, test assets, and size limits.

## Try the example

Use a fresh checkout to try the tool without modifying your own project:

```sh
git clone https://github.com/agammann/embedledger.git
cd embedledger
embedledger check --goos linux --goarch amd64
```

Expected output:

```text
Embedded assets match the baseline (112 bytes).
```

Create `examples/site/assets/note.txt` with a text editor and save the text `hello` without a trailing newline. Run the check again. It exits with `1` and prints:

```text
ADDED    production "examples/site/assets/note.txt" (5 bytes)
1 added, 0 removed, 0 changed; 112 -> 117 bytes.
```

If your editor adds a newline, the byte counts will be larger. Delete only the `note.txt` file you just created and rerun the check; the original baseline matches again. There is no need to overwrite the example baseline.

`go run ./examples/site` prints the embedded HTML to your terminal. It does not start a web server.

## Add to CI

The [GitHub Actions guide](docs/github-actions.md) includes a complete workflow that installs Go, prepares dependencies, installs the pinned CLI, and checks your committed baseline. CI should fail on unexpected drift; update the baseline only after reviewing the asset changes.

## Scope and limits

EmbedLedger uses Go's actual embed file selection. It inventories selected package inputs in one module; it does not automatically include dependency modules, measure a final binary, detect secrets, or prove release provenance. A matching baseline means the selected file paths and bytes match.

See [resolution behavior and resource limits](docs/reference.md#what-it-measures) for environment settings, filesystem requirements, and scan boundaries.

## Build and development

From a checkout, build and run on macOS or Linux:

```sh
go build -o dist/embedledger .
./dist/embedledger version
```

On Windows PowerShell:

```powershell
go build -o dist/embedledger.exe .
.\dist\embedledger.exe version
```

Run the development checks:

```sh
go test -count=1 ./...
go vet ./...
go test -race ./...
```

The race detector needs a supported platform and C compiler. [Repository CI](.github/workflows/ci.yml) runs the real resolver and CLI tests on Windows, Linux, and macOS, plus the Linux race detector.

[Real project checks](docs/real-world-validation.md) · [Validation record](docs/validation.md) · [Research and related work](docs/research.md)

## License

EmbedLedger is licensed under the [MIT License](LICENSE).
