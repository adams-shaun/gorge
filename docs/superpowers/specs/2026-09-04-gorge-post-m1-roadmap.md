# gorge — post-M1 roadmap

Date: 2026-09-04. Status: approved in brainstorm. From M2 onward this
document supersedes the milestone table in
`2026-09-03-mtgcore-go-engine-design.md`; every other section of that spec
still binds.

## Where the engine stands

M0 and M1 are complete on `main` (`2fe9c72`, plus one tests-only residual
commit from the merge-gate review).

| Measure | Value |
|---|---|
| Repo decks playable headless | 12 decks, 136 cards, at 2/4/6/8 seats |
| Acceptance chain heads | 2 seats `7705a6505954f6cd`, 4 `2d5589b31c4853cd`, 6 `bf4012092fdad38b`, 8 `01b9f48c1b6dc135` |
| `make sim` | 20/20 games replay byte-identically |
| `make report` | 33,667 corpus cards, 15,265 playable (45.3 %) |
| Coverage ratchet (`rules/acceptance_test.go` `knownUnsupported`) | 35 of 136 deck cards still listed |
| Primitives implemented | 81 of the 264 the curated pool needs |

## What changed since the engine spec

| Engine-spec item | Now |
|---|---|
| Module `mtgcore/` inside mtgbld (D7) | Own repo, module `github.com/adams-shaun/gorge`; packages `cards state decision events effects rules view seat replay cmd/*` |
| D5: WebSocket transport | **SSE downstream + POST upstream**, stdlib only (see the M2a spec, D10). WebSocket only on a measured need |
| Open question 4: in-process vs own service | **Both**: an embeddable `host` library mtgserve imports, plus a thin standalone `gorged` binary in this repo |
| M2 = "WebSocket protocol and a browser client" | Split into M2a–M2d below; spectators are a first-class requirement, not a by-product |
| Client contract | The existing mtgplay/mtgserve interface is **evidence of needs, not an API to copy**; the M2 interface is designed fresh |
| Format | **Legacy 4-player first** on the 12 repo decks; Commander is its own milestone |

## Milestones

| # | Deliverable | Done when | Runs |
|---|---|---|---|
| **M2a** | Tables & spectators: `host` library, `protocol` envelope, `gorged`, Svelte client (overview, focused table, DVR) | `gorged` runs 4 tables × 4 bots on repo decks, paced; overview → focus → omniscient board + stack + pending tray + log; DVR pause / step / return-to-live; late-join backfill; finished matches replay after a restart; perpetual tables; card images | Spec: `2026-09-04-gorge-m2a-tables-spectators-design.md` |
| **M2r** | Ratchet to zero: the 21 missing primitives for the 35 listed deck cards, plus the parked engine items below | `knownUnsupported` is empty; every one of the 12 decks plays every card; chain heads move only with a regenerated golden and a ledgered reason | Parallel, engine-only, own worktree |
| **M2b** | Player seat: a human at a seat through the browser | A human beats a scripted bot in a 4-player Legacy game; the client contains no legality logic (engine-spec M2 criterion) | After M2a |
| **M2c** | mtgserve integration | mtgserve hosts matches in-process through the `host` library; accounts, decks and match history come from mtgbld; the mtgbld snapshot god-view leak is impossible by construction because every payload is a `view.Project` for a seat | After M2b, in mtgbld |
| **M2d** | Commander | Command zone, commander tax, commander damage, 40 life, singleton + colour-identity validation, curated Commander decks and the coverage they need; the 4-tables-of-Commander demo from the original picture | After M2a and M2r |
| M3 | wasm plugin tier | Per engine spec | After M2r |
| M4 | Curated pool: 264 primitives, 1,657 cards | Per engine spec | After M3 |
| M5 | Perf + self-play; resume-from-log for `host` | Per engine spec, plus: a perpetual table survives a `gorged` restart mid-match | After M4 |
| M6 (stretch) | Standard-legal parity | Per engine spec | — |

M2a and M2r run in parallel worktrees. Their file surfaces barely overlap:
M2a adds `host/`, `protocol/`, `cmd/gorged/`, `cmd/gentypes/`, `web/` and
extends `view/`; M2r lives in `cards/`, `effects/`, `rules/`. Both touch the
`Makefile`. M2r's first task is the `rules/trigger.go` split, landed and merged
before anything else so M2a never rebases across it.

## M2r scope

Everything below is booked in the M0/M1 ledger with a ruling; the ratchet plan
picks them up in this order.

1. Split `rules/trigger.go` (1,062 lines, three concerns) — structural, first,
   merged alone (Ruling F-4).
2. The 21 primitives behind the 35 `knownUnsupported` cards, driven by the
   acceptance decks, ratchet shrinking with every task.
3. `Defined$ ReplacedCard` in replacement effects, so Rest in Peace–class cards
   exile instead of graveyarding (Ruling F-1 booked it; the totality guard
   `ensureLeftTheStack` stays as a backstop). Decide with it whether the
   coverage report should count `Defined$ ReplacedCard` as unsupported.
4. T19c-b: Equip/Attach, `TriggeredCard$`, and enumeration of non-mana
   activated abilities as options.
5. N2: `TargetMin$ 0` (343 corpus `A:SP$` lines) against the `spec != ""`
   fizzle gate.
6. Hygiene minors with a rules consequence: `effRepeat` unbounded repeat count;
   `mana.go` / `applyCountOp` int32 overflow; `applyReplacements` /
   `replacementMatches` split out of `sba.go`.

Not M2r: N4 and the CR 800.4a general case (a departed player's pending
decisions) and T21-h (multi-block last-blocker approximation) belong to M2b,
where a human at the table can observe them.

## Outside gorge

- **mtgbld prod leak**: `mtgserve/internal/matches/handlers.go:696-742` serves
  a god-view snapshot mid-game. Fix in mtgbld now; it does not wait for M2c.

## Risks

| Risk | Mitigation |
|---|---|
| Two worktrees on one young engine drift apart | M2r task 1 merges alone first; M2a never edits `rules/` except through `view/`'s public surface |
| Perpetual tables run for days | Everything per-match is bounded and rotated with the match (rings, snapshots, files); a soak test is part of M2a's done-when |
| Card images depend on a third party (Scryfall) | Name-based lookup with a client cache in M2a; a build-time printing table when M2c needs offline or bulk use |
| The client grows rules knowledge to look polished | The M2 gate from the engine spec stands: the client renders views and options, never computes legality; a test greps for it |

## Open questions

1. **Printing identity source.** Forge scripts carry no set/collector number.
   M2a resolves images by exact name at runtime; M2c decides whether a
   `forgec`-built printing table (from Scryfall bulk data) replaces that.
2. **mtgserve's UI after M2c**: embed gorge's `web/` build or keep mtgserve's
   own templates around the `host` library. Decide at M2c.
3. **Commander deck source** for M2d: curate from the user's lists or from a
   public precon set.
