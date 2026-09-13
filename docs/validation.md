# Validation

Date: September 13, 2026.

## Local evidence

The implementation was exercised on Windows amd64 using the official Go 1.27.1 toolchain. The downloaded toolchain archive was verified against the SHA256 digest published by go.dev.

`go test -count=1 -coverprofile=coverage.out ./...` passed. The main CLI package reported 83.1 percent statement coverage after adding the partial package selection check. The example application is exercised separately rather than by its own unit tests.

`go vet ./...` passed. The bundled HTML asset resolved to 112 bytes and SHA256 `eb1f3865d429` as its displayed hash prefix. A snapshot followed by a check for Linux amd64 completed successfully.

## Behaviors exercised

1. Real Go resolution for directory, wildcard, and `all:` patterns, including hidden files.
2. Repeat scans produce identical JSON inventories.
3. Production, internal test, and external test assets remain distinct.
4. Build tags and target operating system affect the selected asset set.
5. Added files, removed files, and changes with the same byte count produce diffs.
6. A compiled CLI returns process status 0 for a match, 1 for drift, and 2 for a malformed baseline.
7. Checking preserves the baseline; overwriting requires an explicit force option.
8. Different scan scopes, missing packages (including a mixture of valid and unmatched patterns), invalid embed patterns, cancellation, and exceeded byte budgets fail without reporting a complete scan.
9. Invalid, duplicate, or inconsistent baseline records are rejected.
10. An embedded baseline is rejected to prevent a circular hash comparison.
11. Escaping symlinks are rejected by the rooted file API, on systems where the test process may create symlinks.

## CI evidence

The [GitHub Actions workflow](https://github.com/agammann/embedledger/actions/workflows/ci.yml) is the authoritative record for remote Linux, Windows, macOS, and race detector results. The workflow also checks the committed example inventory.

[Initial published revision c5fa79a](https://github.com/agammann/embedledger/actions/runs/34783332928) passed all four CI jobs. [Revision 0514ca6](https://github.com/agammann/embedledger/actions/runs/34783450415), which also rejects partially matched package selections, passed all four jobs: Windows, Linux, macOS, and the Linux race detector.

## Installation and publication

A fresh consumer installation with `go install github.com/agammann/embedledger@latest` retrieved revision `c5fa79a` through the Go module proxy. Installation of the subsequent exact revision `0514ca6933a2fd24b80124832ce57b9767c16595` was also verified. The installed executable printed version 0.1.0 and successfully checked the committed example baseline. Publication was read back through Git, and the remote tree matched the reviewed local tree. The public repository and rendered README were also checked in GitHub's browser UI.

The version 0.1.0 release packages this implementation with installation instructions and a changelog. Tagged installation uses `go install github.com/agammann/embedledger@v0.1.0`. Go 1.27 or newer must be installed and remain on PATH when using the tool.

Local cross compilation does not establish that a binary ran on that operating system. Consult the corresponding CI job for execution evidence. These checks are functional validation, not an exhaustive security audit.
