# GitHub Actions setup

[Back to the README](../README.md)

## Prepare a reviewed baseline

From your module root, prepare dependencies and save the Linux amd64 inventory:

```sh
go mod download
embedledger snapshot --goos linux --goarch amd64
embedledger check --goos linux --goarch amd64
```

If a baseline already exists, review it with `check` before deciding to update it. Review `embedledger.json` and commit it. The workflow below expects this file and `go.mod` at the repository root. Keep the baseline outside all embed patterns.

## Add the workflow

Save this as `.github/workflows/embedded-assets.yml` in your project:

```yaml
name: Embedded assets

on:
  push:
  pull_request:

permissions:
  contents: read

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262 # v4
      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6
        with:
          go-version: '1.27.1'
          cache: false
      - name: Prepare project dependencies
        run: go mod download
      - name: Install EmbedLedger
        run: go install github.com/agammann/embedledger@v1.0.0
      - name: Check reviewed embedded assets
        run: embedledger check --goos linux --goarch amd64 --max-bytes 10485760 --timeout 5m
```

This example permits up to **10 MiB** of selected asset input, uses CGO disabled, and checks production assets across `./...`. It allows five minutes for resolution and hashing, including cold cache work. Adjust the byte budget and timeout for your project. Use a Go version that also satisfies your module's requirements. Private module downloads need the same authentication as your existing build job; no special EmbedLedger account is required.

When assets change, the check exits with status `1` and fails the job. Review the changes locally, update the baseline explicitly with matching options, and commit its diff. Do not run `snapshot --force` in the checking workflow: doing so would accept changes before comparing them.

## Adapt to your build

If your module is in a subdirectory, set the job's `defaults.run.working-directory` to that directory. Checkout and setup actions still operate at the job level.

If your build uses tags, a different target, CGO, selected package patterns, or test assets, pass the same options to snapshot and check. Put flags before package arguments. A CGO build may also require the target's compiler and system libraries.

For multiple targets, create separate baseline files and check each one. For example:

```sh
embedledger snapshot --goos linux --goarch arm64 --file embedledger.arm64.json
embedledger check --goos linux --goarch arm64 --file embedledger.arm64.json
```

Run the comparison immediately before your normal build against a stable checkout. The inventory covers the selected source inputs, not the linked binary or all dependency modules.
