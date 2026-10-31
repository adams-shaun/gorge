# Repo map — what lives where

A lookup table for agents and new contributors. One line per package or
directory, plus where to go when you need to change a particular kind of
thing. Verified against the tree on 2026-10-16 (after the
[ADR 0001](../decisions/0001-repo-layout.md) restructure, `go list ./...` and
`ls`); where this file and the code disagree, the code wins — fix this file in
the same commit. `go run ./cmd/pkgmap` prints every package with its purpose
and tier, generated from the source.

See also: [invariants.md](invariants.md) (the rules every change must keep)
and [do-not.md](do-not.md) (the mistakes that have actually happened here).

This is the index. The package and path rows live in the per-domain files
below; this file changes only when a GROUP or DOMAIN appears, not when a
package does.

## Groups

| Group | What it holds | Detail |
|---|---|---|
| `engine/` | The rules core: `cards`, `deck`, `state`, `decision`, `events`, `effects`, `rules` (+ sub-packages), `replay`, `eventdiff`, `view`, `game` (+`gorgegame`), and `seat` (the `Seat` contract only). Imports nothing outside `engine/`. | [repo-map-engine.md](repo-map-engine.md#engine) |
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

## Where do I change…

| I want to… | Go to |
|---|---|
| Implement an `api:` effect | `engine/effects/`, `effects.Register("Name", fn)` in an `init()`. If it needs more than `effects.Host` offers, it is rules work. |
| Implement a keyword/trigger/static/replacement | `engine/rules/`, `effects.RegisterNonAPI("kw:X", …)`. A keyword that expands to script lines is a `engine/cards/kw_*.go` expander instead (moves the fingerprint). A trigger mode's matcher goes in `engine/rules/trigmatch/`. |
| Add a `count:` value head | The evaluator arm in `engine/effects/` **and** `effects.modelledValueHeads` (`engine/effects/value_heads.go`; `TestValueHeadRegistryMatchesEvaluator`). |
| Add an event kind | Append to `engine/events/event.go` after the last constant and add its one `kindInfo` entry in `engine/events/kindinfo.go` (name, trigger class, optional Describe template). Behaviour (`events.Apply`, a custom `view.Describe` case) is separate. |
| Add a decision kind | Don't, without an operator decision — the set is closed; every seat, bot, client and the protocol must answer it. |
| Change the default bot's decisions | `bots/botpolicy/` (the one policy, reading a `Board`); its seat adapter is `bots/heuristic/bot.go`. |
| Add a bot policy | Bench-only: a `seat.Seat` driven through `testkit/bench` (the larger bench CLI lives outside the repo). Hosted: `bots.Register` in an `init()` (`bots/registry.go`), linked from `bots/all`. See [`docs/site/bots/training-and-adoption.md`](../site/bots/training-and-adoption.md) and [`docs/design/2026-09-28-hosted-bot-packages.md`](../design/2026-09-28-hosted-bot-packages.md). |
| Add a repo deck | `testkit/testutil/decks/*.json` via `cmd/deckimport`; it must be fully supported (the acceptance ratchet). |
| Turn a player report into a test | `go run ./cmd/repro -emit-test <pkg> <feedback-dir>`. |
| Change the wire | `server/protocol/`, then `make gentypes`. |
| Change draft events | The pure logic in `draft/limited/*` and `draft/event`; hosting in `draft/eventhost`; wire types in `server/protocol/draftevent.go`, then `make gentypes`. See [`docs/site/architecture/draft.md`](../site/architecture/draft.md). |
| Change an external file, wire or grammar (`.dzgw`, the azvisits corpus, the protocol goldens, the `gorge-agent/1` goldens) | Its owner package (`bots/gonet`, `bots/policynet`, `server/protocol`, `draft/draftlab/extern/wire`). A private companion module keeps the contract registry and pins these goldens' hashes, so a golden change here needs a matching update there (the maintainers handle it; say so in the PR). |
| Change a seat's projected view | The family file for the seam: a card/battlefield projection in `engine/view/card.go`, a stack projection in `engine/view/stack.go`. Only an assembler change (`Project`/`project`) belongs in `engine/view/view.go`, which two projection tickets once collided on. |
| Move or add a package | Classify it in `docs/site/architecture/packages.tiers.json` in the same commit (`go run ./cmd/pkgmap -check`, a pre-commit hook). |
| Add or change a living design | `docs/design/`. Historical records are parked; see [`docs/design/ARCHIVE.md`](../design/ARCHIVE.md). |
