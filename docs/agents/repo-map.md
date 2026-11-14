# Repo map — what lives where

A lookup table for agents and new contributors. One line per package or
directory, plus where to go when you need to change a particular kind of
thing. Where this file and the code disagree, the code wins — fix this file in
the same commit. `go run ./cmd/pkgmap` prints every package with its purpose
and tier, generated from the source.

See also: [invariants.md](invariants.md) (the rules every change must keep)
and [do-not.md](do-not.md) (the mistakes that have actually happened here).

This is the index. The package and path rows for the other groups live in the
per-domain files below; the `dirac/` rows are here because it is a domain of its
own. This file changes only when a GROUP or DOMAIN appears, not when a package
does.

## Groups

| Group | What it holds | Detail |
|---|---|---|
| `engine/` | The rules core: `cards`, `deck`, `state`, `decision`, `events`, `effects`, `rules` (+ sub-packages), `replay`, `eventdiff`, `view`, `game` (+`gorgegame`), and `seat` (the `Seat` contract only). Imports nothing outside `engine/`. | [repo-map-engine.md](repo-map-engine.md#engine) |
| `dirac/` | The second engine, in lockstep with `engine/` and built for machine learning: the two-player `kop`-table engine, its oracle seam to gorge, its table generator, its adapter and its commands. The core imports nothing outside `dirac/`; nothing in `engine/` imports it. | [below](#dirac) |
| `bots/` | `botpolicy`, the default `heuristic` bot, the bots registry and the hosted bots with their inference runtime. Imports `engine/` only. | [repo-map-engine.md](repo-map-engine.md#bots) |
| `server/` | `gorged` (the importable server: `Main`, `Extensions`, `RegisterNetBot`; `cmd/gorged` is its thin main), `host` (+`httpapi`, `manabrewhttp`), `protocol`, `manabrew`, `divergence`. | [repo-map-engine.md](repo-map-engine.md#server) |
| `draft/` | The limited stack: `limited` (+`setdata`, `booster`, `draft`, `build`, `pickbot`), `event`, `eventhost`, `gorgedevents`, `draftlab` (+`extern`). `cmd/gorged` links it; only `cmd/gorged`, `cmd/draftsim`, `cmd/labdemo` and `cmd/extagent` may depend on it. | [repo-map-engine.md](repo-map-engine.md#draft) |
| `internal/`, `testkit/`, `cli/` | `internal/`: architecture ratchets and generators. `testkit/` (test support, the shared bench runner, paymirror) and `cli/` (the command-line parser) are public so outside modules can use them. | [repo-map-internal.md](repo-map-internal.md) |
| `cmd/` | Product and developer binaries. | [repo-map-paths.md](repo-map-paths.md#cmd) |
| private companion (not in this repo) | Machine learning, reinforcement learning and private experiments. A private module that imports gorge at a pinned commit; gorge never imports it, and nothing in gorge needs it to build or test. Seams: `bots.Register`, `server/gorged.RegisterNetBot` ([ADR 0002](../decisions/0002-public-engines-and-draft.md)). | (private) |
| gorge-compliance (not in this repo) | The compliance oracles (gorge vs XMage vs Forge), the hand Oracle-text scenario audit and the XMage-ported scenarios, with their coverage targets for a pinned gorge. Public (`github.com/adams-shaun/gorge-compliance`); imports gorge's exported oracle runner, gorge never imports it. gorge keeps the runner (`engine/rules/oracle_*.go`) and the residual `engine/rules/testdata/oracle/` files its card tests load. | [ADR 0001](../decisions/0001-repo-layout.md) amendment 3 |
| `scripts/` | Contributor and CI helpers only. | [repo-map-paths.md](repo-map-paths.md#everything-else) |
| `web/` | The Svelte + Vite client. | [repo-map-paths.md](repo-map-paths.md#everything-else) |
| `docs/` | This guide, living designs (`docs/design/`), decisions (`docs/decisions/`), the public site (`docs/site/`). | [repo-map-paths.md](repo-map-paths.md#everything-else) |

## `dirac/`

gorge's second engine: two-player, driven as a `Step`/`Apply` state machine over
a `kop.Table`, and fast enough for self-play, search and policy training. gorge
leads and dirac follows decision by decision; a card dirac supports must play
identically, and anything outside the generated tables stays with gorge. The
core imports nothing outside `dirac/`. [`dirac/README.md`](../../dirac/README.md)
has the API, the rules for code that touches `State`, and the import boundary.

| Package | What it is | Where a change goes |
|---|---|---|
| `dirac` | The engine: state, `Step`/`Apply`, combat, stack, mulligans, projection, and the interpreter for the `kop` ops. | Start at `doc.go`. One `<topic>_<mechanic>.go` per mechanic; never append to a shared tail. |
| `dirac/kop` | The card-effect vocabulary: `Table`, `CardDef`, `Ability`, `Instr` and the op enum the tables and interpreter share. | A new op or ext sub-op goes in its own `kop` file with an `init` name and an unused literal; never renumber. |
| `dirac/cardset` | The engine-neutral `Set` interface and the loader of binary card tables (`ByName`, `Names`); imports `dirac/kop` and the root table key. | Loader in `load.go`; committed pool inputs under `cardset/pools/`. |
| `dirac/vanilla` | A hand-written table of basic lands, vanilla creatures and simple spells, with synthetic decks; the small-table test bed. | This table only; keep it hand-written so `example_test.go` needs no generated tables. |
| `dirac/feat` | The network feature encoder: a dirac position to the same `policynet` inputs gorge produces. | Encoder and its policynet parity tests. |
| `dirac/crcoverage` | Test-only CR coverage audit, one file per CR chapter; `TestCRCoverageIsCurrent` checks every row against the code. | The affected rule's row, in the same change as the mechanic. |
| `dirac/search/hbot` | The native port of gorge's auto-pay heuristic bot. | Mirror a gorge auto-pay change here; lockstep is the check. |
| `dirac/search/mull` | gorge's London-mulligan policies as pure functions over a hand. | The same policy change as gorge's, kept pure. |
| `dirac/neteval` | `dirac/search`'s network `Evaluator`: the `dirac/feat` encoder feeding an in-process backend. | The evaluator and its parity tests. |
| `dirac/oracle` | The engine-neutral seam to gorge: `Side`, `GorgeSide`, `RealKernel`/`FDNSide`, `Lockstep`, `Fuzz`, `Replay`, `Repro` and the policies. | Lockstep and fuzz logic here; its commands live under `oracle/cmd`. |
| `dirac/kgen` | Lowers gorge's card IR into `kop` tables. | A `names_<mechanic>.go` and/or `opsdoc_<mechanic>.go`; do not edit `names.go` or `opsdoc.go`. |
| `dirac/adapter` | A dirac game behind `engine/game`: what `gorged -engine dirac` serves, with a log and views derived from positions. | The adapter; it reads gorge's types and never `engine/rules`, `engine/replay` or `dirac/oracle`. |
| `dirac/canon` | The canonical vocabulary the oracle and the adapter share, and dirac's translation into it. | `translate.go`, `faces.go` and `names.go`; `chooseKinds` stays one shared list. |
| `dirac/support` | Which cards are supported: the `evidence` and `refused` lists. | Generated from evidence runs by `scripts/dirac-first-refusals.py`; never by hand. |

| Command | What it is | Where a change goes |
|---|---|---|
| `dirac/cmd/kgen` | Writes the binary card tables under the corpus (`make dirac-tables`). | Table generation in `dirac/kgen`; keep this main thin. |
| `dirac/cmd/dplay` | Plays games on dirac alone; reads gorge deck files, and its bots read gorge's compiled cards through `dirac/oracle`. | Its flags are pinned by `TestDplayFlagsDocumented` (`dirac/docs/running.md`); bot play goes through `dirac/oracle`. |
| `dirac/cmd/sbv2dirac` | Serves dirac as a SpellBench v2 engine, running gorge as the lockstep shadow. | The protocol and main here; parity in `dirac/oracle`. |
| `dirac/oracle/cmd/kfuzz` | The lockstep fuzzer over gorge and dirac. | `dirac/oracle`; `scripts/dirac-gate.sh` holds what the change must keep true. |
| `dirac/oracle/cmd/khbot` | dirac's heuristic-bot parity run. | `dirac/search/hbot` and the `dirac/oracle` policies. |
| `dirac/oracle/cmd/ksbt` | dirac's sb-tactical parity run. | The `dirac/oracle` policies. |

## Where do I change…

| I want to… | Go to |
|---|---|
| Implement an `api:` effect | `engine/effects/`, `effects.Register("Name", fn)` in an `init()`. If it needs more than `effects.Host` offers, it is rules work. |
| Implement a keyword/trigger/static/replacement | `engine/rules/`, `effects.RegisterNonAPI("kw:X", …)`. A keyword that expands to script lines is a `engine/cards/kw_*.go` expander instead (moves the fingerprint). A trigger mode's matcher goes in `engine/rules/trigmatch/`. |
| Add a `count:` value head | The evaluator arm in `engine/effects/` **and** `effects.modelledValueHeads` (`engine/effects/value_heads.go`; `TestValueHeadRegistryMatchesEvaluator`). |
| Add an event kind | Append to `engine/events/event.go` after the last constant and add its one `kindInfo` entry in `engine/events/kindinfo.go` (name, trigger class, optional Describe template). Behaviour (`events.Apply`, a custom `view.Describe` case) is separate. |
| Add a decision kind | Don't, without a maintainer decision — the set is closed; every seat, bot, client and the protocol must answer it. |
| Change the default bot's decisions | `bots/botpolicy/` (the one policy, reading a `Board`); its seat adapter is `bots/heuristic/bot.go`. |
| Add a bot policy | Bench-only: a `seat.Seat` driven through `testkit/bench`. Hosted: `bots.Register` in an `init()` (`bots/registry.go`), linked from `bots/all`. See [`docs/site/bots/training-and-adoption.md`](../site/bots/training-and-adoption.md) and [`docs/design/2026-09-28-hosted-bot-packages.md`](../design/2026-09-28-hosted-bot-packages.md). |
| Add a repo deck | `testkit/testutil/decks/*.json` via `cmd/deckimport`; it must be fully supported (the acceptance ratchet). |
| Turn a player report into a test | `go run ./cmd/repro -emit-test <pkg> <feedback-dir>`. |
| Change the wire | `server/protocol/`, then `make gentypes`. |
| Change draft events | The pure logic in `draft/limited/*` and `draft/event`; hosting in `draft/eventhost`; wire types in `server/protocol/draftevent.go`, then `make gentypes`. See [`docs/site/architecture/draft.md`](../site/architecture/draft.md). |
| Change an external file, wire or grammar (`.dzgw`, the visit corpus, the protocol goldens, the `gorge-agent/1` goldens) | Its owner package (`bots/gonet`, `bots/policynet`, `server/protocol`, `draft/draftlab/extern/wire`). A private companion module keeps the contract registry and pins these goldens' hashes, so a golden change here needs a matching update there (say so in the PR). |
| Change a seat's projected view | The family file for the seam: a card/battlefield projection in `engine/view/card.go`, a stack projection in `engine/view/stack.go`. Only an assembler change (`Project`/`project`) belongs in `engine/view/view.go`. |
| Add a dirac cold array (state only some tables need) | The feature's own file, never `dirac/statecopy.go` or `dirac/arena.go`: `var coldX = newCold[T]("x", func(g *genPrep) int { if g.anyX { return n }; return 0 })`, read through `coldX.of(s)` (nil when the table has none). Chain `.perSlot()` for a per-battlefield-slot array, which is then cleared as the slot takes a new permanent. `.view(func(c *coldArrays) *[]T { return &c.x })` plus a `coldArrays` field is only for a hot path, and it grows `State` by a slice header (re-pin both `TestStateLayoutPinned` and `TestStateLayoutPinnedMP`, saying why). No `coldLayout` field, no `prepCold` line, no `place` line, no offset walk. See [`docs/site/architecture/dirac/state.md`](../site/architecture/dirac/state.md#hot-and-cold). |
| Move or add a package | Classify it in `docs/site/architecture/packages.tiers.json` in the same commit (`go run ./cmd/pkgmap -check`, a pre-commit hook). |
| Add or change a living design | `docs/design/`. Historical records are parked; see [`docs/design/ARCHIVE.md`](../design/ARCHIVE.md). |
