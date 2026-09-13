# Research and related work

Research date: September 13, 2026.

## The gap explored

The question was whether a small Go tool could make changes to the resolved `go:embed` asset set reviewable in version control and enforce that review in CI.

The proposed workflow combines Go's resolver, a deterministic inventory of paths and content hashes, explicit build context, and comparison exit codes. We did not find a standalone implementation of that exact workflow in the searches below. This is a bounded search result, not a claim that no implementation exists or that the concept is legally novel.

## Evidence of the problem

The Go project's [embed documentation](https://pkg.go.dev/embed) explains that a directory pattern excludes files starting with `.` or `_`, while a wildcard can directly match hidden files, and `all:` includes hidden files recursively. This creates a concrete reason to inspect the resolved file list instead of assuming a directory contains only intended assets.

In the original [Go embed design discussion](https://www.reddit.com/r/golang/comments/hv96ny/), a participant raised the concern that explicit source would make accidentally compiling a large amount of image data more obvious. This is evidence of the broader visibility concern, not a request for this exact product. The implementation here was written independently.

## Adjacent work

| Project or feature | Existing purpose | Difference from EmbedLedger |
| :--- | :--- | :--- |
| [Gopls embed analyzer](https://go.dev/gopls/analyzers#embed) | Check embed directive usage | Does not provide a reviewed asset inventory and content diff |
| [Go list](https://pkg.go.dev/cmd/go) | Expose package and embedded file metadata | Supplies the authoritative selection; does not itself save and compare this baseline |
| [git-pkgs/pin](https://github.com/git-pkgs/pin) | Vendor browser dependencies and verify them using a lockfile | Manages downloaded browser assets; EmbedLedger inventories arbitrary existing embedded files |
| [archive portability](https://github.com/GitHubCatTest/archive-portability) | Check archive filename portability | A close implementation of an earlier candidate idea, which was therefore rejected |

## Search record

Web searches included `golang embed manifest lock audit tool embedcheck`, `"go:embed" "manifest" "diff"`, `"go:embed" "lockfile"`, `"go:embed" "linter"`, and the exact proposed name `"embedledger"`.

GitHub repository searches for `"go:embed" manifest` and `embedledger` returned no repositories at research time. A search for `"go:embed" audit` returned an unrelated application. Search coverage is limited by indexing, query wording, repository descriptions, private code, and unpublished work.

The name check found no exact product or repository match in those searches. It was not a trademark clearance or domain availability check.
