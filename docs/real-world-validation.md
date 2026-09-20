# Real project validation

[Back to the README](../README.md)

Date: September 19, 2026. Version under test: 0.1.1.

These are CLI compatibility checks against public Go projects, performed locally on Windows amd64 with Go 1.27.1 and a Linux amd64 scan target. They are not user interviews, endorsements, production deployments, or execution of those projects' applications.

## Projects and results

| Project revision | Module and package selection | Inventory |
| :--- | :--- | :--- |
| [Glamour 49df656](https://github.com/charmbracelet/glamour/tree/49df6562f7a3740f872c3c96d46d3257aef30e56) | Nested `examples` module; `./artichokes` | 1 record, 1,639 bytes |
| [Goose 43d2d9c](https://github.com/pressly/goose/tree/43d2d9c819ed6c9ba2b67a86bdf9fc08562495b7) | Root module; `--tests . ./testdata` | 10 records, 4,162 bytes |
| [Mods 0425d0d](https://github.com/charmbracelet/mods/tree/0425d0d7861e4bbc396200b3bd2eee1825715300) | Root module; default `./...` | 1 record, 17,802 bytes |

For each selected scope, the following passed:

- Independently recompute every file's SHA256 with PowerShell `Get-FileHash`, verify byte sizes, and sum the inventory.
- Save a baseline and verify an unchanged checkout exits with status 0.
- Change embedded content without changing its byte count and verify status 1 with a changed record.
- Restore the original bytes and verify status 0.
- Verify checking never changes the saved baseline.
- Set `--max-bytes 1` and verify status 2.

Goose selects five SQL files in both production and external test contexts, so its ten records deliberately count the files twice. Adding and removing a SQL file under its wildcard produced two added or removed records and status 1. Restoring the files returned to a match.

Deleting Mods' literally named `config_template.yml` made Go's embed directive invalid. The CLI correctly returned status 2 with no successful JSON report. All temporary asset changes were restored.

## Reproduction scope

Check out the linked revisions in separate directories. Install Go 1.27.1 and EmbedLedger 0.1.1, then prepare dependencies with `go mod download` in each selected module. Review any resulting module metadata changes before scanning.

Glamour's nested example had older module metadata than the parent module it replaces locally. Dependency preparation updated `examples/go.mod` and `examples/go.sum`; the successful example checks used that prepared state. With the original metadata, readonly resolution correctly rejected the scan. Its root module's empty inventory was also valid: root `./...` does not enter nested modules.

Run from each indicated module directory:

```sh
# Glamour: from examples/
embedledger scan --goos linux --goarch amd64 --timeout 5m --json ./artichokes

# Goose: from the repository root
embedledger scan --goos linux --goarch amd64 --tests --timeout 5m --json . ./testdata

# Mods: from the repository root
embedledger scan --goos linux --goarch amd64 --timeout 5m --json
```

For snapshot and check, keep the same target, test setting, and package arguments. Pass `--file` with a baseline path outside the selected embedded assets. Put flags before package arguments. The selected Goose packages are intentional: Go's `./...` skips `testdata` directories.

## Timeout fix

Initial broad scans of Goose and Mods failed near the old fixed one-minute deadline with an unhelpful `go list: exit status 1:` message. Subsequent warm-cache scans succeeded. This suggested cold Go resolution was reaching the deadline.

A controlled check against Mods with a fresh Go build cache and `--timeout 1s` reproduced cancellation. Version 0.1.1 returned status 2, preserved `context deadline exceeded`, suggested `--timeout 5m`, and emitted no successful JSON report. Retrying the same scan with five minutes succeeded with 1 record and 17,802 bytes.

Regression tests cover cancellation propagation, an expired deadline, a successful retry, and invalid timeout values. The full local test suite and `go vet ./...` passed.

## New user directions

The README example was exercised from a fresh checkout under a Windows path containing spaces. Its committed 112-byte baseline matched; adding exactly five bytes in `note.txt` reported 112 to 117 bytes and status 1; deleting that new file restored the match. Running the bundled example printed its HTML as documented.

See [CI results](https://github.com/agammann/embedledger/actions/workflows/ci.yml) for native Windows, Linux, macOS, and Linux race detector execution. Selecting Linux as a scan target on Windows does not itself establish native Linux execution. These finite checks establish the listed behavior; they do not establish compatibility with every Go project or constitute an exhaustive security audit.
