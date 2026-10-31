# Repo map — every other path

Part of the [repo map](repo-map.md) index. The platform chain is in
[repo-map-engine.md](repo-map-engine.md), `internal/` in
[repo-map-internal.md](repo-map-internal.md). Machine learning and private
experiments live in a private companion module.

## `cmd/`

| Command | Purpose |
|---|---|
| `forgec` | Fetch and compile the Forge corpus; `report`, `coverage`. |
| `gorged` | The server's thin main over `server/gorged` (perpetual bot tables, vs-bot games, web client), linking gorge's `bots/all` and, with `-events`, the draft stack. The maintainers' hosted demo is a companion module's main over the same package that adds trained-network bots. |
| `mtgsim` | Headless self-play over the repo decks with replay verification (`make sim`). |
| `draftsim` | Headless draft events with replay verification (`make draftsim`). |
| `labdemo` | The `draftlab` example workflow; writes 17Lands-shaped artefacts with `-out`. |
| `extagent` | The reference `gorge-agent/1` agent as a process. |
| `repro` | Replays a player feedback snapshot; `-emit-test` writes a failing test. |
| `headdiff` | Plays the `TestHeads` acceptance games; `-dump` records their event streams, `-against` names the first divergent event between two builds. |
| `cardfuzz` | Random mono-colour decks over the supported corpus; hunts panics, livelocks, divergences. |
| `enginebench` | The engine-speed rows (random/bot play, Clone, one step, search rates). `make enginebench-pair BASE=main CAND=.`, `make enginebench-profile`, `make enginebench-verify` (scripts: `scripts/enginebench-*.sh`). |
| `testbudget` / `gcgate` | Per-test wall and peak-RSS budget check (the post-merge full suite); by-hand GC-share check. |
| `gentypes` | Regenerates `web/src/protocol.ts` (`make gentypes`; `-check` in lint). |
| `pkgmap` | The package map for the docs site; `-check` (a pre-commit hook) fails on an unclassified package. |
| `codeshape` | Prints `internal/codeshape`'s metrics. |
| `deckimport` | Plain-text decklist → repo deck JSON. |
| `keywordbench` / `paymirror` | Keyword presence stats / payment A/B CLI. |

Training commands live in a private companion module; the compliance commands (`oraclediff`, `compliance`, `xmageport`, `oraclepacket`) to gorge-compliance's `cmd/` (public).

gorge carries no maintainer automation or demo-deploy machinery.

## Everything else

| Path | What it is |
|---|---|
| `web/` | Svelte + Vite client. `web/src/protocol.ts` is **generated** — never hand-edit. |
| `examples/` | Example bots and agents; `examples/extagent` is the Python `gorge-agent/1` agent. |
| `docs/design/` | Living designs and contracts, flat, `YYYY-MM-DD-<slug>[-design].md`, plus `cast-payment-plans/` and `seat-deck-manifest/` brief sets. Deliberate contracts (not debt) are in `docs/design/2026-09-22-engine-contracts.md`. |
| `docs/design/ARCHIVE.md` | Index of the parked historical design records; their text is in git history and the operator's local `.superpowers/archive/`. |
| `docs/decisions/` | Numbered architecture decisions (`0001-repo-layout.md`). |
| `docs/site/` | The public MkDocs site (`mkdocs.yml`); `docs/site/architecture/packages.tiers.json` classifies every package. |
| `docs/agents/` | This guide. |
| `.githooks/` | pre-commit and pre-merge-commit (run `pkgmap-check`), pre-push (the opt-in `RACE=1` race gate on a push of main) (`core.hooksPath=.githooks`). |
| `scripts/` | Contributor/CI only: `ci_shard.sh`, `smoke.sh`, `enginebench-*.sh`, `docs_sets.py`. (Maintainer-only scripts live outside the repo.) |
| `.github/workflows/` | `ci.yml`; `docs.yml` (builds the site, regenerates the package map); `coverage.yml` (on push to main: `make coverage`, publishes `.coverage/` as the `card-coverage` artifact; commits nothing). |
| `testkit/testutil/testdata/wall_exceptions.txt`, `rss_exceptions.txt` | Shrink-only inventories of tests over the per-test wall / RSS budget. |

## Untracked but load-bearing

| Path | Notes |
|---|---|
| `.cards/` | Forge corpus (`cardsfolder/`, `tokenscripts/`) and the IR cache `ir-<fingerprint>.gob.gz` with its per-card segment file `ir-<fingerprint>.seg` (written by `Registry.Save`, read by the subset loader). GPL-3.0 — never tracked. Without it, corpus tests **skip and read green**. |
| `.ds4/` | Maintainers' local scratch. `TestNothingUnderDS4IsTracked` keeps it out of git. |
| `.superpowers/` | Orchestration records, seat prompt context and `archive/` (the parked design records). |
| `.worktrees/` | Sibling task checkouts. Exclude from searches or you get duplicate hits. |
| `gorged-data/` | Default server persistence. Never share one directory between servers. |
| `feedback/` | Player bug reports. Excluded via `.git/info/exclude`, not `.gitignore`. |
| `cmd/repro/testdata/.tokens/` | GPL token text stripped from committed feedback fixtures. |
| the training-data disk | Training corpora, checkpoints, `GOTMPDIR`. Never `/tmp` (RAM-backed). |
