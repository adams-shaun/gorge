# mtgcore — a native Go Magic rules engine

Date: 2026-09-03
Status: design, pending approval
Replaces (eventually): `mtgplay/` — the JVM WebSocket bridge over XMage
Home: `mtgcore/` (own `go.mod`) in this monorepo now; standalone repo later

## Problem

`mtgplay` wraps the vendored XMage engine (`mtgplay/vendor/xmage`,
`magefree/mage`, MIT) behind a JSON WebSocket bridge. It works, and a year of
AI training rests on it, but four costs have become structural:

1. **The client has to understand the engine.** The `priority` prompt ships raw
   zone contents and a bag of options, and the browser reconstructs enough game
   state to render and validate. Rules knowledge leaked across the wire; every
   new mechanic touches both sides.
2. **Two players only.** `SpikeGame` is an in-tree port of XMage's
   `TwoPlayerDuel`. Multiplayer needs the `Mage.Server.Plugins` module we
   deliberately do not compile.
3. **No replay.** There is a `MatchReplay` and a `ScriptedReplayPlayer`, but no
   seeded, verifiable "resume this match at turn 7" primitive. Match history is
   log scraping (`GameLogWatcher`).
4. **Build and runtime weight.** ~5-10 min cold Maven build over 3,700 engine
   classes plus 31,818 card classes; ~290 MB image; a JVM per match host; and
   `WebSocketPlayer.copy()` returns `null`, so the engine's own state-branching
   path is untested for human seats.

A native Go engine collapses the stack to one language, one binary and one
deployment, and lets the wire protocol be designed around a rules-ignorant
client from the start.

**Note on framing:** the vendored engine is XMage, not Forge. Both matter here.
XMage is the engine we run today; Forge is the source of the card-behaviour
corpus this design proposes to reuse.

## Goals

- Rules engine in pure Go, no cgo, no JVM.
- Clean client/server split: the server owns all rules; the client renders a
  projection and answers enumerated decisions.
- 4+ players as a first-class concept, not a 2-player engine widened later.
- Every match reproducible from a seed and an intent list; "playback to N" and
  "resume from N" are the same operation.
- Card behaviour reused from public work, not hand-written per card, and
  extensible without recompiling the server.
- An explicit AI seat interface. Building the AI is out of scope for now.

## Non-goals (this spec)

- Replacing the XMage-based AI training pipeline. The policy work stays on
  `mtgplay` until the Go engine can host a search.
- Deck construction, collection, inventory — those stay in `mtgserve`.
- Full 33,669-card parity at launch. The target is the ~1,500-card curated pool; see Milestones.
- Tournament / sanctioned-play rules enforcement, spectator chat, matchmaking.

## Evidence

Everything below was measured, not estimated. POC sources are listed in
Appendix A; they are throwaway and do not ship.

### The Forge corpus is a real, parseable behaviour source

`Card-Forge/forge` ships **33,669 declarative card scripts** in
`forge-gui/res/cardsfolder`. A 400-line Go parser handles the entire corpus:

| Measure | Value |
|---|---|
| Files parsed | 33,669 (34,544 faces) |
| Parse time | **2.09 s** (16,094 cards/sec, single-threaded) |
| Diagnostics | **9** — all unresolved `SVar` references, i.e. upstream data bugs |
| Source size | 24.4 MB of `.txt` |
| Compiled IR | 34.3 MB gob / **6.7 MB gzipped**, decodes in 84 ms |

### The primitive surface is small enough to implement

Distinct engine symbols the corpus references:

| Symbol class | Distinct |
|---|---|
| Effect APIs (`SP$`/`AB$`/`DB$`/`ST$`) | 192 |
| Trigger modes (`T:Mode$`) | 137 |
| Static modes (`S:Mode$`) | 76 |
| Replacement events (`R:Event$`) | 34 |
| Keywords (`K:`) | 252 |
| **Union** | **694** |

Card coverage as a function of primitives implemented, taking them in
descending order of how many cards reference them:

```
 25 prims → 35.0%      200 prims → 86.1%
 50 prims → 54.5%      300 prims → 93.3%
100 prims → 71.8%      500 prims → 99.1%
150 prims → 80.8%      694 prims → 100.0%
```

The honest larger number: **4,007 distinct `(API, parameter)` pairs**, of which
2,547 are used by ≥1% of that API's occurrences, plus two sub-languages — 978
filter predicates (169 used ≥10×) and 417 `Count$` expression heads (86 used
≥10×). Full parity is roughly 4,000 parameter handlers and ~1,400 filter and
expression symbols. That is large, but it is 10-60× less work than 33,669 cards
and it degrades gracefully: an unimplemented primitive makes specific cards
unplayable, not the engine wrong.

Read that curve alongside the pool table below and they look contradictory —
300 primitives covering 93% of the corpus, but Modern-legal (66% of the corpus)
needing 547. They measure different things. The curve answers "if I implement
the N most popular primitives, how many cards fall out complete?"; the pool
table answers "for this specific list of cards, what must exist?" A pool
defined by format legality drags in rare primitives that the popularity
ordering puts near the end. The pool table is the one to plan against.

### The target pool is small, and the cost curve saturates fast

Measured against this project's own data — the prod catalogue in
`backups/20260903T045706Z-mtgbld-prod.sqlite` and the 12 Legacy decks in
`mtgplay/src/main/resources/ai-decks`:

| Card pool | Cards | Primitives | (API, param) pairs | Filter predicates |
|---|---:|---:|---:|---:|
| The 12 repo AI decks | 136 | **81** | 274 | 20 |
| Every card in the user's decks | 990 | 239 | 927 | 74 |
| **Every card in the user's lists** | **1,657** | **264** | **1,015** | **102** |
| Standard-legal | 4,885 | 306 | 1,569 | 168 |
| Modern-legal | 22,441 | 547 | 2,957 | 527 |
| Whole Forge corpus | 33,669 | 694 | 4,007 | 978 |

Two things fall out of this table.

**Forge covers essentially the entire real catalogue.** Name resolution
matched 990/990 deck cards, 1,657/1,658 list cards, 4,885/4,887 Standard-legal
and 22,441/22,450 Modern-legal — 99.96% overall. Whatever else is uncertain,
"will the corpus have the cards we care about" is not.

**The curve saturates.** Going from 136 cards to 1,657 costs 183 more
primitives. Going from 1,657 to 4,885 — tripling the pool — costs 42 more.
Going all the way to 22,441 costs 283 more. Early primitives are expensive per
card and late ones are cheap, which is the opposite of the usual scaling worry
and is what makes the staged plan credible.

The chosen ~1,500-card milestone is therefore concrete: it is the user's own
1,657-card list pool, and it costs **264 primitives, 1,015 parameter handlers
and 102 filter predicates**.

### The engine core works, at 2, 4, 6 and 8 seats

A 1,560-line Go engine spike plus a 370-line script parser drives real Forge-parsed cards (Mountain, Forest,
Lightning Bolt, Shock, Raging Goblin, Grizzly Bears) through a full turn
structure, priority, stack, combat, state-based actions and elimination:

```
2 players:   409 intents,  1,871 events,  37 turns
4 players:   993 intents,  3,961 events,  55 turns
6 players: 2,397 intents,  9,061 events, 101 turns
8 players: 3,196 intents, 11,686 events, 117 turns
```

60 random seeds all terminate with invariants intact (208,577 events). No card
behaviour is written in Go — it comes from the parsed scripts.

The invariant checker found a real double-push bug on the stack the first time
it ran. That is the argument for shipping it in CI.

### Determinism and replay hold

| Property | Result |
|---|---|
| Same seed + same intents → same match | chain `8494904a5ad602b3` twice; seed+1 gives `ca32262b73e6e52e` |
| Replay from `(seed, intents)` reproduces the log | identical 3,961-event chain, identical 236 RNG draws |
| Playback to intent N matches the chain prefix | 1,890 events at N=496, hash matches `HashAt` |
| Resume from N to end reproduces the finish | identical terminal chain |

### The scripting decision is settled by one number

Per effect resolution, three host interactions each:

| Approach | ns/op | allocs |
|---|---|---|
| Native Go | **0.72** | 0 |
| **wasm, batched callback-free ABI (wazero)** | **28.4** | **0** |
| wasm, no host interaction at all (floor) | 9.0 | 0 |
| gopher-lua, host callbacks | 190 | 1 |
| starlark-go, host callbacks | 291 | 7 |
| wasm, 3 host callbacks | 859 | 14 |

The naive design — "the script calls back into the host" — is the trap: a
wazero host callback costs ~283 ns and allocates. Crossing the wasm boundary
itself costs 9 ns. Staging inputs in linear memory and reading a mutation list
back makes wasm **7× faster than Lua, 10× faster than Starlark, and
allocation-free**, at 40× native for the small slice of work that needs it.

Instantiation: 6.7 µs / 7 KB for a hand-encoded module; **979 µs / 5.5 MB** for
a Go-compiled `wasip1` guest. Guests must be compiled with TinyGo or Rust, or
instantiated once and reused. This design does the latter.

At game scale the tier disappears into the noise — the wasm run measuring
faster than native is variance, not a speedup:

| Configuration | ms / 4-player game | allocs |
|---|---|---|
| Native effects | 3.59 | 47,676 |
| `DealDamage` served by a wasm plugin | 3.19 | 47,691 |
| Native, hash chain disabled | 2.16 | 37,200 |

Running the same match with `DealDamage` implemented in Go and then in wasm
produced **byte-identical event chains** (`9acfcc01537938f9`, 3,961 events).

Two further numbers that shape the design: the SHA-256-over-JSON hash chain
costs **35%** of engine time (use a compact binary encoding), and a naive deep
copy of the map-based state costs **10.3 µs / 281 allocs** — which is why the
production state model must be dense slices, not maps, before any AI search
lands on it.

## Decisions

### D1. Card behaviour comes from Forge scripts, fetched not vendored

| Option | Pros | Cons |
|---|---|---|
| **Forge `cardsfolder`, fetched at build time (chosen)** | 33,669 cards; declarative; a DSL designed for exactly this; new sets arrive from upstream contributors; 694-primitive surface | GPL-3.0, so it cannot be redistributed inside a permissive artifact; adds a fetch step |
| Reflect XMage's ability graph into an IR | MIT throughout; matches the engine we run today | ~800 effect classes and 248 keyword classes to reimplement; needs a JVM build step; brittle across upgrades; the graph is code, not data |
| Compile behaviour from Scryfall/MTGJSON oracle text | No license question; data is already in `mtgserve` | Natural-language rules parsing is an open research problem; unreliable in exactly the corner cases that matter |

**Chosen posture (user decision):** `mtgcore` is Apache-2.0. It ships a
*compiler*, never the scripts. At build or first run the operator fetches a
pinned Forge tag's `cardsfolder`, `mtgcore` compiles it to the IR cache, and
that cache stays out of published images and out of git. The engine keeps a
`cards.lock` recording the upstream ref and a digest so a match's card
definitions are as reproducible as its seed.

Compliance rules, to be enforced in CI:

- No `.txt` from Forge in this repo, in any image layer we publish, or in any
  release artifact.
- The fetcher records the upstream ref, license and digest, and prints the
  GPL-3.0 notice on first fetch.
- The IR cache is a build product on the operator's machine, treated like a
  compiler cache.
- The compiler itself contains no Forge code — only a parser for a file format,
  written from the published scripting API.

### D2. Scripting runs on wasm via wazero, with a callback-free ABI

| Option | Pros | Cons |
|---|---|---|
| **wasm / wazero, batched ABI (chosen)** | 28 ns, zero-alloc; pure Go host, no cgo; any source language; deterministic by construction; sandboxed with no ambient authority | Contributors need a wasm toolchain; guests must avoid fat runtimes |
| Starlark (`starlark-go`) | Deterministic by design; trivial to write; no toolchain | 291 ns and 7 allocs per effect; one language only; no plugin-artifact story |
| Lua (`gopher-lua`) | Familiar to game modders; 190 ns | `pairs()` iteration order is nondeterministic — a replay hazard; one language only |

Both scripting languages lose on the axis that matters most here — they are
callback-shaped, and callbacks are what cost. The wasm ABI is:

```
host: stage a fully resolved request in the guest's linear memory
guest: pure function, no imports, no clock, no randomness, no I/O
guest: write a list of PROPOSED mutations
host: validate every mutation, then emit them as ordinary events
```

Guests cannot read hidden zones, cannot mutate state, and cannot introduce
nondeterminism, because they have no host imports at all. Mid-resolution player
choices (targets, modes, "you may") are handled by the guest returning a
`need_choice` continuation; the host asks the player, records the answer as an
intent, and re-invokes the guest. Choices stay in the replay log, so scripted
cards replay exactly like native ones.

Contributor ergonomics: `mtgcore` publishes a thin PDK (Go, Rust, and
AssemblyScript to start) that hides the byte layout. **Extism** is the fallback
if maintaining our own PDK proves worse than adopting theirs — its Go SDK is
wazero-based, so the swap is contained. This is deliberately left as a
follow-up decision once there are real third-party plugins to learn from.

### D3. The plugin unit is the effect primitive, not the card

This is the load-bearing choice.

| Option | Pros | Cons |
|---|---|---|
| **One plugin per effect API (chosen)** | ≤694 plugins ever; cards stay pure data; a new set usually needs *zero* code because its cards reuse existing primitives; hot-swappable | A genuinely novel mechanic needs a new primitive |
| One plugin per card | Maximum expressiveness | 33,669 plugins; instantiation and cache pressure; every set is a code drop |
| No plugins, native only | Fastest, simplest | Every primitive is a server rebuild and redeploy — the thing the user explicitly wants to avoid |

Consequences:

- Adding a set is a data refresh: re-fetch `cardsfolder`, recompile the IR,
  and the new cards work if their primitives are already implemented. Given the
  coverage curve, most of a new set costs nothing.
- Adding a *mechanic* means writing or amending one primitive, shipped as a
  wasm plugin, loaded without restarting the server.
- Primitives graduate: a plugin that proves correct and hot gets a native Go
  implementation behind the same identifier. The POC showed both tiers produce
  identical logs, so graduation is a swap, not a rewrite.

### D4. The log is intents *and* events, tied together by a hash chain

| Option | Pros | Cons |
|---|---|---|
| **Both, chained (chosen)** | Intents replay compactly; events survive engine changes; the chain turns "did the engine drift?" into one comparison | Two representations to keep in step |
| Command sourcing only (seed + intents) | Smallest log | A rules fix silently rewrites history; old matches become unreplayable with no signal |
| Event sourcing only | Robust to engine change | Larger; "resume from" needs derived state rebuilt anyway |

The chain is over a compact binary encoding, not JSON — the POC measured the
JSON version at 35% of engine time. Full state checkpoints every N events keep
"jump to turn 12" from replaying the whole match. On replay, a chain mismatch is
a loud, specific error naming the first divergent event.

### D5. Transport is a WebSocket carrying a versioned envelope

| Option | Pros | Cons |
|---|---|---|
| **WebSocket + versioned JSON envelope (chosen)** | Reuses the existing Envoy route and 1 h `BackendTrafficPolicy`; browser-native; trivially debuggable | Hand-rolled schema discipline |
| Connect-RPC / gRPC-web | Generated clients; typed streams | New gateway config; heavier browser client; protobuf toolchain in the build |
| SSE downstream + POST upstream | Survives proxies well; simple | Two channels to correlate; awkward for rapid priority round-trips |

The envelope is generated from Go types so the client and server cannot drift.
Migrating to protobuf later is a wire-format change behind the same envelope.

### D6. One goroutine owns each match

A match is a single-writer goroutine with an inbound intent channel and
per-seat outbound view channels. This falls out of event sourcing: there is one
serialisation point, so there is one ordering, so there is one log. No locks
around game state, no `copy()` semantics to reason about, and the AI seat is
just another channel consumer.

### D7. `mtgcore/` now, standalone repo later

A subdirectory with its own `go.mod`, depending on nothing in `mtgserve`. The
dependency arrow points one way: `mtgserve` may import `mtgcore`; `mtgcore`
never imports `mtgserve`. Extraction is then a `git filter-repo` away.

### D8. Seats are an interface; humans and bots are both implementations

```go
type Seat interface {
    Decide(ctx context.Context, v View, d Decision) (Intent, error)
}
```

The POC's deterministic bot and a WebSocket-backed human seat implement the same
interface. This is the hook the AI work plugs into later, and it is what makes
headless self-play possible on day one.

## Architecture

```
                         ┌──────────────────────────────────────┐
  browser ──WS──►        │ mtgserve (Go)                        │
                         │   match host: one goroutine/match    │
                         │   ┌────────────────────────────────┐ │
                         │   │ mtgcore (own go.mod)           │ │
                         │   │                                │ │
                         │   │  rules   ← turn/priority/stack │ │
                         │   │  state   ← dense object arena  │ │
                         │   │  events  ← log + hash chain    │ │
                         │   │  view    ← per-seat projection │ │
                         │   │  effects ← native primitives   │ │
                         │   │  plugin  ← wazero tier         │ │
                         │   │  cards   ← Forge IR loader     │ │
                         │   └────────────────────────────────┘ │
                         └──────────────────────────────────────┘
                                        ▲
                          cards.lock    │  compiled IR cache (build product)
                                        │
                            forgec ── fetch pinned Forge tag → parse → IR
```

### Packages

| Package | Responsibility | Depends on |
|---|---|---|
| `mtgcore/cards` | Forge script parser, IR types, intrinsic-ability layer, IR cache load/store | — |
| `mtgcore/state` | Objects, zones, players, dense arena, cheap clone | `cards` |
| `mtgcore/events` | Event union, apply, log, hash chain, checkpoints | `state` |
| `mtgcore/rules` | Turn structure, priority, stack, combat, SBAs, legality | `state`, `events` |
| `mtgcore/effects` | Native primitive implementations, filter and `Count$` evaluators | `rules` |
| `mtgcore/plugin` | wazero host, ABI codec, mutation validation, plugin registry | `rules` |
| `mtgcore/view` | Per-seat projection, event redaction | `state` |
| `mtgcore/seat` | `Seat` interface, scripted seat, bot harness | `view` |
| `mtgcore/cmd/forgec` | Fetch and compile the corpus | `cards` |
| `mtgcore/cmd/mtgsim` | Headless self-play, replay verification, corpus generation | all |

### Data flow for one decision

1. `rules` reaches a point needing a choice and builds a `Decision`: a prompt,
   and a list of legal `Option`s each with a label and an index.
2. `view` projects state for that seat and attaches the decision. Hidden zones
   contribute counts only; another seat's decision is never attached.
3. The client renders the view and returns `{"seq": N, "choices": [i]}`.
4. `rules` validates the intent against the decision it issued — out-of-range,
   duplicate, wrong-player and wrong-sequence are all rejected — appends the
   intent to the log, and applies it.
5. Effects emit events. Every event goes through one `emit` that appends to the
   log then folds into state, so replay and play cannot diverge by construction.

The client never computes legality, never learns a hidden zone's contents, and
needs no rules knowledge. That is the direct answer to the current design's main
complaint.

## Card pipeline

```
forge tag ──► fetch cardsfolder ──► parse ──► IR ──► intrinsics ──► validate
                                                                      │
                            ┌─────────────────────────────────────────┘
                            ▼
             supported? ── yes ──► IR cache (gzip gob, 6.7 MB)
                  │
                  no ──► unsupported set: card is refused at deck-build time,
                         with the specific missing primitive named
```

`validate` is the honesty mechanism: every card is checked against the
implemented primitive set at compile time, so an unimplemented mechanic is a
deck-builder error message, never a mid-match failure. The check is also the
project's progress metric — "we support N of 33,669 cards" is a number the build
prints.

**Intrinsics matter more than they look.** Basic lands carry no mana ability in
the corpus; Forge's engine grants abilities from land subtypes. Any port must
supply that layer or every basic land is a blank card. Keyword abilities are the
same story: `K:Flying` is a token the engine must give meaning.

## Multiplayer

- Seats are a ring. Priority passes in APNAP order, skipping eliminated seats.
- Turn order advances to the next surviving seat.
- A player leaving the game is an event; their permanents leave with them.
- Combat targets a chosen defending player or planeswalker, not "the opponent".
- Range of influence, monarch, day/night and similar shared-state mechanics are
  out of scope for M1 but are modelled as game-level state from the start so
  they do not require a refactor.

The spike ran 2, 4, 6 and 8 seats to completion with no seat-count-specific
code, which is the property worth preserving.

## Testing

| Layer | Approach |
|---|---|
| Parser | Whole-corpus parse must stay at ≤10 diagnostics; regressions fail CI |
| Primitives | Table tests per `(API, parameter)` pair, sourced from real cards |
| Rules core | Invariant checker over seeded fuzz — card conservation, zone agreement, no negative damage, no decisions for dead players, termination |
| Replay | Golden logs: a corpus of recorded matches must replay to the same chain on every commit. A deliberate rules change updates goldens in the same commit, which makes rules drift visible in review |
| Views | Property test: no projection for seat A may contain a hidden zone's contents belonging to seat B |
| Plugins | Every plugin-backed primitive must produce the same chain as its native counterpart where both exist |
| Cross-engine | Opportunistic differential testing against `mtgplay` on scripted 2-player matches, to catch rules misreadings a self-consistent engine cannot catch alone |

## Migration

`mtgcore` runs beside `mtgplay`, not instead of it, until parity is real.

1. `mtgserve` gains a per-match engine selector, defaulting to `mtgplay`.
2. `mtgcore` matches are opt-in behind a flag, initially for the curated pool.
3. When the Legacy decks play cleanly on `mtgcore`, the default flips; `mtgplay`
   stays reachable for AI training.
4. `mtgplay` is retired only when a Go AI seat matches the current policy's
   strength — a separate project, not this one.

## Milestones

Each milestone's scope is a measured number from the pool table, not a guess.

| # | Deliverable | Done when |
|---|---|---|
| M0 | `mtgcore` module skeleton, `forgec` fetch+compile, IR cache, `cards.lock` | Whole corpus compiles in CI; a check proves no Forge `.txt` is committed or shipped |
| M1 | Rules core: 4+ seats, priority, stack, combat, SBAs, continuous-effect layers, event log, replay, per-seat views. **81 primitives.** | The 12 repo decks' 136 cards play headless at 2/4/6/8 seats; invariants, golden replays and view-leak tests green |
| M2 | WebSocket protocol and a browser client holding no rules knowledge | A human beats a scripted bot in a 4-player game through the browser; the client contains no legality logic |
| M3 | wasm plugin tier: registry, ABI, PDK, choice continuations | A primitive authored outside the repo loads without a server restart and replays byte-identically to a native equivalent |
| M4 | **The curated pool: 264 primitives, 1,015 parameter handlers, 102 predicates** | Deck-build validation accepts all 1,657 cards in the user's lists; every one of the 12 decks is playable end to end |
| M5 | Dense-arena state, binary hash chain, `mtgsim` headless self-play | 4-player game under 1 ms; state clone under 2 µs; AI seat interface exercised at self-play scale |
| M6 (stretch) | Standard-legal parity | 306 primitives; validation accepts 4,885 cards |

M1 and M4 are the two that matter. M1 proves the architecture on a pool small
enough to finish; M4 is the scope the user chose, and the table says it costs
183 primitives more than M1, not an order of magnitude more.

## Risks

| Risk | Mitigation |
|---|---|
| The `(API, parameter)` tail is 4,007 pairs corpus-wide, not 192 APIs | The curated pool needs 1,015 of them, measured not guessed; coverage is published per build and validation refuses unsupported cards rather than playing them wrong |
| Continuous effects and the layer system are the hardest part of Magic and the spike models pump as counters | Build the layer system in M1, before coverage work — retrofitting layers is the classic way these projects die |
| Forge's filter and `Count$` sub-languages are undocumented in places | They are data: mine the corpus for every usage, implement against real occurrences, fail loudly on unknown tokens |
| GPL boundary is a judgement call | Never redistribute scripts; keep the compiler clean-room against the published scripting API; the posture is reversible by switching to the XMage-derived IR |
| Upstream Forge renames or restructures the corpus | `cards.lock` pins a ref; the fetcher is the only thing that has to change |
| Rules bugs are silent | Invariants in CI, golden replays, and differential testing against `mtgplay` |

## Open questions

1. **Layer system shape.** Timestamps plus dependency, or a simpler
   apply-in-order model good enough for the curated pool? Needs its own spike
   before M1 coding starts.
2. **PDK vs Extism.** Defer until there are real third-party plugins.
3. **Checkpoint interval.** Needs a measurement once match length distribution
   is known.
4. **Whether `mtgserve` hosts matches in-process or `mtgcore` gets its own
   service.** In-process is simpler and matches the "one binary" goal;
   a separate service isolates crashes. Decide at M2.

## Appendix A — POC sources

Throwaway, preserved at `~/mtgcore-poc/` with a README that re-derives every
number in this spec (it fetches the Forge corpus itself; nothing GPL is stored
there). Not for merge:

| Path | Proves |
|---|---|
| `poc/forgescript/parse.go` | Whole-corpus parse, IR, `SVar` linking |
| `poc/forgescript/face.go` | Printed characteristics and the intrinsics layer |
| `poc/cmd/corpus` | Primitive histogram, coverage curve, greedy set-cover |
| `poc/cmd/deckprims` | Milestone sizing from real pools: repo decks, the prod catalogue's deck and list cards, Standard, Modern |
| `poc/cmd/irsize` | IR artifact size and load time |
| `poc/script/*` | Runtime benchmark: native / wasm / Starlark / Lua, and the hand-encoded wasm modules isolating boundary cost from callback cost |
| `poc/engine/*` | 4+ seat rules core, event log, hash chain, replay, views, invariants, wasm effect tier |

## Appendix B — sources

- [Card-Forge/forge](https://github.com/Card-Forge/forge) — GPL-3.0
- [Forge card scripting API](https://github.com/Card-Forge/forge/wiki/Card-scripting-API)
- [magefree/mage (XMage)](https://github.com/magefree/mage) — MIT
- [tetratelabs/wazero](https://github.com/tetratelabs/wazero)
- [extism/go-sdk](https://github.com/extism/go-sdk)
