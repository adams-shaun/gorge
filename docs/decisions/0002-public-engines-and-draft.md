# 0002: dirac and draft are public; the line between public and private

- **Status:** accepted 2026-10-30.
- **Amends:** ADR 0001 decision 2 (identity), decision 3 and its amendment's
  fence (draft moves back inside), decision 5 (draft seam) and the first
  amendment's "What moved" and "seams" for draft and for
  `cmd/gorged`. Amendment 2 (pipeline ops) and amendment 3 (gorge-compliance)
  stand unchanged.

## Why

gorge's goal is traceability and reasonable speed. dirac, the second engine,
exists to be fast enough for machine learning and is judged decision by decision
against gorge. Both, the limited stack and a demo server are useful to
outsiders, and a standalone dirac repository would be a module cycle: its oracle
and `kgen` import gorge's engine, and `gorged` will import dirac's adapter.

## Decisions

1. **Identity.** gorge is two open engines in one module,
   `github.com/adams-shaun/gorge` (Apache-2.0): `engine/` and `dirac/`, plus the
   limited stack `draft/`, the reference server, web client, baseline bots and
   tools. `cmd/gorged` links draft events directly, so the public binary is the
   demo server.
2. **What is private.** Machine learning, reinforcement learning and private
   experimentation stay in a private companion module: trainers, networks and
   weights, the az bots, `dzgorge`, the private self-play runner's research keys, draft RL and its
   networks, `dirac`'s search and network evaluator until promoted, and operations
   and deploy scripts. Nothing public needs it to build or test. No new repository.
3. **Dependency direction.** gorge never imports the companion module, which imports
   gorge at a pinned commit. The root `go.mod` still has no `require` line
   (`TestRootModuleHasNoRequires`); `TestPublicBoundary` adds a direct check.
4. **No git history is imported.** Moves are squashed commits with a provenance
   line naming the companion module's commit, which remains the archive.
5. **Generated card tables are never committed.** `kgen`'s output is a mechanical
   translation of GPL-3.0 Forge scripts. It is written from `.cards` into a
   gitignored directory, as gorge's IR cache is. A fresh clone builds;
   corpus-dependent tests skip the way `testutil.CorpusRegistry` skips. Card names
   and CardID maps are facts and may be committed. No Forge script text may be
   committed, in any file, of any length, including a fragment in a test under
   `dirac/` or `draft/`. Existing gorge fixtures are a shrinking baseline.
6. **The ratchet.** `internal/archtest/public_boundary_test.go` fails on generated
   tables, binaries under `dirac/`, private paths, private imports and
   unresolvable ticket tags in the moved trees and the draft commands. It has no allowlist.
7. **Third-party data stays attributed.** Every data source is named in `NOTICE`:
   17Lands public datasets (CC BY 4.0), MTGJSON (MIT), XMage-derived set lists
   (MIT), MageZero's encoder design (MIT), SpellBench (MIT), mtg-kernel's pauper deck
   catalog (MIT). Deck pools are 17Lands derivatives under CC BY 4.0 and ship as an
   attributed release asset, not in git. *(Superseded 2026-11-05: deck pools
   are not published as a release asset; see the amendment below.)*
8. **Zero divergence.** A card dirac supports plays identically to gorge, decision
   by decision. gorge leads and dirac catches up; until it has, dirac refuses the
   card.

## Target root, additions to ADR 0001

    dirac/     kop cardset vanilla feat crcoverage search/{hbot,mull} oracle kgen cmd/ docs/ support/
    draft/     limited/ event eventhost gorgedevents draftlab
    cmd/       adds draftsim labdemo extagent
    examples/  adds extagent

## dirac

`dirac/` holds a second rules engine: a fixed-layout core plus one pointer-free
variable buffer per state, binary card tables and a transition loop that allocates
nothing. `dirac/oracle` plays it against `engine/rules` decision by decision.

- **Boundary.** The core (`dirac`, `kop`, `cardset`, `vanilla`, `feat`, `search/*`)
  imports nothing outside `dirac/` but the module root's embedded table sources
  (`TestDiracCoreImportsNoGorge`). `engine/` imports nothing from `dirac/`
  (`TestEngineTierImportsNeitherDraftNorDirac`). Only `dirac/oracle`, `dirac/kgen` and
  the tests that read synthetic pools reach the raw tables
  (`TestOnlyTheOracleReadsRawTables`).
- **Tables.** `make dirac-tables` writes them under the gitignored `.cards`, keyed by
  corpus and kgen sources. They are never committed (`TestNoGeneratedTablesTracked`).
  A clone without a corpus builds and its corpus-dependent tests skip
  (`make fresh-clone-check`).
- **Support.** A card is supported when kgen lowers it, it is not refused by name in
  `dirac/support/refused/` and it has evidence in `dirac/support/evidence/`: clean
  lockstep games. `dirac/docs/support.md` is generated from those lists.
- **Gate.** `scripts/dirac-gate.sh <base>` runs on a change to dirac or to anything
  dirac's oracle plays against. The whole oracle runs before a merge to `main`, not
  per change.

## What this replaces in ADR 0001

- Decision 2 said draft events are extensions fenced under `lab/`. They are now
  platform.
- The first amendment's `server/gorged.Extensions{Events}` seam stays as an API:
  `cmd/gorged` passes the plugin, and `server/gorged` itself still does not import
  `draft/`, so `TestEventsNeedAPlugin` still holds for a bare library caller.
  `RegisterNetBot` and `bots.Register` stay for the companion module's bots.
- "Private companion" in gorge's docs now means only the machine-learning and
  private-experiment work above. Where a doc says draft or dirac lives there,
  it is out of date from the commit that moves that tree.
- The second engine is called dirac. The code path changes from `kernel/` to `dirac/`.

## Not in scope

Any other part of the private module. Any change to gorge-compliance.

## Amendment (2026-11-03): the demo moves in whole

Everything the public demo serves now builds from this repository alone, so
the demo binary is `cmd/gorged` and nothing demo-related stays private.

- **Moved in.** The search teacher (`bots/search`), sb-search
  (`bots/spellbench/sbsearch` and its hosted adapter `bots/sbsearch`), the
  SpellBench builtins' hosted adapter (`bots/sbbuiltin`), cast-profile
  (`bots/castprofile`), their host-level gates (`bots/hosttest`), and the demo
  deploy (`scripts/deploy-demo.sh`, `make deploy-demo`, `stop-demo`,
  `prewarm-art`). Decision 2's "the az bots" and "deploy scripts" no longer
  name anything private; the rest of its list stands.
- **Linked by the binary, not by `bots/all`.** `cmd/gorged` links the moved
  policies beside `bots/all`, so `server/host`'s registry-wide gates keep the built-in
  set's size and cost.
- **The deploy's settings stay out of git.** The deploy reads its ports and
  directories from the untracked `deploy/demo.env` (`deploy/demo.env.example`
  documents every variable).

## Amendment (2026-11-04): the research record is public

Decision 2 keeps machine learning private. The record of it is not: its results,
findings, architecture drawings and per-set card strength lists are published in
`docs/site/research/`. The trainers, networks and weights, and the experiment
code and data stay in the companion module, so the pages describe that code in
words and name no private path. `TestPublicBoundary` holds them to the same
boundary as every other file.

## Amendment (2026-11-05): deck pools are not a release asset

Decision 7 said deck pools ship as an attributed release asset. They do not:
the full deck pools are not published and have no download. The committed pool
inputs under `dirac/cardset/pools/` and the committed slices are what a clone
has. The original text of decision 7 stays above, marked superseded.

## Amendment (2026-11-05): dirac's search and network evaluator are promoted

Decision 2 kept "`dirac`'s search and network evaluator until promoted"
private. They were promoted on 2026-11-05, in commit
`359e84a0e9cd6ac48a672e95909c06abda8d9cef` ("Import dirac's search, its
network evaluator and the search oracle tests"):

- **`dirac/search`** — the AlphaZero-style search over a `dirac.State`, with
  its option families. It imports only `dirac` packages, so it sits in
  dirac's core.
- **`dirac/neteval`** — the search's network Evaluator, fed by `dirac/feat`
  and feeding `bots/netsimd` or `bots/gonet`. It imports `bots/`, so it sits
  outside the core. The remote backend (a client for a private network
  server) was left in the companion module.
- **`dirac/oracle/searchtest`** — the lockstep tests that need the search or
  the evaluator, which `dirac/oracle` may not import.

Decision 2's list no longer names either package; the rest of the list
stands, and `TestPublicBoundary` holds the promoted packages to the public
boundary like every other file.
