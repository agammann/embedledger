# Changelog

## 1.0.0

Released October 6, 2026.

- Adopt the MIT license and publish native ZIP packages with SHA256 checksums.
- Include documentation and the checked example in each package. Go 1.27 or newer remains required on PATH for asset resolution.
- Verify each native package after unpacking on Windows, Linux, and macOS before publishing it.
- Define the 1.x compatibility contract for documented commands, flags, exit codes, and schema-1 baselines.
- Resolve filesystem aliases consistently when comparing Go package directories with the module root, including macOS temporary directories.

The baseline format is unchanged. Existing 0.1.1 baselines remain compatible when their build scope matches. Upgrade the executable, keep the same scan options, and run `check`; a new snapshot is not required.

## 0.1.1

Released September 19, 2026.

- Preserve Go subprocess cancellation and deadline errors instead of returning an unexplained process failure.
- Add `--timeout` to scan, snapshot, and check. The default remains one minute, with positive durations up to one hour supported. Timeout errors now suggest a concrete retry.
- Add regression coverage for cancellation, timeout recovery, and invalid durations.
- Rewrite installation and quick start directions, and add command, troubleshooting, and GitHub Actions guides.
- Record compatibility checks against pinned Glamour, Goose, and Mods checkouts, including independently checked hashes, drift, restoration, and resource limits.

The baseline format is unchanged. Existing 0.1.0 baselines remain compatible when their build scope matches.

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
