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

- Package `cli`, a stdlib-only cobra/viper-style command layer: subcommands,
  GNU flags that still accept Go `flag` single-dash spellings,
  `GORGE_<CMD>_<FLAG>` environment variables, a JSON `--config` file and
  `--print-config`. Precedence is flag > env > config > default. See
  `docs/cli.md` (#9).

### Changed

- `dzgorge` is built on `cli`. Every flag keeps its name and default, and
  existing `-flag value` invocations are unchanged. New: `dzgorge help`,
  `--help`, `GORGE_DZGORGE_<CMD>_<FLAG>`, `--config` and `--print-config`.
  A stray positional argument is now an error (exit 2); it used to be
  ignored silently (#9).

### Fixed

- CR 113.6: a card's own `MayPlay$` static now functions only on the
  battlefield unless its script names an `EffectZone$`. Before this fix,
  Omniscience in hand offered itself as a free cast, and Future Sight on top
  of the library granted itself a play permission. (Dan Brooks, #3)

### Contracts

<!-- Changes to entries in contracts/registry.json: name, old -> new major, what changed. -->

- New `contracts` package and `docs/contracts.md`. Every external file, wire
  and grammar has a name, a major version and golden hashes pinned in
  `contracts/registry.json` (#7).
- dzgorge-version 1 (new): `dzgorge version` prints one JSON object with the
  build, every contract's major version, the entity encoder hash, the cards
  compiler fingerprint, the IR cache version and the `cards.lock` commit and
  digest.
- dzg-games 1 (additive): each `games.jsonl` record and the `.summary.json`
  carry `format: "dzg-games-1"` and `gorge: {version, revision}`.
- dzgw 1 (additive): the `.dzgw` header may carry `encoder_hash`. A network
  whose hash differs from this build's entity encoder is refused; one without
  the field loads with a warning.
- dzgorge-spec 1: a flag-style policy key given a value (`autopay=0`) is an
  error. It used to enable the flag silently.
- dzgorge-spec 1 (additive): `play -a/-b` accept a policy as a JSON object
  (`{"kind":"az","sims":200,"autopay":true}`), in a config file or as a
  `{...}` flag value. Paths may contain `:` in this form (#9).
- dzgorge-config 1 (new): the `dzgorge --config` file, tagged
  `"$schema": "dzgorge-config-1"` (#9).

### Breaking

<!-- Incompatible changes to stable packages (.github/stable-packages.txt) or contracts, with migration notes. -->

- dzg-pack 1 -> 2: `dzgorge pack` writes `dzg-pack-2` by default. `meta.json`
  adds `encoder_hash`, `record_type`, `schema_version` and `gorge`. `kinds` is
  the canonical append-only table `contracts.PackKinds`, and the `kind` column
  indexes into it. Inputs that disagree on encoder hash, record type or schema
  version are refused, and an undecodable record is an error at file:line
  (`-allow-truncated` tolerates a truncated gzip tail). Migration:
  `pack -format 1` still writes dzg-pack-1 byte for byte, deprecated, and is
  removed in the next release.
