# Repo map — the platform chain: `engine/`, `bots/`, `server/`

Part of the [repo map](repo-map.md) index. See also:
[invariants.md](invariants.md) (the rules every change must keep) and
[do-not.md](do-not.md) (the mistakes that have actually happened here).

The import direction is one-way: `engine/` imports nothing outside itself,
`bots/` imports only `engine/`, and `server/` imports both.
`internal/archtest`'s `TestDependencyOrderHolds` pins the arrows that must
never appear (see [invariants.md](invariants.md#1-dependency-direction)).

## `engine/`

| Package | What it owns |
|---|---|
| `engine/cards` | Compiles Forge card scripts into the card IR; keyword expanders (`engine/cards/kw_*.go`); `cards.Registry`, coverage. `cards.OpenCorpusFor` (`engine/cards/subset.go`) opens only a known card pool's cards from the per-card segment file, falling back to the whole corpus when the pool reads the name universe; bench tools' fixed-deck modes use it. `cards.CompilerFingerprint` hashes every non-test `.go` file here, so **any edit in `engine/cards/` invalidates every IR cache** and forces a recompile. |
| `engine/cards/oracletext` | Printed-card (Oracle) view for the Oracle audit. A subpackage on purpose so it does not move the fingerprint. |
| `engine/state` | Objects, zones, players, `state.Game`, the `ContinuousEffect` vocabulary. Plain data; mutated only by `events.Apply`. |
| `engine/decision` | `Decision` / `Option` / `Intent`, and the closed set of `decision.Kind`s (`decision.Kinds`). The whole engine↔seat contract. |
| `engine/deck` | `{name, count}` deck-list JSON resolved against a `cards.Registry`. |
| `engine/events` | The event union, `events.Apply` (the only state mutator), the log, the sha256 hash chain. `events.Kind` is append-only. |
| `engine/effects` | `api:` primitive implementations, filter/count/value evaluators. Talks to the engine only through the small `effects.Host` interface; never imports `rules`. |
| `engine/effects/params` | Leaf home of the compiled ability-parameter records shared by `effects` and the payment layer. |
| `engine/rules` | The engine: turn structure, priority, stack, combat, SBAs, layers, casting and payment, and the `kw:`/`trig:`/`stat:`/`repl:` primitives. `rules.New`, `Pending`, `Submit`, `Advance`, `Clone`. `rules.NewStaged` (`engine/rules/staging.go`) builds a hypothetical engine at a described position. Also holds the coverage ratchets (`engine/rules/*_test.go`) and the golden heads (`engine/rules/testdata/heads/`, pinned by `TestHeads`). |
| `engine/rules/chars` | The read-only CR 613 layer walk: `Types` (layer 4), `Compute` (layers 3/5/6 with CR 613.6 dependency order, then 7), `PT` (layer 7), `Name`, `BaseKeywords`, the layer-7a CDA evaluators. Reads the game only through `chars.Board`, which `engine/rules/chars_board.go` implements; never imports `rules`. A new characteristic rule goes here; a new Board method fails `TestCharsBoardOnlyShrinks`. |
| `engine/rules/combat` | Attack/block legality: `CanAttack`/`CanAttackPair`/`CanBlock`, the CR 508.1d requirement set (`AttackRequirements`), goad, restriction walks, MustBlock duties. Reads the game only through `combat.Board` (`engine/rules/combat_board.go`). The declaration flow stays in `engine/rules/combat.go` and `engine/rules/attack_cost.go`. A new Board method fails `TestCombatBoardOnlyShrinks`. |
| `engine/rules/crcoverage` | The CR coverage tables: one file per CR chapter, one row per rule with status, the deciding code (path#Symbol) and tests. A change that alters how gorge handles a rule updates that rule's row in the same commit; TestCRCoverageIsCurrent (engine/rules/crcoverage_test.go) fails on a reference that no longer resolves. |
| `engine/rules/cost` | The shared cost vocabulary: `Cost`, `CostPart`, the Forge cost-string parsers. A leaf (`TestCostVocabularyIsALeaf`). |
| `engine/rules/pay` | The L5 payment-planning layer (lasagna spec §9). |
| `engine/rules/resolve` | The L6 resolution kernel (lasagna spec §7); sole owner of the mid-resolution ask state, `Engine.tape` (`TestTapeStateOwnedOnlyByTheResolutionKernel`). |
| `engine/rules/scriptfacts` | Pure facts a card's printed text and compiled script imply, derived without game state. |
| `engine/rules/trigmatch` | The trigger-mode matchers: the `Mode$` registry (`Lookup`, `Match`), one matcher per mode, reading the engine through `trigmatch.Board` (`engine/rules/trigmatch_board.go`). Queueing and latches stay in `rules`. A new trigger mode registers here. |
| `engine/view` | Projects one seat's redacted `view.View` (`Project`, `ProjectFor`, `RedactEvents`). The only way a client reads state. `engine/view/view.go` is the orchestration; the card/battlefield family lives in `engine/view/card.go` and the stack family in `engine/view/stack.go`. A projection change goes in the family file for its seam. |
| `engine/seat` | The seat contract only: `seat.Seat`, `PaymentPlanConsumer`, `TargetHintConsumer`. Imports only `decision` and `view`. Concrete bots live in `bots/`. |
| `engine/replay` | Re-executes `(Config, Log)` and names the first divergent event (`Replay`, `ReplayTo`). |

The layering design for `engine/rules` is
[`docs/design/2026-10-03-rules-engine-lasagna-design.md`](../design/2026-10-03-rules-engine-lasagna-design.md).

## `bots/`

| Package | What it owns |
|---|---|
| `bots/botpolicy` | The one heuristic bot policy (`botpolicy.Decide`), in one copy, shared by `bots/heuristic` and rules' own fuzz driver. Reads a `Board`, never a view or an engine (archtest forbids `view`, `rules`, `seat`). Cast profiles in `bots/botpolicy/profiles/`. |
| `bots/heuristic` | The default bot: `heuristic.Bot` (`NewBot`, `NewLethalPressureBot`, `NewAttackSimBot`, `NewExploreBot`, `NewCastProfileBot`, …), `heuristic.BoardSeat` (a seat that reads a `botpolicy.Board`), and `PolicyNetBot` (`NewPolicyNetBot`). |
| `bots` | The closed registry of hosted policies (`bots/registry.go`, `bots.Factory`); gorge's built-ins register from `bots/bot`, `bots/lethalpressure`, `bots/sbtactical` and `bots/azredeal`, linked together by `bots/all`. Companion modules register further bots the same way; network-backed hosted bots (az-c3/c4/c5) are linked through `server/gorged.RegisterNetBot` and enabled by gorged's `-az-c*-net` flags (`server/gorged/azc3.go`, `server/gorged/azclosed.go`). |
| `bots/sanity` | Opt-in common-sense guard decorator over engine target hints (default off). |
| `bots/bot`, `bots/lethalpressure`, `bots/azredeal`, `bots/sbtactical` | The hosted policy packages (`az-redeal` and `sb-tactical` are BP adapters of the hosted-bot-packages design) over the inference runtime below. |
| `bots/azmcts` | AlphaZero-style PUCT search ([`docs/design/2026-09-27-alphazero-mcts-design.md`](../design/2026-09-27-alphazero-mcts-design.md)). |
| `bots/gonet` (+`gonettest`) | The `.dzgw` network forward pass the az bots evaluate. |
| `bots/policynet` | Feature layer of the learned per-option policy: deterministic encoder from a seat's view, MLP, `.gpol` checkpoints. |
| `bots/searchprobe` | Hidden-information world sampler (`Collector`, `Sample`, `Redealer`, known-card tracking). |
| `bots/searchseat` | The PIMC search decision function (`Choose`) shared by the teacher corpus generator and `SearchBot`. |
| `bots/spellbench` (+`builtins`, `payexec`) | The SpellBench benchmark deck catalogs (moved here from the old `lab/` tree; generator `bots/spellbench/fdn-catalog.py`), the builtin and tactical policies, and the payment-plan lowering. |
| `bots/traceboard` | The redacted, map-free decision-trace board schema (written by bench tools' decision traces). |

## `server/`

| Package | What it owns |
|---|---|
| `server/gorged` | The importable server (`Main`, `Serve`): flags, persistence, demo tables, art prewarm. Extension seams: `Extensions{Events EventsPlugin; Flags}` (draft events under `/api/events`, which `cmd/gorged` fills from `draft/gorgedevents` and the zero value refuses; the binary's own flags on gorged's command line), `RegisterNetBot` (the az-c3/c4/c5 network bots) and `RegisterClosedNetBot` (further closed-list net policies, served by the az-c4/c5 machinery under their own `-<policy>-*` flags). Draft events offer every policy that does not read decklists, closed-list net policies included (`eventPolicies`). `cmd/gorged` is its thin main. |
| `server/host` | Tables (one goroutine each), sessions, snapshots, persistence, undo, the hosted bot-policy vocabulary (`server/host/bot_policy.go`). Its only clock is an injected sleep. |
| `server/host/httpapi` | `net/http` JSON + SSE + the embedded web client. Every seat route goes through `claimForTable`. |
| `server/host/manabrewhttp` | The ManaBrew HTTP surface. |
| `server/protocol` | Versioned wire types for host↔client. Types only; never imports `rules`. `server/protocol/manabrew` holds the ManaBrew wire types and codecs. |
| `server/manabrew` (+`mbtest`) | The ManaBrew translator: projects a seat's `view.View` into ManaBrew state, plus a mock client for tests. |
| `server/divergence` (+`shadow`) | Human-divergence capture records (`divergence-v1`, `triage-v1` JSONL), the materiality predicate, the shadow seed and the shadow answer. Design: [`docs/design/2026-10-10-human-divergence-design.md`](../design/2026-10-10-human-divergence-design.md). |
| `server/matchrecord` | The portable per-game match record (`gorge-match-1`, `<id>.match.json.gz`): the Config's replay-input subset, the log, the head and provenance; `Write`/`Load`/`Verify`, and `Match.Config`, the Config builder `feedback.Load` shares. Written by `testkit/bench`'s `Hooks.Record`; `cmd/repro` replays it. Never carries token text; never commit a record. |
| `server/spellbench` (+`cmd/spellbench-gorge-env`) | The SpellBench Protocol v2 environment adapter: catalog, protocol, mapping, session, validation, minihost and goldens. `cmd/spellbench-gorge-env` serves it over stdio. Engine auto-pay is the default (agents never see mana taps); `-manual-mana` restores interactive mana taps. Ported from SpellBench's gorge adapter; see `NOTICE`. |

## `draft/`

| Package | What it owns |
|---|---|
| `draft/limited` (+`setdata`, `booster`, `draft`, `build`, `pickbot`) | The pure limited pipeline: set data generated from trimmed MTGJSON, pack opening, the pod, deck rules, the drafter and builder with the embedded 17Lands ratings. No clock, files, network or engine state; each package's `imports_test.go` holds that. |
| `draft/event` | The event lifecycle: the append-only log, `Apply`, `ReplayLog`, pairings, standings, 17Lands-shaped CSV export. Pure, and never imports the engine. |
| `draft/eventhost` | The runtime inside gorged: store under `<dir>/events`, seat tokens, timers, restart resume, series on host tables. The only package of the stack allowed the clock. |
| `draft/gorgedevents` | Adapts `eventhost` to `server/gorged.EventsPlugin`; `cmd/gorged` passes its `Plugin{}`. `server/gorged` cannot import it: that would be a cycle. |
| `draft/draftlab` (+`extern`, `extern/wire`, `extern/refagent`) | The Go composition API, and out-of-process agents over the `gorge-agent/1` wire (`docs/site/api/gorge-agent.md`, goldens in `extern/wire/testdata`). |
