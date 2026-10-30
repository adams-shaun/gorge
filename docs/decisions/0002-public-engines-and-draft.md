# 0002: dirac and draft are public; the line between public and private

- **Status:** accepted 2026-10-30 (operator). Execution: open-source program,
  workstreams A to H.
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
   experimentation stay in the private module four-yay: trainers, networks and
   weights, the az bots, `dzgorge`, `kplay`'s research keys, draft RL and its
   networks, `dirac`'s search and network evaluator until promoted, and operations
   and deploy scripts. Nothing public needs it to build or test. No new repository.
3. **Dependency direction.** gorge never imports four-yay. four-yay imports gorge
   at a pinned commit. The root `go.mod` still has no `require` line
   (`TestRootModuleHasNoRequires`); `TestPublicBoundary` adds a direct check.
4. **No git history is imported.** Moves are squashed commits with a provenance
   line naming the four-yay commit. four-yay remains the archive.
5. **Generated card tables are never committed.** `kgen`'s output is a mechanical
   translation of GPL-3.0 Forge scripts. It is written from `.cards` into a
   gitignored directory, as gorge's IR cache is. A fresh clone builds;
   corpus-dependent tests skip the way `testutil.CorpusRegistry` skips. Card names
   and CardID maps are facts and may be committed. No Forge script text may be
   committed, in any file, of any length, including a fragment in a test under
   `dirac/` or `draft/`. Existing gorge fixtures are a shrinking baseline.
6. **The ratchet.** `internal/archtest/public_boundary_test.go` fails on generated
   tables, binaries under `dirac/`, private paths, four-yay imports and
   unresolvable ticket tags in the moved trees. It has no allowlist.
7. **Third-party data stays attributed.** Every data source is named in `NOTICE`:
   17Lands public datasets (CC BY 4.0), MTGJSON (MIT), XMage-derived set lists
   (MIT), MageZero's encoder design (MIT), SpellBench (MIT). Deck pools are 17Lands
   derivatives under CC BY 4.0 and ship as an attributed release asset, not in git.
8. **Zero divergence.** A card dirac supports plays identically to gorge. See the
   program's decision 6 as recorded in `dirac/docs/verification.md`. gorge leads
   and dirac catches up; until then dirac refuses the card.

## Target root, additions to ADR 0001

    dirac/     kop cardset vanilla feat crcoverage search/{hbot,mull} oracle sbv2 cmd/ docs/
    draft/     limited/ event eventhost gorgedevents draftlab cmd/ examples/

## What this replaces in ADR 0001

- Decision 2 said draft events are extensions fenced under `lab/`. They are now
  platform.
- The first amendment's `server/gorged.Extensions{Events}` seam stays as an API:
  `cmd/gorged` passes the plugin, and `server/gorged` itself still does not import
  `draft/`, so `TestEventsNeedAPlugin` still holds for a bare library caller.
  `RegisterNetBot` and `bots.Register` stay for four-yay's az bots.
- "Private companion" in gorge's docs now means only the machine-learning and
  private-experiment work above. Where a doc says draft or dirty-dirac lives there,
  it is out of date from the commit that moves that tree.
- dirty-dirac is called dirac. The code path changes from `kernel/` to `dirac/`.

## Not in scope

Promoting `search` or `neteval`. Renaming dirac's `zz…`-prefixed files, whose order
is significant. Any change to gorge-compliance.
