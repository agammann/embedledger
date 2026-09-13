# Changelog

## 0.1.0

Released September 13, 2026.

The first release makes changes to Go embedded assets visible in code review and CI.

1. Resolve the actual asset inputs through `go list`, following Go's embed rules.
2. Inspect paths, byte counts, and SHA256 hashes with `scan`.
3. Save a reviewed JSON inventory with `snapshot`.
4. Report additions, removals, and content changes with `check`, including structured JSON output.
5. Select target OS, architecture, CGO setting, build tags, local packages, and optional test assets explicitly.
6. Reject incompatible baselines, incomplete package selections, embedded baselines, and scans that exceed resource budgets.
7. Preserve an existing baseline unless `snapshot --force` is explicitly requested.

Exit status is 0 for success, 1 for asset drift, and 2 for invalid input or an incomplete scan.

Requires Go 1.27 or newer on PATH. The inventory covers selected source inputs in one module; it does not measure the final linked binary or automatically include dependencies.

The release uses only the Go standard library. Windows, Linux, and macOS execution and the Linux race detector are covered by the repository CI workflow.
