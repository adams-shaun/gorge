# Changelog

All notable changes to gorge are recorded in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
The module follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html),
with the pre-1.0 rules set out in [docs/releasing.md](docs/releasing.md). In
`v0`, a breaking change to a stable package or a contract bumps the minor
version. Stable packages are listed in `.github/stable-packages.txt`. Wire and
file contracts are listed in `contracts/registry.json`, each with its own
major version.

## [Unreleased]

### Added

- CI on GitHub Actions (`.github/workflows/ci.yml`). It gates pull requests
  from branches and forks, and pushes to `main` and `dev`. The jobs are gofmt,
  vet and the generated-types check, then six test shards plus the golden
  chain heads, then a single `ci-ok` aggregate check for branch protection.
  The Go build and test cache is restored from the base branch, so only the
  affected packages re-run. The Forge corpus is cached by `FORGE_REF`, and the
  run fails rather than skipping when the corpus is missing.
- Contributor governance: `CONTRIBUTING.md`, `CODEOWNERS`, a pull-request
  template, this changelog and `docs/releasing.md`.
- A contract and API-stability gate (`.github/workflows/contracts.yml`,
  `scripts/apidiff.sh`). A contract change needs a changelog entry. A stable
  package must stay API-compatible with the newest release tag unless the PR
  is labelled `breaking-change`.

### Fixed

- CR 113.6: a card's own `MayPlay$` static now functions only on the
  battlefield unless its script names an `EffectZone$`. Before this fix,
  Omniscience in hand offered itself as a free cast, and Future Sight on top
  of the library granted itself a play permission. (Dan Brooks, #3)

### Contracts

<!-- Changes to entries in contracts/registry.json: name, old -> new major, what changed. -->

### Breaking

<!-- Incompatible changes to stable packages (.github/stable-packages.txt) or contracts, with migration notes. -->
