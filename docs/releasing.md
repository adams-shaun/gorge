# Versioning and releases

This page sets out what gorge promises to the code and data that depend on
it, and how a release is cut. The contracts themselves are described in
[contracts.md](contracts.md). Contributor expectations are in
[CONTRIBUTING.md](../CONTRIBUTING.md).

## What is versioned

gorge carries two kinds of version.

1. **The module version.** These are Git tags `vX.Y.Z` on `main`, following
   [Semantic Versioning 2.0](https://semver.org/spec/v2.0.0.html). A Go
   consumer pins one with `go get github.com/adams-shaun/gorge@vX.Y.Z`. It
   covers the exported API of the **stable packages** only.
2. **A major version for each contract.** Every wire or file format that
   another program reads is named in `contracts/registry.json`, with its own
   major version and golden-file hashes. Examples are the decision JSON a seat
   client answers, and the draftlab extern wire. A consumer that parses JSON
   pins the contract major, not the Go API.

## Stable packages

The stable packages are listed in
[`.github/stable-packages.txt`](../.github/stable-packages.txt):

| package | why it is stable |
|---|---|
| `decision` | the decision model every seat, bot and client answers |
| `draftlab/extern/wire` | the wire format external draft and training tools speak |

A public `az` search facade is planned to join the list. **Every other package
is unstable.** That includes `rules`, `state`, `effects`, `view`, `host`, every
`internal/` package and every `cmd/` tool. An unstable package may change in
any release without a changelog entry under `### Breaking`.

### Adding or removing a stable package

1. Add the import path, relative to the module root, to
   `.github/stable-packages.txt` in a PR of its own. Only the owner can approve
   that file (`CODEOWNERS`).
2. Note the addition in `CHANGELOG.md` under `### Added`.
3. The `apidiff` check starts guarding the package against the next release
   tag that contains it. Until then the check prints `no baseline, skipped`.

Removing a package from the list is a breaking change. It is recorded under
`### Breaking`.

## What counts as breaking

**For a stable package**, a change is breaking when
[`apidiff`](https://pkg.go.dev/golang.org/x/exp/cmd/apidiff) reports it as
incompatible. Examples:

- removing or renaming an exported identifier
- changing a function signature or a field's type
- adding a method to an exported interface
- adding a field that makes a struct literal stop compiling

Behaviour matters as well as signatures. Changing what a documented field
means is also breaking, even when `apidiff` stays quiet. Reviewers check for
it.

**For a contract**, a change is breaking when a reader written against the
previous major would misread or reject the new output. Examples:

- removing or renaming a field
- changing a field's type or meaning
- making an optional field required
- reordering a positional encoding

Adding an optional field that old readers ignore is not breaking. Any change
updates the registry's golden hashes, and a breaking change also bumps the
contract's major.

## Version numbers while in v0

- **Breaking change** to a stable package, or a contract major bump: bump
  **minor** (`v0.4.2` → `v0.5.0`).
- **Anything else** (new API, fixes, unstable-package changes): bump
  **patch** (`v0.4.2` → `v0.4.3`).
- Every breaking change is listed under `### Breaking` with a migration note,
  even in v0.

### Path to v1

gorge tags `v1.0.0` once two things hold:

- the stable package set includes the search facade
- no stable package or contract has needed a breaking change for a sustained
  period (owner's call; about two months is the working target)

From v1 on, a breaking change to a stable package needs a new major version.
In Go that means a `/v2` module path, so breaks become rare and deliberate.
Contract majors keep their own numbering throughout.

## The CI gate

[`.github/workflows/contracts.yml`](../.github/workflows/contracts.yml) runs
on every PR to `main` or `dev`, and on every push to `main`. It has two jobs.

- **changelog**: a PR that changes `contracts/registry.json` or
  `docs/contracts.md` must also change `CHANGELOG.md`.
- **apidiff**: `scripts/apidiff.sh` compares each stable package with the
  newest `v*` tag. An incompatible change fails the PR unless the PR is
  labelled `breaking-change`. On a push to `main`, the job only reports. With
  no tag yet, it passes with a notice. Run it locally with
  `bash scripts/apidiff.sh [TAG]`.

## Release procedure

The owner cuts releases from an up-to-date, green `main`.

1. Pick the version from the `## [Unreleased]` entries, using the rules
   above. Any `### Breaking` entry, or any contract major bump, means a minor
   bump while in v0.
2. In a PR, rename `## [Unreleased]` to `## [vX.Y.Z] - YYYY-MM-DD`. Delete any
   subsection left empty. Open a fresh `## [Unreleased]` above it, with empty
   `### Contracts` and `### Breaking` placeholders. Merge the PR.
3. Tag the merge commit and push the tag:

   ```sh
   git fetch origin && git tag -a vX.Y.Z -m vX.Y.Z origin/main
   git push origin vX.Y.Z
   ```

4. Publish the release with that version's changelog section as the notes:

   ```sh
   awk '/^## \[vX.Y.Z\]/{f=1;next} /^## \[/{f=0} f' CHANGELOG.md > /tmp/notes.md
   gh release create vX.Y.Z --verify-tag --title vX.Y.Z --notes-file /tmp/notes.md
   ```

The new tag becomes the `apidiff` baseline for every later PR.

## Cadence

*Owner-tunable.* A release is cut on demand whenever a contract or a stable
package's API changes, so that consumers can pin the change. While an external
consumer pins gorge, a release is also cut at least once a week if
`[Unreleased]` is non-empty. Quiet weeks need no release.
