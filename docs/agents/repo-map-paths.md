# Repo map — every other path

Part of the [repo map](repo-map.md) index. The platform chain is in
[repo-map-engine.md](repo-map-engine.md), `internal/` in
[repo-map-internal.md](repo-map-internal.md).

## `cmd/`

| Command | Purpose |
|---|---|
| `forgec` | Fetch and compile the Forge corpus; `report`, `coverage`. |
| `gorged` | The server's thin main over `server/gorged` (perpetual bot tables, vs-bot games, web client), linking gorge's `bots/all` and, with `-events`, the draft stack. A companion module's main over the same package can add trained-network bots. |
| `mtgsim` | Headless self-play over the repo decks with replay verification (`make sim`). |
| `draftsim` | Headless draft events with replay verification (`make draftsim`). |
| `deckpool` | Drafts seeded pods and writes every seat's deck as a deck file, with an index and a manifest. |
| `draftres` | Draft harness: drafts 8-seat pods with a mix of drafter variants and builds every pool. |
| `labdemo` | The `draftlab` example workflow; writes 17Lands-shaped artefacts with `-out`. |
| `extagent` | The reference `gorge-agent/1` agent as a process. |
| `repro` | Replays a player feedback snapshot; `-emit-test` writes a failing test. |
| `headdiff` | Plays the `TestHeads` acceptance games; `-dump` records their event streams, `-against` names the first divergent event between two builds. |
| `cardfuzz` | Random mono-colour decks over the supported corpus; hunts panics, livelocks, divergences. |
| `enginebench` | The engine-speed rows (random/bot play, Clone, one step, search rates). `make enginebench-pair BASE=main CAND=.`, `make enginebench-profile`, `make enginebench-verify` (scripts: `scripts/enginebench-*.sh`). |
| `testbudget` / `gcgate` | Per-test wall and peak-RSS budget check (run by hand; no workflow invokes it); by-hand GC-share check. |
| `gentypes` | Regenerates `web/src/protocol.ts` (`make gentypes`; `-check` in lint). |
| `pkgmap` | The package map for the docs site; `-check` (a pre-commit hook) fails on an unclassified package. |
| `codeshape` | Prints `internal/codeshape`'s metrics. |
| `deckimport` | Plain-text decklist → repo deck JSON. |
| `kcensus` | Mechanic census over a deck pool: which primitives each distinct card reaches, and the smallest mechanic set that covers successively larger shares of the pool's decks. |
| `spellbench-gorge-env` | The SpellBench Protocol v2 environment role over stdio, over `server/spellbench`'s catalog, mapping, session and validation. |
| `spellbench-gorge-agent` | The SpellBench Protocol v2 agent role over stdio: serves gorge's registered bot policies. |
| `keywordbench` / `paymirror` | Keyword presence stats / payment A/B CLI. |

The compliance commands (`oraclediff`, `compliance`, `xmageport`, `oraclepacket`) live in gorge-compliance's `cmd/`.

## Everything else

| Path | What it is |
|---|---|
| `web/` | Svelte + Vite client. `web/src/protocol.ts` is **generated** — never hand-edit. |
| `examples/` | Example bots and agents (`examples/bots`); `examples/extagent` is the Python `gorge-agent/1` agent. |
| `tools/manabrew-bridge` | The ManaBrew bridge. |
| `deploy/` | Container demo deployment files (`deploy/docker`). |
| `cli/` | The command-line parser ([`docs/cli.md`](../cli.md)). |
| `docs/design/` | Living designs and contracts, flat, `YYYY-MM-DD-<slug>[-design].md`, plus the payment-plan codec in `cast-payment-plans/`. Deliberate contracts (not debt) are in `docs/design/2026-09-22-engine-contracts.md`. |
| `docs/design/ARCHIVE.md` | Index of the parked design records; their text is in git history. |
| `docs/decisions/` | Numbered architecture decisions (`0001-repo-layout.md`). |
| `docs/site/` | The public MkDocs site (`mkdocs.yml`); `docs/site/architecture/packages.tiers.json` classifies every package. |
| `docs/agents/` | This guide. |
| `.githooks/` | pre-commit and pre-merge-commit (run `pkgmap-check`); pre-push (opt-in `RACE=1` race gate on a push of main). `core.hooksPath=.githooks`. |
| `scripts/` | Contributor/CI only: `test-targets.sh` (the `make test-*` targets), `ci-fast.sh`, `ci_shard.sh`, `smoke.sh`, `enginebench-*.sh`, `hostbench-pair.sh`, `xengine-bench.sh`, `wire-matrix.sh`, `open-pr.sh` (`make pr`), `dirac-gate.sh`, `dirac-evidence.sh`, `dirac-first-refusals.py`, `docs_*.py`. |
| `.github/workflows/` | `ci.yml` (PR `fast` and `lint`; the merge-queue batch's shards, `examples`, `wire-matrix`, `dirac-slices` and `ci-ok`; a nightly run); `docs.yml` (regenerates the package map, builds the site `--strict`); `coverage.yml` (on push to main: `make coverage`, publishes `.coverage/` as the `card-coverage` artifact; commits nothing); `demo-image.yml` (builds the container demo image on a push to main, and pushes it only when the registry secrets are set). |
| `.github/workflows/oracle.yml` | Runs the whole `dirac/oracle` on main, hourly cron that starts only if no run began in the last 5h30m (about every 6 hours). |
| `.github/workflows/queue-cancel.yml` | Cancels orphaned merge-queue runs. |
| `.mergify.yml` | The merge-queue configuration (batches of up to 10; `hotfix/*` jumps the queue). |
| `testkit/testutil/testdata/wall_exceptions.txt`, `rss_exceptions.txt` | Shrink-only inventories of tests over the per-test wall / RSS budget. |

## Untracked but load-bearing

| Path | Notes |
|---|---|
| `.cards/` | Forge corpus (`cardsfolder/`, `tokenscripts/`) and the IR cache `ir-<fingerprint>.gob.gz` with its per-card segment file `ir-<fingerprint>.seg` (written by `Registry.Save`, read by the subset loader). GPL-3.0 — never tracked. Without it, corpus tests **skip and read green**. |
| `.worktrees/` | Sibling task checkouts. Exclude from searches or you get duplicate hits. |
| `gorged-data/` | Default server persistence. Never share one directory between servers. |
| `feedback/` | Player bug reports. Excluded via `.git/info/exclude`, not `.gitignore`. |
| `cmd/repro/testdata/.tokens/` | GPL token text stripped from committed feedback fixtures. |
