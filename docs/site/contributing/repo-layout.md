# Repository layout

This page maps gorge's source tree for a Go developer who has just cloned it:
what each top-level directory holds, which way imports are allowed to run,
and where a given kind of change belongs. The per-package detail (every
package with its purpose, size and importers) is generated from the source on
[Packages](../architecture/packages.md); this page is the guided tour.

## The tree at a glance

<figure>
<svg viewBox="0 0 760 468" width="100%" role="img" aria-label="gorge's source tree as a layered stack: cmd on top, then draft, server and bots in the middle, engine at the bottom; testkit, cli and internal beside them as tooling; web, scripts and docs as non-Go paths.">
<defs><marker id="rl-arr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" class="dg-arrow"/></marker></defs>
<rect x="10" y="10" width="740" height="58" rx="8" class="dg-box"/>
<text x="24" y="34" class="dg-text-bold">cmd/</text>
<text x="70" y="34" class="dg-text">product and developer binaries</text>
<text x="24" y="54" class="dg-text-muted">gorged  forgec  mtgsim  repro  headdiff  pkgmap  cardfuzz  enginebench  draftsim  labdemo ...</text>
<line x1="270" y1="68" x2="270" y2="94" class="dg-line" marker-end="url(#rl-arr)"/>
<rect x="10" y="98" width="520" height="58" rx="8" class="dg-box"/>
<text x="24" y="122" class="dg-text-bold">draft/</text>
<text x="82" y="122" class="dg-text">the limited stack, linked by cmd/gorged</text>
<text x="24" y="142" class="dg-text-muted">limited  event  eventhost  gorgedevents  draftlab</text>
<g transform="translate(0,88)">
<rect x="10" y="98" width="250" height="78" rx="8" class="dg-box"/>
<text x="24" y="122" class="dg-text-bold">server/</text>
<text x="24" y="142" class="dg-text-muted">gorged  host (+httpapi)  protocol</text>
<text x="24" y="160" class="dg-text-muted">manabrew  divergence  spellbench</text>
<rect x="280" y="98" width="250" height="78" rx="8" class="dg-box"/>
<text x="294" y="122" class="dg-text-bold">bots/</text>
<text x="294" y="142" class="dg-text-muted">registry  botpolicy  heuristic</text>
<text x="294" y="160" class="dg-text-muted">hosted bots and their search / nets</text>
<rect x="10" y="206" width="520" height="164" rx="8" class="dg-box-accent"/>
<text x="24" y="230" class="dg-text-bold">engine/</text>
<text x="88" y="230" class="dg-text">the rules core: imports nothing outside engine/</text>
<rect x="24" y="246" width="74" height="34" rx="5" class="dg-box"/><text x="61" y="268" text-anchor="middle" class="dg-text-mono">cards</text>
<rect x="107" y="246" width="74" height="34" rx="5" class="dg-box"/><text x="144" y="268" text-anchor="middle" class="dg-text-mono">state</text>
<rect x="190" y="246" width="74" height="34" rx="5" class="dg-box"/><text x="227" y="268" text-anchor="middle" class="dg-text-mono">decision</text>
<rect x="273" y="246" width="74" height="34" rx="5" class="dg-box"/><text x="310" y="268" text-anchor="middle" class="dg-text-mono">events</text>
<rect x="356" y="246" width="74" height="34" rx="5" class="dg-box"/><text x="393" y="268" text-anchor="middle" class="dg-text-mono">effects</text>
<rect x="439" y="246" width="78" height="34" rx="5" class="dg-box"/><text x="478" y="268" text-anchor="middle" class="dg-text-mono">rules</text>
<rect x="24" y="290" width="74" height="34" rx="5" class="dg-box"/><text x="61" y="312" text-anchor="middle" class="dg-text-mono">deck</text>
<rect x="273" y="290" width="74" height="34" rx="5" class="dg-box"/><text x="310" y="312" text-anchor="middle" class="dg-text-mono">view</text>
<rect x="356" y="290" width="74" height="34" rx="5" class="dg-box"/><text x="393" y="312" text-anchor="middle" class="dg-text-mono">seat</text>
<rect x="439" y="290" width="78" height="34" rx="5" class="dg-box"/><text x="478" y="312" text-anchor="middle" class="dg-text-mono">replay</text>
<text x="24" y="352" class="dg-text-muted">in the top row each package imports only those to its left; events.Apply is the only state mutator</text>
<line x1="135" y1="68" x2="135" y2="94" class="dg-line" marker-end="url(#rl-arr)"/>
<line x1="405" y1="68" x2="405" y2="94" class="dg-line" marker-end="url(#rl-arr)"/>
<line x1="260" y1="137" x2="276" y2="137" class="dg-line" marker-end="url(#rl-arr)"/>
<line x1="135" y1="176" x2="135" y2="202" class="dg-line" marker-end="url(#rl-arr)"/>
<line x1="405" y1="176" x2="405" y2="202" class="dg-line" marker-end="url(#rl-arr)"/>
<rect x="560" y="10" width="190" height="86" rx="8" class="dg-box"/>
<text x="574" y="34" class="dg-text-bold">testkit/  cli/</text>
<text x="574" y="54" class="dg-text-muted">test fixtures, game runner,</text>
<text x="574" y="70" class="dg-text-muted">flags; importable by other</text>
<text x="574" y="86" class="dg-text-muted">modules</text>
<rect x="560" y="106" width="190" height="70" rx="8" class="dg-box"/>
<text x="574" y="128" class="dg-text-bold">internal/</text>
<text x="574" y="148" class="dg-text-muted">archtest, codeshape, tsgen,</text>
<text x="574" y="164" class="dg-text-muted">testbudget, corpusheap</text>
<rect x="560" y="206" width="190" height="164" rx="8" class="dg-box-muted"/>
<text x="574" y="230" class="dg-text-bold">not Go packages</text>
<text x="574" y="254" class="dg-text-mono">web/</text><text x="630" y="254" class="dg-text-muted">Svelte client</text>
<text x="574" y="278" class="dg-text-mono">scripts/</text><text x="646" y="278" class="dg-text-muted">CI, smoke</text>
<text x="574" y="302" class="dg-text-mono">docs/</text><text x="630" y="302" class="dg-text-muted">this site, ADRs</text>
<text x="574" y="326" class="dg-text-mono">.githooks/</text>
<text x="574" y="350" class="dg-text-mono">.github/</text><text x="646" y="350" class="dg-text-muted">CI, docs</text>
</g>
</svg>
<figcaption>Arrows point from the importer to the imported layer.</figcaption>
</figure>

## Top-level paths

| Path | What it holds |
|---|---|
| `engine/` | The deterministic rules core: `cards` (the Forge-script compiler and card registry), `state`, `decision`, `events` (the event log and `events.Apply`), `effects`, `rules` (the engine, with sub-packages `chars`, `combat`, `cost`, `pay`, `resolve`, `scriptfacts`, `trigmatch`), `view`, `seat` (the `Seat` contract only), `replay`, `eventdiff`, `deck`, and `game` with `gorgegame` (the [seam](../architecture/index.md#the-engine-seam) a host runs an engine through). |
| `bots/` | The bot registry (`bots`, linked by `bots/all`), the one heuristic policy (`botpolicy`), the default bot (`heuristic`), the hosted bots (`bot`, `lethalpressure`, `sbtactical`, `azredeal`, `forgeheuristic`) and what they run on (`azmcts`, `gonet`, `netsimd`, `policynet`, `searchprobe`, `searchseat`, `sanity`, `spellbench/*`, `traceboard`, `explain`). |
| `server/` | The reference server: `gorged` (importable: `Main`, `Serve`, the `Extensions` seam), `host` (tables, sessions, persistence) with `host/httpapi` (JSON + server-sent events), `host/manabrewhttp` and `host/sbv2http`, `protocol` (the wire types the web client is generated from), `manabrew` and `spellbench/*` (the other [wires](../architecture/wires.md)), `divergence`, `matchrecord` (the portable record of a played game that `gorged review` serves). |
| `draft/` | The limited stack: `limited` (+`setdata`, `booster`, `draft`, `build`, `pickbot`), `event`, `eventhost`, `gorgedevents` (the plugin `cmd/gorged` passes to the server) and `draftlab` (+`extern`, the external-agent wire). See [Draft events](../architecture/draft.md). |
| `testkit/` | Public test support: `testutil` (the corpus registry, repo decks, the acceptance decks, `feedback` snapshot loading), `bench` (a policy-agnostic head-to-head game runner), `paymirror` (an A/B check of payment plans), `conformance` (plays one game through a client wire and diffs what it says). |
| `cli/` | The [command-line layer](https://github.com/adams-shaun/gorge/blob/main/docs/cli.md): GNU-style flags, environment variables and JSON config files on the standard library. |
| `internal/` | Module-private ratchets and generators: `archtest` (import-graph and determinism tests), `codeshape`, `corpusheap`, `testbudget`, `tsgen` (Go → TypeScript). |
| `cmd/` | Binaries. Product: `gorged`, `forgec`. Developer: `mtgsim`, `repro`, `headdiff`, `cardfuzz`, `enginebench`, `pkgmap`, `gentypes`, `codeshape`, `testbudget`, `gcgate`, `deckimport`, `keywordbench`, `paymirror`, `kcensus`, `spellbench-gorge-env`, `spellbench-gorge-agent`, `draftsim`, `labdemo`, `extagent`. |
| `examples/` | Bots in Python and Java ([Examples](../bots/examples.md)). |
| `web/` | The TypeScript/Svelte browser client. `web/src/protocol.ts` is generated by `make gentypes`; never edit it by hand. Its own `go.mod` keeps Go tooling out of `node_modules`. |
| `scripts/` | Contributor and CI scripts: `ci_shard.sh`, `smoke.sh`, `wire-matrix.sh`, `examples-smoke.sh`, `enginebench-*.sh`, `docs_sets.py`, `open-pr.sh` (`make pr`). |
| `docs/` | `site/` (this site), `decisions/` (architecture decision records), `design/` (living design documents), `cli.md`, and a guide for AI coding agents. |
| `.githooks/`, `.github/` | The opt-in git hooks (`git config core.hooksPath .githooks`) and the CI, docs and coverage workflows. |
| `AGENTS.md`, `CONTRIBUTING.md`, `README.md`, `Makefile`, `go.mod`, `mkdocs.yml`, `LICENSE`, `NOTICE` | Root files. `go.mod` has no `require` lines. |

Untracked paths you will meet locally:

| Path | What |
|---|---|
| `.cards/` | The Forge corpus (`cardsfolder/`, `tokenscripts/`, `cards.lock`) and the compiled IR cache (`ir-<fingerprint>.gob.gz`). GPL-3.0, gitignored. Without it, corpus tests skip. |
| `bin/` | `make` build outputs (`forgec`, `mtgsim`, `gorged`). |
| `gorged-data/`, `feedback/` | A local server's default persistence and player bug reports. |
| `cmd/repro/testdata/.tokens/` | Token-script text stripped from committed replay fixtures. |

Machine learning, private experiments and the compliance oracles are not here: see
[The three repositories](../repos/index.md).

## Tiers and import direction

The classification file
[`docs/site/architecture/packages.tiers.json`](https://github.com/adams-shaun/gorge/blob/main/docs/site/architecture/packages.tiers.json)
assigns every package a tier and a status. `go run ./cmd/pkgmap` turns it and
`go list` into the tables on [Packages](../architecture/packages.md), and
draws the tier graph from the real imports with any back-edge in red.

| Tier | Directories | May import |
|---|---|---|
| Engine | `engine/` | the standard library and other `engine/` packages only |
| Bots | `bots/` | `engine/`, other `bots/` packages |
| Server | `server/` | `engine/`, `bots/`, other `server/` packages |
| Tooling | `internal/`, `testkit/`, `cli/` | `engine/`, `bots/`, `server/`, other tooling packages |
| Draft | `draft/` | `engine/`, `bots/`, `server/`, `testkit/bench`, other `draft/` packages |
| Commands | `cmd/` | anything; no package imports a command |

Every arrow in the tier graph points down this table except the back-edges the graph
draws in red: test support for the SpellBench adapter imports `testkit/testutil`. Tests
may import anything; these rules are about non-test code.

What is actually *enforced* is a list of edges that must never appear,
checked directly and transitively by `internal/archtest`
`TestDependencyOrderHolds`. The ones a contributor is most likely to hit:

| Never | Why |
|---|---|
| `engine/effects` → `engine/rules` | effects reach the engine only through the small `effects.Host` interface, so the effect library stays testable on its own |
| `engine/view` → `engine/rules`, `server/protocol` → `engine/rules` | a client-facing type can never be handed rules knowledge |
| `engine/cards` → `engine/state`, `engine/deck` → `engine/rules` | the compiler and deck reader sit below the game |
| `bots/botpolicy` → `engine/view`, `engine/rules`, `engine/seat` | the one policy reads a `Board`, never an engine or a view |
| any `bots/` package → `server/host` or `testkit/testutil` | bots plug into a host; they never depend on one |
| `server/host`, `server/host/httpapi`, `server/gorged` → `testkit/testutil` | test scaffolding stays out of the server |
| `server/protocol` → `bots` | the wire is engine-only |
| anything outside `draft/` → `draft/`, except `cmd/gorged`, `cmd/draftsim`, `cmd/labdemo`, `cmd/extagent` | limited play stays above the engine and the server (`TestDraftTreeStaysAbove`) |

Inside `engine/rules`, the sub-packages (`cost`, `chars`, `combat`, `trigmatch`,
`pay`, `resolve`, `scriptfacts`) have their own layering tests
(`TestCostVocabularyIsALeaf`, `TestCombatImportsStayBelowRules`,
`TestTrigmatchImportsStayBelowL5`, `TestCharsImportsStayBelowRules`, …), and
each reads the engine through a narrow `Board` interface whose method count
only shrinks. A new package also needs a tier entry, or `go run ./cmd/pkgmap
-check` (the `pre-commit` hook, and `TestClassificationIsComplete`) fails.

## Where does my change go?

| I want to… | Go to |
|---|---|
| Fix how a card plays | Start with [Your first card fix](first-card-fix.md). |
| Implement an `api:` effect | `engine/effects/`: `effects.Register("Name", fn)` in an `init()`. If it needs more than `effects.Host` offers, it is rules work. |
| Implement a keyword, trigger, static or replacement | `engine/rules/`, declared with `effects.RegisterNonAPI("kw:X", …)`. A trigger mode's matcher goes in `engine/rules/trigmatch/`. A keyword that rewrites into ordinary script lines is an `engine/cards/kw_*.go` expander (`registerKeyword`) instead. |
| Change a characteristic rule (CR 613 layers) | `engine/rules/chars/`. |
| Change attack or block legality | `engine/rules/combat/`; the declaration flow stays in `engine/rules/combat.go`. |
| Add a `count:` value head | The evaluator in `engine/effects/count*.go` **and** `modelledValueHeads` in `engine/effects/value_heads.go` (`TestValueHeadRegistryMatchesEvaluator` holds them together). |
| Change how scripts parse | `engine/cards/`. Any edit there changes `cards.CompilerFingerprint`, so every IR cache recompiles (`make compile-cards`). `engine/cards/oracletext` is exempt. |
| Add an event kind | Append after the last constant in `engine/events/event.go`, add its entry to `kindInfo` in `engine/events/kindinfo.go`, and put the state change in `events.Apply` (`engine/events/apply.go`). See [Invariants](../architecture/invariants.md#eventskind-is-append-only). |
| Add a decision kind | Don't, without a maintainer decision: the set is closed. See [Invariants](../architecture/invariants.md#the-decision-kinds-are-a-closed-set). |
| Change what a seat sees | `engine/view/`: `card.go` (cards, battlefield), `stack.go` (stack); `view.go` only for the assembler. |
| Change the wire | `server/protocol/`, then `make gentypes` (regenerates `web/src/protocol.ts`). |
| Change server routes or seat authorisation | `server/host/httpapi/`; every seat route goes through `claimForTable` (`rest.go`). |
| Change a client wire | `server/manabrew/`, `server/spellbench/`, and the matching cell on [Wires](../architecture/wires.md). |
| Change server flags or startup | `server/gorged/` (`cmd/gorged` is a short main). |
| Change draft events | `draft/`: the pure logic in `limited/*` and `event`, hosting in `eventhost`. See [Draft events](../architecture/draft.md). |
| Change the default bot's choices | `bots/botpolicy/` (the policy), `bots/heuristic/` (its seat adapter). |
| Add a bot | `bots.Register` in an `init()` (`bots/registry.go`), blank-imported from `bots/all`. Read the [bots section](../bots/index.md) first. |
| Add a repo deck | `testkit/testutil/decks/*.json` via `go run ./cmd/deckimport`; every card in it must be fully supported. |
| Turn a feedback report into a test | `go run ./cmd/repro -emit-test <pkg> <feedback-dir>` (see [Contributing](index.md#reproduce-a-feedback-report)). |
| Add a package | Wherever it fits the tiers, plus an entry in `packages.tiers.json` in the same commit. |
| Change the web client | `web/` (needs Node; see `web/README.md`). |
| Change this site | `docs/site/`, built by `mkdocs.yml`; see [Contributing](index.md#documentation). |

Behaviour that looks like a bug but is deliberate is recorded in
[`docs/design/2026-09-22-engine-contracts.md`](https://github.com/adams-shaun/gorge/blob/main/docs/design/2026-09-22-engine-contracts.md);
read it before filing.
