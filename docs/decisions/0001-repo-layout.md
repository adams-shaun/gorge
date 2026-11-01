# 0001 — Repository layout: engine / server / bots / lab / ops

- **Status:** accepted 2026-10-16. Execution: phases below.
- **Supersedes:** the flat-root layout and README's "Package layout" chain.
- **Amended by:** [ADR 0002](0002-public-engines-and-draft.md) (2026-10-30).

## Why

The root holds 43 tracked entries (31 Go package dirs) plus ~15 untracked
ones. Five audiences share one flat namespace — the rules engine, the
reference server, bot/AI research, compliance oracles and the agent
pipeline — with near-collisions (`event`/`events`, `bots`/`botpolicy`,
`host`/`eventhost`, `tools`/`scripts`, `compliance`/`validation`). An outside
contributor fixing a card (PR #3, `rules/mayplay.go`) has no way to tell the
platform from the research, and the docs are written for agent seats.

## Decisions

1. **Audience.** The repo is laid out for an outside contributor first.
   Agent-seat operating rules move under `ops/`; AGENTS.md becomes thin.
2. **Identity.** gorge is an extensible platform: engine, reference server,
   web client, baseline and hosted bots, developer tooling. Draft/limited
   events, compliance oracles and scripted train/retrain/iterate research are
   *extensions*: fenced under `lab/` now, moved to another repo later.
3. **Fence.** Nothing outside `lab/` imports `lab/`. `internal/archtest`
   enforces it; `scripts/ci_shard.sh`'s research exclude regex seeds the list.
4. **Hosted bots: inference in the platform.** The runtime half of the hosted
   bots (MCTS search, net forward pass, belief, sb builtins, searchseat)
   moves into `bots/`. Trainers, corpus builders and calibration benches go to
   `lab/training/`. `policynet` and `searchprobe` are split along that line.
5. **Draft in gorged: seam + lab composition.** The seam is the `lab` build
   tag: `cmd/gorged/events.go` and `art_sets.go` (the draft runtime and set
   art prewarm) build only with `-tags lab`; the untagged binary links no
   `lab/` package and refuses `-events` (`TestEventsNeedAPlugin`).
   `make deploy-demo`, the per-ticket gate, the full suite and CI build with
   `-tags lab`. `server/protocol/draftevent.go` stays: it is wire types the
   web client generates from and imports nothing from `lab/`.
6. **`seat` is a contract, bots are injected.** `seat` keeps only `Seat`,
   `PaymentPlanConsumer`, `TargetHintConsumer` and imports only `decision` +
   `view`; it lives at `engine/seat`. `BoardSeat` (names `botpolicy.Board`)
   moves to `bots`. `seat.Bot` → `bots/heuristic`, `PolicyNetBot` →
   `bots/policynet`. `host` receives a `seat.Factory` (the `bots` registry)
   injected by its command instead of importing concrete bots. This also
   breaks today's `seat → botpolicy`, `bots → seat` cycle.
   *As built:* the contract is `engine/seat` (`Seat`, `PaymentPlanConsumer`,
   `TargetHintConsumer`); `Bot`, `PolicyNetBot` and `BoardSeat` live in
   `bots/heuristic` (botpolicy may not import view, so it cannot own an
   interface that embeds `seat.Seat`). `server/host` already resolves bots
   through the `bots` registry; a narrower factory interface is follow-up.
7. **Engine prefix now.** The core moves under `engine/` in the same window as
   the other Go moves, before any external consumer pins import paths.
8. **Dead material is deleted**, not archived; git history keeps it.
9. **Historical design records leave the tree.** `docs/superpowers/` is parked
   in a gitignored local directory;
   living content is rewritten into `docs/architecture/` and this
   `docs/decisions/` series.

## Target root

```
README.md CONTRIBUTING.md AGENTS.md LICENSE NOTICE Makefile go.mod
engine/    cards state decision events effects rules view replay deck seat
server/    host (+httpapi) protocol manabrew divergence feedback
bots/      policy (was botpolicy), registry, heuristic, policynet, hosted inference
web/       TS client
cmd/       product + developer tools only
internal/  archtest testutil corpusheap
lab/       draft/ oracle/ training/ cmd/ scripts/
ops/       deploy scripts docs
docs/      contributing/ architecture/ decisions/ site/
```

## As built (2026-10-16)

All phases landed in one sitting on branch `wt/restructure`, driven by
`ops/restructure/restructure.py` (stage `go`, then `--stage=ops`) plus hand
fixes for what a path rewrite cannot see: archtest/codeshape short package
names, gate trigger regexes, smoke fixtures, Python path joins. The fence is
`internal/archtest/lab_fence_test.go`. `packages(t)`-style checks that build
`module + "/" + name` from short names would have gone silently inert; they
were found by reading archtest, not by a failing test -- re-check that class
when moving packages again.

## Phases (as planned)

Every move phase ships a committed, idempotent move script (`git mv`, import
rewrite, `goimports`, path rewrite in docs / AGENTS.md / seat prompt files),
lands with the queue drained, and runs the full suite. A stale branch is
rebased by re-running the script on it.

- **P0 — hygiene and onboarding (no moves, no pause).**
  - Delete: `cmd/autopayaudit`, `cmd/manamodelstats`, `deck.list` (+ its
    `.gitattributes` rule), unreferenced scripts (`compliance-printed.py`,
    `forge-oracle-test-{cast,setup}.sh`,
    `xmage-oracle-test-{driver-contract,forge-chunks}.sh`, `az-stage0.sh`,
    `m1b-paired.py`, `manamodel_ab_report.py`, `manamodel_report.py`,
    `encablate/summarize_keep.py`, `searchbench/{plot,report_tables}.py`,
    `spellbench-arena/*` except `rate.py`, `tests/test_perf_record.py`) —
    each re-checked for references before deletion.
    `.gitignore` gains `feedback/`.
  - README: fix the stale claims (test budget, orchestrator, dependency
    chain, hosted-bot vocabulary, spellbench-prep markers, fork source,
    `make test`, ratchet location, missing `core.hooksPath`); move the bot
    training section to `docs/research/`.
  - New: `CONTRIBUTING.md`; `docs/contributing/{first-card-fix,testing,
    glossary,card-pipeline}.md`. first-card-fix follows PR #3 and also the
    no-Go oracle-scenario route (`rules/testdata/oracle/`, `make oracle-audit`).
  - `doc.go` for `rules/` and `effects/`; consolidate the 42 duplicate
    corpus-lookup test helpers into one shared file.
- **P1 — cut fence edges (code, no moves).** Decisions 4–6: split
  `policynet`/`searchprobe`; move `Bot`/`PolicyNetBot` out of `seat`; host
  `seat.Factory` injection; gorged registration seam; land the archtest lab
  rule as a shrink-only allowlist.
- **P2 — Go moves, one drain window.** `lab/`, `bots/`, `server/`,
  `engine/`, `internal/` trim (`tsgen`/`codeshape`/`testbudget` beside their
  commands). Rename `event` → `draftevent`, `botpolicy` → `bots/policy`.
- **P3 — `ops/` move.**

## Not in scope

Splitting the flat `engine/rules/` directory (the lasagna work owns it). Moving
`lab/` to its own repository.

## Amendment (2026-10-16): the lab tree moves to a private companion module

- **Status:** accepted 2026-10-16, the same day as the layout
  above. Supersedes decision 2's "moved to another repo later", decisions 3
  and 5, and the *Not in scope* line about moving `lab/`. Everything above
  this section is the record as first written.

**What moved.** The fenced `lab/` tree left gorge for a new **private** module,
which requires gorge. With it went the research binaries, the hosted research
bots, the clairvoyant world source, the 15 lab design docs and the agent docs
that described only lab content.

**What gorge keeps.** The hosted bots `bot`, `lethal-pressure`,
`sb-tactical` and `az-redeal` (with the `azmcts` search core) and their
support packages: `botpolicy`, `heuristic`, `sanity`, `policynet`, `gonet`,
`traceboard`, `searchseat`, `searchprobe`, `spellbench` (the SpellBench deck
catalogs, moved from `lab/training/spellbench` to `bots/spellbench`, with
their generator `bots/spellbench/fdn-catalog.py`), `spellbench/builtins`,
`spellbench/payexec` and `bots/all`.

**The seams that replace the fence.** gorge never imports the companion
module; the companion composes gorge from outside:

- `server/gorged` is the importable server (`Main`, `Serve`). Its
  `Extensions{Events EventsPlugin}` carries the draft-event runtime; with no
  plugin, `-events` is refused at startup (`TestEventsNeedAPlugin`).
  `RegisterNetBot` links network-backed policies in and gives each its flags.
  `cmd/gorged` is a thin main over it that links gorge's `bots/all`.
- `bots.Register` is unchanged; a companion module links gorge's `bots/all` plus
  its own bots.
- `testkit/` (`testutil`, `bench`, `paymirror`) and `cli/` moved out of
  `internal/` so an outside module can import them.

**What replaced the enforcement.** The fence is now structural: the root
`go.mod` has no `require` lines (`TestRootModuleHasNoRequires`), so gorge
cannot import a companion. `internal/archtest/lab_fence_test.go` and the check
that engine tests never import the compliance packages were deleted as
vacuous, and the `lab` build tag is gone from gorge, its CI and its gates.

**Process changes.**

- The Makefile targets for the moved tools went with them.
- gorge drops its compliance gate.
- The docs workflow no longer measures set support (`oraclediff` is
  private); the public sets page shows its hand snapshot, and
  `scripts/docs_sets.py` stays in gorge as the renderer.
- Some contract goldens are owned by gorge packages (`bots/policynet`'s
  azvisits, `bots/gonet`'s dzgw header, `server/protocol`'s frames) while
  a companion module's contract registry pins their hashes, so changing one is a
  coordinated change across both repositories.

## Amendment 2 (2026-10-16): pipeline ops leave the repository

- **Status:** accepted 2026-10-16. Supersedes decision 1's "Agent-seat
  operating rules move under `ops/`" and the `ops/` line of *Target root*.

gorge, the public module, carries **no agent-pipeline or demo-deploy
machinery**. `ops/`, the pipeline configuration, docs, Makefile targets and
deploy-script tests moved to a private companion module; those tests check a
deploy's arguments through `gorged.ServeFlagSet`.

gorge keeps its contributor tooling: `.githooks/` (the package-classification
check and the opt-in `RACE=1` race gate), `scripts/` (`ci_shard.sh`,
`smoke.sh`, `enginebench-*`, `docs_sets.py`; enginebench takes a lock only
when `HEAVY_LOCK` is set), `cmd/testbudget`, the ratchets in `internal/`, and
CI.

## Amendment 3 (2026-10-16): compliance oracles move to gorge-compliance

- **Status:** accepted 2026-10-16; lands with PR #10.

**What moved.** A new **public** module and repository, gorge-compliance
(`github.com/adams-shaun/gorge-compliance`), backs up the rules engine: it
runs the three-way oracle (hand-authored Oracle-text scenarios, XMage,
Forge) and holds the coverage targets for a **pinned** gorge revision
(`GORGE_PIN`; a pin bump is a pull request there that reruns and updates the
targets, and a nightly job reports drift against gorge `main` without
failing). It has no functional use to engine users beyond confidence.

- From the companion module (moved, layout kept): `oracle/` (`compliance/**`,
  `validation/**`, `xmageoracle/**`), `cmd/oraclediff`, `cmd/compliance`,
  `cmd/xmageport`, `cmd/oraclepacket`, the `xmage-oracle-*`, `forge-*` and
  `compliance-pass` scripts, `driver_replay_batch.sh` and the oracle smokes,
  the three compliance designs, the `compliance-*`/`*-oracle-setup` Makefile
  targets with the XMage/Forge-oracle pins, and the CI `compliance` job.
  `internal/gorgedir` and `internal/boundary` are copied (both repositories
  run the licence guards).
- From gorge: the scenario corpora `engine/rules/testdata/oracle/**` (1,103
  of 1,112 files) and `engine/rules/testdata/xmageport/**` (220 files, with
  their XMage NOTICE), now gorge-compliance's `scenarios/oracle/` and
  `scenarios/xmageport/`; their drivers `oracle_audit_test.go`,
  `xmageport_test.go` and `oracle_unconsumed_filter_test.go`, rewritten as
  gorge-compliance's external `audit/` tests; and `make oracle-audit`.

**What gorge keeps.** The runner (`engine/rules/oracle_*.go`), every runner
unit test (including `oracle_unconsumed_test.go`, the leftover-answer
regression), the set audits, and the nine scenario files its own card tests
load (Wizard's Staff, Heirloom Epic, Gray Merchant of Asphodel, Rhovanion
Rampager, Supper for Spiders, Uncover the Moon-Letters, and the census
carriers Witness Protection, Purphoros and Bard's Company).
`oracle_contract_test.go` plays them through the exported API as the
runner's contract check. A scenario file lives in exactly one repository;
gorge-compliance's audit and gate read the union.

**The seam.** gorge-compliance uses only exported API. The hand audit needs
its own run mode and the log-only replay check the in-package driver had, so
`engine/rules/oracle_audit_export.go` adds `RunOracleAuditScenarioJSON(reg,
raw, fixture)` (returning `OracleAuditResult{Fails, Transcript,
ReplayDiff}`), `CheckOracleScenarioJSON(raw)` (corpus-free shape check) and
`OracleUnconsumedMarker`; the test helpers `replayFromLog`/`diffGames` became
thin wrappers over the non-test `logReplayGame`/`gameStateDiff` they now
share with it. Changing these is an API change for gorge-compliance.
gorge never imports gorge-compliance.
