# Command reference

[Back to the README](../README.md)

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
| `--timeout` | `1m` | Scan deadline as a Go duration, greater than zero and at most `1h` |
| `--force` | false | Permit replacing a baseline with `snapshot` |

Put flags before package arguments. Package arguments must be `.` or local patterns starting with `./`. The default is `./...`.

```sh
embedledger scan --json ./internal/web ./templates
embedledger snapshot --tags enterprise --goos linux --goarch arm64 --file embedledger.arm64.json
embedledger check --tags enterprise --goos linux --goarch arm64 --file embedledger.arm64.json
embedledger scan --tests --max-bytes 10485760
```

Use the same target, CGO setting, tags, package patterns, and test setting for snapshot and check. Different scopes return an error rather than a misleading diff. The byte limit and timeout are execution budgets and may differ between those commands.

| Exit code | Meaning |
| :--- | :--- |
| `0` | Inventory printed, baseline saved, or baseline matched |
| `1` | Added, removed, or changed assets |
| `2` | Invalid input, incompatible baseline, exceeded budget, or incomplete scan |

JSON diffs always contain `ok`, `added`, `removed`, `changed`, `before_bytes`, and `after_bytes`. Each changed item contains complete `before` and `after` records. Errors go to stderr; a failed scan does not emit a successful JSON report.

## What it measures

EmbedLedger reads `EmbedFiles` from `go list -json`. With `--tests`, it also resolves `TestEmbedFiles` and `XTestEmbedFiles` using `go list -test`, excluding synthetic package variants to avoid counting them again.

This follows Go's [documented embed rules](https://pkg.go.dev/embed), including hidden file behavior, `all:`, build constraints, and nested module exclusions. It does not implement its own glob matcher.

The inventory covers the selected packages in one module. It does not automatically include dependencies or sibling modules from a workspace. `./...` can include packages that a particular application never imports. Linker elimination can also remove embedded data. The total is the sum of selected input records, **not the size of a final binary**. A file selected for both production and tests is recorded in both contexts and contributes to both totals.

The tool fixes `GOWORK=off`, clears `GOFLAGS`, and defaults CGO to zero; pass the supported target and tag options explicitly. Other Go environment settings, including architecture tuning and experiments, remain inherited. Use a consistent toolchain and environment for repeatable results. Go version is recorded as informational metadata; a version change alone does not fail a comparison.

The tool requests readonly module resolution, disables automatic toolchain downloads, and sets `GOPROXY=off` and `GOSUMDB=off`. It never runs your program, tests, or generators. Go may write its own build cache, and required dependencies must already be available locally. Prepare dependencies with your usual Go workflow before scanning.

## Boundaries

This is an inventory review tool. It does not recognize secrets, decide whether a file is appropriate to ship, inspect a compiled binary, or prove that a release came from the reviewed inputs. A matching baseline says only that the selected paths and bytes match. Treat the baseline as reviewed source and run the check immediately before a build against a stable checkout.

Scans default to a 60 second deadline; use `--timeout 5m` for a cold Go cache or a larger project (maximum `1h`). They also have a 32 MiB Go metadata limit, a 100,000 record limit, and the configurable byte budget. Asset hashing streams data through a module rooted filesystem handle. Common concurrent changes are rejected, but the scan is not an atomic filesystem snapshot. Baseline creation uses a temporary file and a hard link to avoid overwrites; the destination filesystem must support hard links. Replacement uses rename and does not promise durability through every power failure or across every filesystem.
