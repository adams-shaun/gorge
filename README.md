# gorge

> [!NOTE]
> **Documentation: [adams-shaun.github.io/gorge](https://adams-shaun.github.io/gorge/)**
>
> | I want to… | Start at |
> |---|---|
> | understand how the engine works | [Architecture](https://adams-shaun.github.io/gorge/architecture/) |
> | run the demo or bot-vs-bot games | [User's guide](https://adams-shaun.github.io/gorge/guide/) |
> | connect a bot (Go, Python, Java) | [Bots](https://adams-shaun.github.io/gorge/bots/) and the [API reference](https://adams-shaun.github.io/gorge/api/) |
> | fix a card or a rule | [Contributing](https://adams-shaun.github.io/gorge/contributing/) |
> | compare speed or card coverage | [Performance](https://adams-shaun.github.io/gorge/performance/) and [Card support](https://adams-shaun.github.io/gorge/support/sets/) |

A deterministic, event-sourced *Magic: The Gathering* rules engine, written in
pure Go. Card behaviour is not hand-written per card: gorge compiles
[Forge](https://github.com/Card-Forge/forge)'s card scripts into an
intermediate representation (IR), and the engine implements the primitives
that IR references (effects, triggers, keywords, statics, value expressions).
A card is playable when every primitive it uses is implemented; today that is
about 95% of Forge's ~34,000 scripts (`make report` prints the live figure).

Around the engine sit a reference server (`gorged`: bot tables, play-vs-bot,
HTTP + server-sent events, a Svelte browser client), baseline and hosted bot
players, and developer tools for replaying, benchmarking and auditing games.
gorge plays full games and replays them byte for byte, but it is **not** yet
at rules parity with mature engines and is not ready for production use.

## The one design rule

Every state mutation goes through `events.Apply`. Nothing writes a
`state.Game` field except by emitting an event into the hash-chained log. So a
match is fully described by its config plus its event log: replaying a log and
resuming a match from any point in it are the same operation, and search
clones and bug reproductions reuse that one mechanism. A write that bypasses
`events.Apply` is a replay bug.

## Quickstart

You need Go (the version in `go.mod`), `git`, `make` and `bash`.

```sh
make fetch-cards compile-cards               # one-time: Forge corpus (~240 MB) into .cards/, then the IR cache
go test -run 'TestHeads$' ./engine/rules/    # a focused test; corpus tests SKIP without .cards/
make sim                                     # 20 verified 4-seat bot games; each prints "replay OK"
go build -o bin/gorged ./cmd/gorged
bin/gorged -addr :8090 -dir /tmp/gorge-local -tables 2 -seats 4 -vsbot   # then open http://localhost:8090/
```

The browser client is built separately with `make web` (needs Node); until
then the web root answers `503`, but the `/api/*` endpoints work. Never point
two servers at one `-dir`.

## Repository map

| Directory | What it holds | Who it is for |
|---|---|---|
| `engine/` | The deterministic rules core: `cards` (script → IR compiler), `state`, `decision`, `events`, `effects`, `rules`, `view`, `replay`, `deck`, and `seat` (the `Seat` contract only) | Anyone fixing a card or a rule |
| `server/` | The reference server: `gorged` (the importable server, with an `Extensions` seam and `RegisterNetBot`), `host` (tables, sessions, persistence) with `host/httpapi` and `host/manabrewhttp`, `protocol` (the wire types), `manabrew`, `divergence` | Server and client work |
| `bots/` | Bot players: `botpolicy`, `heuristic` (the default bot), the `registry`, and the hosted bots (`bot`, `lethal-pressure`, `forge-heuristic`, `forge-heuristic-hold`, `sb-tactical`, `az-redeal`) with their inference code | Bot authors |
| `draft/` | The limited stack: `limited` (boosters, pods, deck building, pick bots, set data), `event`, `eventhost`, `gorgedevents` (the plugin `cmd/gorged` links, serving events with `-events`) and `draftlab` with its external-agent wire | Draft and limited work |
| `web/` | The TypeScript/Svelte browser client | Front-end work |
| `cmd/` | Product and developer binaries: `gorged`, `forgec`, `repro`, `mtgsim`, `gentypes`, `pkgmap`, `headdiff`, `enginebench`, `cardfuzz`, `deckimport`, … | Everyone |
| `testkit/`, `cli/` | Public support packages for outside modules: `testkit/testutil` (fixtures, corpus registry), `testkit/bench` (the game runner), `testkit/paymirror`, and `cli` (command-line flags, env and config) | Contributors, downstream modules |
| `internal/` | Ratchets and generators: `archtest` (import-graph rules), `codeshape`, `testbudget`, `tsgen`, `corpusheap` | Contributors |
| `scripts/` | Contributor and CI scripts (`smoke.sh`, `enginebench-*`, `ci_shard.sh`, `docs_sets.py`) | Contributors, CI |
| `docs/` | `decisions/` (ADRs), `site/` (the public docs site), `agents/` (the guide for AI coding agents) | Everyone |

Dependencies run one way, in tiers: `engine/` imports nothing above it,
`server/` and `bots/` build on `engine/`, and `cmd/` sits on top.
What is actually enforced is a list of forbidden import edges (for example,
`effects` never imports `rules`, `view` and `protocol` never import `rules`)
in `internal/archtest`'s `TestDependencyOrderHolds`, not a total order. The
layout and its reasons are in [ADR 0001](docs/decisions/0001-repo-layout.md).

### The private companion module

Machine learning, reinforcement learning and private experiments live in a
**private** companion module that imports gorge at a pinned commit; gorge never
imports it. It plugs in through `bots.Register` and
`server/gorged.RegisterNetBot`. Nothing in gorge needs it to build or test.

### gorge-compliance, the public compliance repository

The compliance oracles (gorge vs XMage vs Forge, card by card), the
hand-authored Oracle-text scenario audit, the XMage-ported interaction
scenarios and their coverage targets live in
[gorge-compliance](https://github.com/adams-shaun/gorge-compliance)
(`github.com/adams-shaun/gorge-compliance`, public). It measures a **pinned**
gorge revision through gorge's exported oracle runner
(`rules.RunOracleScenarioJSON`, `rules.RunOracleAuditScenarioJSON`,
`rules.CheckOracleScenarioJSON`); gorge never imports it and does not gate on
it. gorge keeps the runner, its unit tests and nine residual scenario files
its own card tests load (`engine/rules/testdata/oracle/`), played by
`engine/rules/oracle_contract_test.go` as the runner's contract check.

## Where to go next

- **Contributing:** [CONTRIBUTING.md](CONTRIBUTING.md) covers setup, the rules
  that matter, testing and the PR checklist. Fixing a card that plays wrong is
  walked through in
  [docs/site/contributing/first-card-fix.md](docs/site/contributing/first-card-fix.md).
- **Docs site:** <https://adams-shaun.github.io/gorge/> has the architecture,
  per-set card support, performance numbers and the contributor guides
  (sources in [docs/site/](docs/site/)).
- **Design decisions:** [docs/decisions/](docs/decisions/).
- **If you are an AI agent:** start at [AGENTS.md](AGENTS.md) and
  [docs/agents/](docs/agents/README.md).

## Bot player training and adoption guidelines

Moved to [docs/site/bots/training-and-adoption.md](docs/site/bots/training-and-adoption.md):
the decision contract a bot answers, the engine's search and RL capabilities
and the adoption ladder a policy climbs before it plays real users. Writing a
bot in Go, Python or Java starts at [docs/site/bots/](docs/site/bots/index.md)
(published at https://adams-shaun.github.io/gorge/bots/).

## License

gorge is licensed under Apache-2.0 (see [LICENSE](LICENSE)). Forge's card scripts are GPL-3.0, so gorge ships a
*compiler* for the script format and never the scripts themselves:
`make fetch-cards` downloads them into the gitignored `.cards/` directory,
pinned to a commit (`FORGE_REF` in the `Makefile`) of the
[adams-shaun/forge](https://github.com/adams-shaun/forge) fork, and
`engine/cards/boundary_test.go` fails the build if a script is ever tracked.
kgen's generated card tables are also never committed: they are built from
`.cards` into a gitignored directory. `NOTICE` names every third-party data
source, including 17Lands public datasets (CC BY 4.0).
