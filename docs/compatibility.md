# Stability and upgrades

[Back to the README](../README.md)

## The 1.x contract

EmbedLedger 1.x preserves the documented `scan`, `snapshot`, `check`, `version`, and `help` commands; existing flags and their defaults; and these exit codes:

| Exit code | Meaning |
| :--- | :--- |
| `0` | Complete inventory, saved baseline, or matching baseline |
| `1` | A complete comparison found added, removed, or changed assets |
| `2` | Invalid input, incompatible baseline, or an incomplete scan |

Schema-1 baselines retain their field names and meanings. Version 1.x continues to read existing valid schema-1 baselines. The JSON inventory and comparison fields documented in the [command reference](reference.md) retain their types and meanings. Additional commands and optional flags may be added; removing existing behavior or changing these formats requires a new major version.

A baseline comparison uses the recorded target OS, architecture, CGO setting, build tags, package selection, and test setting. Keep those options identical when saving and checking. The byte budget and timeout may differ. Go version is recorded as information and does not itself cause drift; Go's selected inputs can change between toolchains, so review any reported asset changes after a toolchain update.

Go 1.27 or newer must remain on PATH even when using a prebuilt executable. Native packages contain the CLI, MIT license, README, changelog, documentation, and the example module. The package filenames identify the OS and architecture that actually built and ran the package checks. These packages do not bundle Go.

## Upgrade from 0.1.1

Install the pinned source release:

```sh
go install github.com/agammann/embedledger@v1.0.0
embedledger version
```

The version output is `embedledger 1.0.0`. Alternatively, replace your previous executable with the verified native package for your OS and architecture.

Keep your reviewed `embedledger.json` and run `check` with the same options you used in 0.1.1:

```sh
embedledger check --goos linux --goarch amd64
```

The baseline format is unchanged; do not run `snapshot --force` merely to upgrade. If the check reports drift, review the inputs before deciding whether to accept a new baseline. Update the installation pin in your CI workflow separately.

## Verify a native package

Each ZIP has a matching `.zip.sha256` file. Compare its hash with the ZIP before unpacking. The release also includes `SHA256SUMS` for all native ZIP files.

On Windows PowerShell:

```powershell
Get-FileHash .\embedledger_1.0.0_windows_amd64.zip -Algorithm SHA256
Get-Content .\embedledger_1.0.0_windows_amd64.zip.sha256
```

On Linux, run `sha256sum -c` with the checksum filename. On macOS, run `shasum -a 256 -c` with it. Use the package matching your architecture.

After unpacking, use the CLI from the package directory or place it on PATH. Its included example baseline can be checked with `embedledger check --goos linux --goarch amd64`; this reports 112 bytes. In PowerShell, use `.\embedledger.exe` for the executable in the current directory. On macOS and Linux, use `./embedledger`.
