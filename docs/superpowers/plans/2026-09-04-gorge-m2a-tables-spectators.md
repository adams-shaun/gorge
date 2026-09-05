# gorge M2a — Tables & Spectators Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `gorged` runs N perpetual tables of 4 bots on the repo decks, paced for humans; a browser spectator watches every table, focuses one (omniscient board, hands, stack, pending tray, transcript, card images), rewinds it event by event while the match keeps running, is backfilled on reconnect, and can replay finished matches after a restart.

**Architecture:** Three new Go packages sit above the engine — `protocol` (versioned JSON envelope, types only), `host` (table registry, match loop, sessions, snapshots, view-at-seq, persistence, crash handling) and `host/httpapi` (SSE downstream + POST/GET upstream, `net/http` only) — plus a thin `cmd/gorged` binary that embeds the Svelte build. `view` gains a visibility mode (public / omniscient), printing identity, target labels and deterministic one-line event descriptions. The engine gets exactly one additive capability, `rules.Engine.Clone`, so the host can keep turn-start snapshots and answer "view at seq N" with at most one turn of replay. The client (`web/`, Svelte 5 + Vite + TypeScript) is a renderer: it holds no rules knowledge, keeps a DVR cursor over the event stream, and fetches server-computed views.

**Tech Stack:** Go 1.26 stdlib only (`net/http`, `encoding/json`, `embed`, `reflect`); Svelte 5, Vite, TypeScript, Vitest, Playwright (Node 20.19+; Node touches `web/` only).

**Spec:** `docs/superpowers/specs/2026-09-04-gorge-m2a-tables-spectators-design.md` (M2a), sequenced by `docs/superpowers/specs/2026-09-04-gorge-post-m1-roadmap.md`; the engine spec `docs/superpowers/specs/2026-09-03-mtgcore-go-engine-design.md` still binds everything not amended there. Design evidence (needs, not contracts): `docs/superpowers/reports/2026-09-03-m0-m1/ui-inspiration.md` recommendations 23–33.

## Global Constraints

Every task's requirements implicitly include this section. Values are copied from the specs and the M0/M1 ledger.

- **Go module stays stdlib-only.** `go.mod` gains no `require`. `CGO_ENABLED=0`. Node touches `web/` only; the Go build never needs Node.
- **Every state mutation goes through `events.Apply`.** `events.Kind` is append-only; never add a field to `events.Event` (the hash chain covers its binary encoding). Nothing in this plan adds an event kind.
- **Dependency order** `cards → state → decision → events → effects → rules → view → seat → replay → protocol → host → host/httpapi → cmd/*`. `effects` never imports `rules`. `protocol` imports `view`, `events`, `decision`, `state` — never `rules`. `host` is the first package allowed to import `time`; `host/httpapi` and `cmd/gorged` may too; nobody else (Task 6 enforces this with a test).
- **No nondeterminism reaches an event, an option order, a view, a frame body, a `.events`/`.intents`/sidecar file or `mtgsim` output.** No `time.Now` anywhere (the only `time` calls are the host's injected `Sleep`, the SSE writer's ticker and keep-alive, and the resume-grace timer). No global `math/rand`. No `map` range order reaching any of the above — sort keys first.
- **One goroutine drives each match.** Readers (`ViewAt`, `Events`, fan-out) synchronise with a per-match mutex; they never drive the engine. Any reachable panic, infinite loop or stall is a bug: a match that does not produce a decision within 400 000 intents is a crash, not "slow".
- **Licensing.** The Forge corpus (GPL-3.0) lives only in gitignored `.cards/`. Never commit, stage or embed a Forge `.txt`. `cards/boundary_test.go` stays. Before every push: `git ls-files | grep -c '\.txt$'` prints `0`.
- **Gates that must not move without a ledgered reason:** acceptance chain heads 2 seats `7705a6505954f6cd`, 4 `2d5589b31c4853cd`, 6 `bf4012092fdad38b`, 8 `01b9f48c1b6dc135`; `make sim` 20/20 replay OK; `make report` → `cards: 33667  playable: 15265 (45.3%)`; `knownUnsupported` 35/136. The engine-level tests `TestRepoDecksPlayAtEverySeatCount`, `TestRepoDeckGamesReplayExactly` and the five Task 22 pins stay green unmodified.
- **Gates every task runs before its commit:** `make lint`, `go build ./...`, `go test -count=1 ./...`. Tasks touching `rules/`, `view/` or `replay/` also run `go test -race -count=1 ./rules/ ./view/ ./replay/`. Tasks 1–5 also run `make sim` and confirm the four chain heads are unchanged.
- **The client holds no rules knowledge.** It renders views, options and server-supplied lines; it never computes legality, targets, costs or timing. Task 24 greps `web/src` for the forbidden vocabulary.
- **Git.** Never bare `git stash`/`git stash pop`. Every commit ends with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`. Work lands on `main` (the user consented at M0; push to `origin/main` is authorized). M2r runs in a parallel worktree in `cards/`, `effects/`, `rules/` — this plan touches `rules/` only in Task 1 (new file `rules/clone.go`, plus four lines in `rules/rng.go`) and `cards/` only in Task 2 (new file `cards/open.go`), so the two never edit the same file.
- **Comment discipline.** Doc comments state what the code does and why the shape is what it is; they do not narrate ledger history. Ruling IDs are cited only where the code would be surprising without them.

## Plan-level rulings (fixed here; the spec left names to the plan)

Each is a `Ruling` in the SDD ledger. Executors treat them as spec.

- **PL-1 `Engine.Clone` is the snapshot.** D11's "turn-start snapshots + ≤ 1 turn replay" needs an engine, not a `state.Game`: `view.Project` reads derived P/T, keywords and pending triggers through `view.Chars`, which only a `*rules.Engine` satisfies (continuous effects and the pending-trigger queue live in the engine, not in `state.Game`), and `replay.ReplayTo` is intent-granular. So Task 1 adds `rules.Engine.Clone()` (+ `events.Log.Clone()`, + `rng.clone()`). View-at-seq is then: nearest turn-start clone → re-clone → `Submit` the recorded intents up to the last intent boundary at or before `seq` → `events.Apply` the remaining events of that burst onto the clone's own `G` → `view.ProjectFor(clone.G, clone, …)`. Inside a burst, zones/life/damage/counters/stack are exact (they are `Apply`'s job); derived P/T from continuous effects and the pending tray are as of the burst's start, which is at most one resolution stale — the next boundary is the very next priority decision. *Cost if wrong:* one additive method in `rules/` (new file), removable without trace.
- **PL-2 Intents are persisted beside events.** `replay` needs `(Config, Log)` with `Log.Intents`; a `.events` file alone cannot rebuild a finished match. Persistence is `<k>.events` (one `events.Event` JSON per line) **and** `<k>.intents` (one `decision.Intent` JSON per line), both append-only, plus the `<k>.json` sidecar. *Cost if wrong:* one extra file per match.
- **PL-3 The wire event is `protocol.Event`, not `events.Event`.** Kind, zones and step travel as their names (`"move_zone"`, `"battlefield"`, `"main1"`), not as `uint8`s a client would have to decode. `protocol.EventFrom(events.Event) Event` is the only converter. `events.Event`'s own JSON stays untouched (it is what the `.events` file holds). *Cost if wrong:* one 13-field mirror struct.
- **PL-4 The HTTP layer is a library too.** `host/httpapi.NewHandler(*host.Registry, Options) http.Handler` carries SSE, the JSON GETs and the POSTs; `cmd/gorged` is flags + corpus + tables + embedded web + `ListenAndServe`. mtgserve mounts the handler at M2c instead of re-implementing SSE. The `Authorize` hook lives on `httpapi.Options` (it takes an `*http.Request`); `host` never imports `net/http`. *Cost if wrong:* a package boundary, movable later.
- **PL-5 Widget frames bypass the ring and carry no `id`.** They are coalesced per (session, table) to the latest value and flushed by the SSE writer every `max(pace, 250 ms)`. A resumed stream gets a fresh widget within one tick anyway, so replaying stale widgets from the ring would only waste the ring. Every other frame type is ring-buffered and resumable. *Cost if wrong:* a client sees a widget up to one tick late after resume.
- **PL-6 The SSE `id:` is `<session>:<frame>`** (for example `s3:4182`). `Last-Event-ID` therefore names both the session ring and the position in it; the header alone tells the server which session to resume. A session whose stream disconnected survives for `ResumeGrace` (default 30 s) and is then closed. *Cost if wrong:* one string split.
- **PL-7 The web build is embedded from `cmd/gorged/webdist/`** (gitignored except `.keep`; embedded with `//go:embed all:webdist`). A clean clone builds and tests with no Node; `gorged` serves `503 web build missing — run make web` for `/` until `make web` has run. *Cost if wrong:* a directory name.
- **PL-8 Deck files and corpus opening move into leaf packages.** New `deck` package (`Parse`, `Resolve`, `Load`) and `cards.OpenCorpus(dir)`; `internal/testutil` delegates to both. `host` and `cmd/gorged` never import `internal/testutil`. *Cost if wrong:* two small moves inside the module.
- **PL-9 A spectator's viewer id is `view.NoSeat = 255`.** `state.PlayerID` is a `uint8`; no table has 255 seats. `view.ProjectFor(..., view.NoSeat, view.Public, nil)` is today's out-of-range-spectator projection under a name. *Cost if wrong:* a constant.
- **PL-10 Omniscient hides exactly library order.** In `view.Omniscient`, every non-Secret event passes unredacted and every hand and pool is projected; a **Secret** `Shuffle` or `Note` (the two Secret kinds whose payload is library order — genesis shuffle and `effRearrangeTopOfLibrary`'s peek) is reduced to its shape; a Secret `Draw`/`MoveZone` out of the library passes through (the card is now in a visible hand). *Cost if wrong:* a two-kind allowlist.
- **PL-11 No wall clock anywhere but the four `time` calls above.** Sidecars carry no timestamps; the finished-match list is ordered by match index. Two runs of one table configuration produce byte-identical `.events`, `.intents` and sidecar files (Task 12's determinism test). *Cost if wrong:* two fields added later.
- **PL-12 `StackView.Params` is deferred.** The engine records neither `X` nor chosen modes on a stack object yet (M2r territory); a field nothing populates is not added. `Targets[].Label` (from `TgtPrompt$`, falling back to `ValidTgts$`) and `Kind: "trigger"` are added as the spec asks. *Cost if wrong:* one additive field when the engine records the values.
- **PL-13 fsync is an option.** `host.Options.Sync` (default true in `gorged`, false in tests) fsyncs after every decision's append. A crash leaves a consistent prefix; a trailing partial line is ignored on read. *Cost if wrong:* durability of the last decision in a power loss.
- **PL-14 Seats' names and bot seeds.** Seat *i* of match *k* is named after its deck file stem; its bot is `seat.NewBot(matchSeed ^ uint64(i+1))`; `matchSeed = host.MatchSeed(table.Seed, k)` (splitmix64 finaliser over `tableSeed ^ (k · 0x9E3779B97F4A7C15)`). Decks rotate one seat per match: seat *i* plays `Decks[(i+k) % len(Decks)]`. *Cost if wrong:* a formula; changes nothing chain-covered.
- **PL-16 The live board refreshes by fetch, once per burst.** Event frames carry no view and the client applies no events (it has no rules), so a focused live client GETs `view?seq=<head>` after each `decision` or `match_end` frame — one request per decision, coalesced to one in flight. The same fetch path serves the DVR cursor (`view?seq=N`, cached per seq), so live and paused rendering are one code path. Rejected: a snapshot frame per burst (a whole view per decision on the stream, for every focused client). *Cost if wrong:* one GET per decision per focused client; at 1.5 s pace, negligible.
- **PL-17 `CardView` carries combat relationships.** `attacking_player` (the seat a creature attacks) and `blocked_by` (the creatures blocking it) are added in Task 4 so the arrow overlay draws attack and block arrows from the view alone. *Cost if wrong:* two additive fields.
- **PL-15 Historical event redaction uses the state at head.** `Events(t, k, since)` in `Public` visibility redacts against the current game (an object still in a hand stays hidden; one since revealed is named). Live `event` frames redact each burst against the state that burst produced (the existing `RedactEvents` convention). *Cost if wrong:* a historical line names a card that has since become public — no hidden information leaks.

## File structure

New and modified files, with one responsibility each. Tasks reference these paths exactly.

| Path | Responsibility | Task |
|---|---|---|
| `events/log.go` (+`Clone`) | independent copy of a log with its chain state | 1 |
| `rules/rng.go` (keep the `*rand.PCG`), `rules/clone.go` | deep copy of an engine at an intent boundary | 1 |
| `cards/open.go` | `OpenCorpus(dir)`: load `ir.gob.gz` or recompile when stale | 2 |
| `deck/deck.go` | deck JSON parse + resolve against a registry | 2 |
| `internal/testutil/decks.go` | thin wrappers over `deck` and `cards.OpenCorpus` | 2 |
| `view/visibility.go` | `Visibility`, `NoSeat`, `ProjectFor`, `RedactEventsFor` | 3 |
| `view/view.go` | `Printing`, `Token`, `ManaCost`, `Kind: trigger`, `TargetView.Label`, `View.Visibility` | 3, 4 |
| `view/describe.go` | `Describe(g, ev)`: one deterministic line per event | 5 |
| `internal/archtest/arch_test.go` | import-graph constraints (`time`, `rules`, `testutil`, client vocabulary) | 6, 24 |
| `protocol/protocol.go`, `protocol/event.go`, `protocol/testdata/*.json` | envelope, frame bodies, wire event, goldens | 7 |
| `internal/tsgen/tsgen.go`, `cmd/gentypes/main.go`, `web/src/protocol.ts` | Go structs → TypeScript, committed output, freshness test | 8 |
| `host/seed.go`, `host/table.go`, `host/match.go`, `host/registry.go` | seeds, table config, match loop, registry | 9 |
| `host/session.go`, `host/fanout.go` | sessions, subscriptions, ring, overflow, frame building | 10 |
| `host/snapshot.go`, `host/viewat.go` | turn-start clones, view-at-seq, events-since | 11 |
| `host/persist.go`, `host/restart.go` | files, sidecars, `tables.json`, restart semantics, finished-match replay | 12 |
| `host/crash.go` | panic recovery, crash report, halted tables | 13 |
| `host/httpapi/rest.go`, `host/httpapi/errors.go` | JSON GETs, POST subscribe/unsubscribe, error mapping | 14 |
| `host/httpapi/sse.go`, `host/httpapi/handler.go` | `/api/stream`, ring resume, widget ticker, keep-alive, static SPA | 15 |
| `cmd/gorged/main.go`, `cmd/gorged/embed.go`, `cmd/gorged/webdist/.keep` | binary | 16 |
| `web/` (Vite + Svelte 5 + TS) | client | 17–23 |
| `web/e2e/` | Playwright against a real `gorged` | 24 |
| `Makefile`, `README.md`, `AGENTS.md`, `.gitignore` | targets `gentypes web web-dev gorged test-e2e-web soak`; docs | 8, 16, 17, 25 |

## Task order and parallelism

Tasks are sequential (one implementer at a time). Tasks 1–6 are engine-side enablers and each ends with `make sim` unchanged. Tasks 7–8 fix the wire. Tasks 9–13 build `host` against `httptest`-free unit tests. Tasks 14–16 expose it. Tasks 17–23 build the client against the running `gorged`. Task 24 is the browser gate, Task 25 the done-when walk. M2r's first task (the `rules/trigger.go` split) may land before, between or after any task here — no file overlaps.

---

## Phase 0 — engine-side enablers

### Task 1: `Engine.Clone` and `Log.Clone`

**Files:**
- Modify: `events/log.go` (add `Clone`), `events/log_test.go`
- Modify: `rules/rng.go` (keep the `*rand.PCG`; add `clone`)
- Create: `rules/clone.go`, `rules/clone_test.go`

**Interfaces:**
- Consumes: `state.Game.Clone()` (exists), `math/rand/v2.PCG.MarshalBinary/UnmarshalBinary`, `rules.newTestBot` and `rules.diffGames` (test helpers in package `rules`).
- Produces: `func (l *events.Log) Clone() *events.Log`; `func (e *rules.Engine) Clone() *rules.Engine`. Task 11 calls `Clone` at every turn start and again per view request.

- [ ] **Step 1: Write the failing `Log.Clone` test**

Append to `events/log_test.go`:

```go
func TestLogCloneIsIndependentAndKeepsTheChain(t *testing.T) {
	l := NewLog(9)
	l.Append(Event{Kind: GameStart, Amount: 2})
	l.Append(Event{Kind: Shuffle, Player: 1, IDs: []state.ObjID{3, 1, 2}, Secret: true})
	l.Intents = append(l.Intents, decision.Intent{Seq: 2, Player: 0, Choices: []int{1}})

	c := l.Clone()
	if c.Head() != l.Head() || c.HeadAt(2) != l.HeadAt(2) {
		t.Fatalf("clone head %s / %s, want %s / %s", c.Head(), c.HeadAt(2), l.Head(), l.HeadAt(2))
	}
	if c.Seed != l.Seed || len(c.Events) != 2 || len(c.Intents) != 1 {
		t.Fatalf("clone did not copy seed/events/intents: %+v", c)
	}

	// Appending to the original must not move the clone, and vice versa.
	l.Append(Event{Kind: TurnChange, Player: 0, Amount: 1})
	if len(c.Events) != 2 || c.Head() == l.Head() {
		t.Fatal("appending to the original moved the clone")
	}
	c.Append(Event{Kind: Priority, Player: 1})
	if len(l.Events) != 3 {
		t.Fatal("appending to the clone moved the original")
	}

	// No shared backing arrays: mutating a cloned event's IDs or an
	// intent's Choices leaves the original untouched.
	c.Events[1].IDs[0] = 99
	c.Intents[0].Choices[0] = 42
	if l.Events[1].IDs[0] != 3 || l.Intents[0].Choices[0] != 1 {
		t.Fatal("clone shares a backing array with the original")
	}
	// The clone's chain continues correctly from the copied state.
	if c.Head() != c.HeadAt(len(c.Events)) {
		t.Fatalf("clone chain desynced: Head %s, HeadAt %s", c.Head(), c.HeadAt(len(c.Events)))
	}
}
```

`events/log_test.go` already imports `decision` and `state`; if not, add `"github.com/adams-shaun/gorge/decision"` and `"github.com/adams-shaun/gorge/state"`.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./events/ -run TestLogCloneIsIndependentAndKeepsTheChain -count=1`
Expected: FAIL — `l.Clone undefined`.

- [ ] **Step 3: Implement `Log.Clone`**

Append to `events/log.go`:

```go
// Clone returns an independent copy of l: the same Seed, NoHash and chain
// state, and fresh copies of Events and Intents (down to each event's IDs
// and Pairs and each intent's Choices). Appending to either log afterwards
// leaves the other untouched, and the copy's Head continues from exactly
// where l's was. rules.Engine.Clone uses it to snapshot a match.
func (l *Log) Clone() *Log {
	c := *l
	c.Events = make([]Event, len(l.Events))
	for i, e := range l.Events {
		e.IDs = append([]state.ObjID(nil), e.IDs...)
		e.Pairs = append([][2]state.ObjID(nil), e.Pairs...)
		c.Events[i] = e
	}
	c.Intents = make([]decision.Intent, len(l.Intents))
	for i, in := range l.Intents {
		in.Choices = append([]int(nil), in.Choices...)
		c.Intents[i] = in
	}
	c.buf = make([]byte, 0, cap(l.buf))
	return &c
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./events/ -count=1`
Expected: PASS (whole package).

- [ ] **Step 5: Write the failing `Engine.Clone` test**

Create `rules/clone_test.go`:

```go
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// drive answers n decisions with the package's own testBot and returns the
// intents it submitted, so the same choices can be replayed elsewhere.
func drive(t *testing.T, e *Engine, b *testBot, n int) []decision.Intent {
	t.Helper()
	var out []decision.Intent
	for i := 0; i < n && !e.G.Over && e.Pending() != nil; i++ {
		d := e.Pending()
		in := b.answer(e.G.Step.IsMain(), d)
		if err := e.Submit(in); err != nil {
			t.Fatalf("intent %d: %v", i, err)
		}
		out = append(out, in)
	}
	return out
}

func TestCloneStaysIndependentAndReplaysInLockstep(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 4)
	e := New(Config{Seed: 7, Names: names, Decks: decks})
	e.Advance()
	bot := newTestBot(7)
	drive(t, e, bot, 40)

	c := e.Clone()
	headBefore, drawsBefore, eventsBefore := e.L.Head(), e.RNGDraws(), len(e.L.Events)
	if got := diffGames(e.G, c.G); got != "" {
		t.Fatalf("clone differs from original at the boundary: %s", got)
	}

	// Diverge the clone by 60 more decisions; the original must not move.
	recorded := drive(t, c, bot, 60)
	if len(recorded) == 0 {
		t.Fatal("clone accepted no intents")
	}
	if e.L.Head() != headBefore || e.RNGDraws() != drawsBefore || len(e.L.Events) != eventsBefore {
		t.Fatal("driving the clone changed the original")
	}

	// Feed the original the very same intents: identical events, chain head
	// and RNG position mean the clone copied every piece of engine state.
	for i, in := range recorded {
		if err := e.Submit(in); err != nil {
			t.Fatalf("original rejected recorded intent %d: %v", i, err)
		}
	}
	if e.L.Head() != c.L.Head() {
		t.Fatalf("chain heads differ after lockstep: %s vs %s", e.L.Head(), c.L.Head())
	}
	if e.RNGDraws() != c.RNGDraws() {
		t.Fatalf("RNG draws differ: %d vs %d", e.RNGDraws(), c.RNGDraws())
	}
	if got := diffGames(e.G, c.G); got != "" {
		t.Fatalf("games differ after lockstep: %s", got)
	}
	if len(e.pendingTriggers) != len(c.pendingTriggers) || len(e.continuous) != len(c.continuous) {
		t.Fatalf("engine-internal queues differ: triggers %d/%d, continuous %d/%d",
			len(e.pendingTriggers), len(c.pendingTriggers), len(e.continuous), len(c.continuous))
	}
}

func TestCloneSharesNoMutableStateWithTheOriginal(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	e := New(Config{Seed: 3, Names: names, Decks: decks})
	e.Advance()
	drive(t, e, newTestBot(3), 30)
	c := e.Clone()

	// Mutate every cloned collection that has a backing array or map.
	c.G.Players[0].Life = -100
	c.L.Events[0].Kind = 200
	if c.pending != nil && len(c.pending.Options) > 0 {
		c.pending.Options[0].Label = "mutated"
	}
	for i := range c.continuous {
		c.continuous[i].AddKeywords = append(c.continuous[i].AddKeywords, "Mutated")
	}
	for i := range c.pendingTriggers {
		if c.pendingTriggers[i].Ctx.SVars != nil {
			c.pendingTriggers[i].Ctx.SVars["mutated"] = "yes"
		}
	}
	c.triggerFireCount = map[triggerKey]int32{{Source: 1, Idx: 0}: 99}

	if e.G.Players[0].Life == -100 || e.L.Events[0].Kind == 200 {
		t.Fatal("clone shares Game or Log storage with the original")
	}
	if e.pending != nil && len(e.pending.Options) > 0 && e.pending.Options[0].Label == "mutated" {
		t.Fatal("clone shares the pending decision's Options")
	}
	for _, ce := range e.continuous {
		for _, k := range ce.AddKeywords {
			if k == "Mutated" {
				t.Fatal("clone shares a continuous effect's AddKeywords")
			}
		}
	}
	for _, pt := range e.pendingTriggers {
		if pt.Ctx.SVars["mutated"] == "yes" {
			t.Fatal("clone shares a pending trigger's SVars map")
		}
	}
	if e.triggerFireCount[triggerKey{Source: 1, Idx: 0}] == 99 {
		t.Fatal("clone shares triggerFireCount")
	}
}

func TestCloneOfAFinishedGameIsFinished(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	e := New(Config{Seed: 11, Names: names, Decks: decks})
	e.Advance()
	drive(t, e, newTestBot(11), 400000)
	if !e.G.Over {
		t.Fatal("fixture game did not finish")
	}
	c := e.Clone()
	if !c.G.Over || c.L.Head() != e.L.Head() || c.Pending() != nil {
		t.Fatal("clone of a finished game is not finished with the same head")
	}
	if err := c.Submit(decision.Intent{}); err == nil {
		t.Fatal("clone of a finished game accepted an intent")
	}
}
```

- [ ] **Step 6: Run it to verify it fails**

Run: `go test ./rules/ -run 'TestClone' -count=1`
Expected: FAIL — `e.Clone undefined`.

- [ ] **Step 7: Keep the PCG in `rng` and add `clone`**

In `rules/rng.go` change the struct and constructor and add `clone`:

```go
type rng struct {
	src   *rand.Rand
	pcg   *rand.PCG // kept so clone can copy the generator's exact position
	Draws uint64
	seed  [2]uint64
}

func newRNG(seed uint64) *rng {
	s := [2]uint64{seed, seed ^ 0x9e3779b97f4a7c15}
	pcg := rand.NewPCG(s[0], s[1])
	return &rng{src: rand.New(pcg), pcg: pcg, seed: s}
}

// clone copies the generator at its exact position: the next IntN on the
// copy returns what the next IntN on the original would. PCG's binary
// marshalling is the stdlib's own round-trip for that state.
func (r *rng) clone() *rng {
	raw, err := r.pcg.MarshalBinary()
	if err != nil {
		panic("rules: PCG MarshalBinary: " + err.Error())
	}
	pcg := &rand.PCG{}
	if err := pcg.UnmarshalBinary(raw); err != nil {
		panic("rules: PCG UnmarshalBinary: " + err.Error())
	}
	return &rng{src: rand.New(pcg), pcg: pcg, Draws: r.Draws, seed: r.seed}
}
```

- [ ] **Step 8: Implement `Engine.Clone`**

Create `rules/clone.go`:

```go
package rules

import (
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// Clone deep-copies the engine: game, log, RNG position, the pending
// decision, continuous effects, the pending-trigger queue and the trigger
// bookkeeping maps. The copy and the original then evolve independently —
// the same intents submitted to both produce the same events, chain head
// and RNG draw count (clone_test.go pins that), and nothing submitted to
// one is visible to the other. Card data (*cards.Card, *cards.SA) is
// shared: the compiled corpus is immutable once loaded.
//
// Call it only at an intent boundary — after New, Advance or Submit has
// returned, so Pending() != nil or G.Over. That is the only moment the
// fields below are not being written. The host (package host) clones at
// every turn start to answer "view at seq N" with at most one turn of
// replay.
func (e *Engine) Clone() *Engine {
	c := &Engine{
		G:                   e.G.Clone(),
		L:                   e.L.Clone(),
		rng:                 e.rng.clone(),
		orderedTriggers:     e.orderedTriggers,
		applyingReplacement: e.applyingReplacement,
	}
	if e.pending != nil {
		d := *e.pending
		d.Options = append([]decision.Option(nil), e.pending.Options...)
		c.pending = &d
	}
	if e.continuous != nil {
		c.continuous = make([]ContinuousEffect, len(e.continuous))
		for i, ce := range e.continuous {
			ce.AddKeywords = append([]string(nil), ce.AddKeywords...)
			ce.AddTypes = append([]string(nil), ce.AddTypes...)
			c.continuous[i] = ce
		}
	}
	if e.pendingTriggers != nil {
		c.pendingTriggers = make([]pendingTrigger, len(e.pendingTriggers))
		for i, pt := range e.pendingTriggers {
			pt.Ctx.Targets = append([]state.Target(nil), pt.Ctx.Targets...)
			pt.Ctx.Remembered = append([]state.Target(nil), pt.Ctx.Remembered...)
			if pt.Ctx.SVars != nil {
				m := make(map[string]string, len(pt.Ctx.SVars))
				for k, v := range pt.Ctx.SVars {
					m[k] = v
				}
				pt.Ctx.SVars = m
			}
			c.pendingTriggers[i] = pt
		}
	}
	c.triggerFireCount = cloneCounts(e.triggerFireCount)
	c.damageOnceFired = cloneCounts(e.damageOnceFired)
	return c
}

// cloneCounts copies a trigger-bookkeeping map, preserving nil (trigger.go
// lazily allocates these on first use and checks for nil itself).
func cloneCounts(m map[triggerKey]int32) map[triggerKey]int32 {
	if m == nil {
		return nil
	}
	out := make(map[triggerKey]int32, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
```

If `Engine` has gained a field since this plan was written (`grep -n '^\t[a-zA-Z]' rules/engine.go` between `type Engine struct {` and `}`), copy it too, deep if it has a backing array or map — the test in Step 5 compares the internal queue lengths and the chain head after lockstep and will catch a missed field that affects play.

- [ ] **Step 9: Run the tests, then the whole rules package with `-race`**

Run: `go test ./rules/ -run 'TestClone' -count=1 -v`
Expected: PASS (three tests).
Run: `go test -race -count=1 ./rules/ ./events/`
Expected: PASS.

- [ ] **Step 10: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./... && make sim | grep -c 'replay OK'`
Expected: lint clean, build ok, all packages ok, `20`. `go test ./rules/ -run 'TestRepoDeckGamesReplayExactly' -v -count=1` prints the four chain heads unchanged (`7705a6505954f6cd`, `2d5589b31c4853cd`, `bf4012092fdad38b`, `01b9f48c1b6dc135`).

```bash
git add events/log.go events/log_test.go rules/rng.go rules/clone.go rules/clone_test.go
git commit -m "feat(rules): Engine.Clone and Log.Clone for host snapshots

An engine copied at an intent boundary replays the same intents to the
same chain head and RNG position, and shares no mutable state with the
original. The PCG is kept on rng so its exact position can be copied.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 2: `cards.OpenCorpus` and the `deck` package

**Files:**
- Create: `cards/open.go`, `cards/open_test.go`
- Create: `deck/deck.go`, `deck/deck_test.go`, `deck/testdata/tiny.json`
- Modify: `internal/testutil/decks.go` (delegate `OpenCorpusRegistry` and `LoadRepoDeck`)

**Interfaces:**
- Consumes: `cards.LoadRegistry(path)`, `cards.CompileDir(dir)`, `cards.CorpusDir(dir)`, `cards.Registry.Lookup(name)`, `cards.NormalizeName`.
- Produces: `func cards.OpenCorpus(dir string) (*Registry, error)`; package `deck` with `type File struct{ Name, Format string; Cards []Entry }`, `type Entry struct{ Name string; Count int }`, `func Parse(raw []byte) (File, error)`, `func (f File) Resolve(r *cards.Registry) ([]*cards.Card, error)`, `func Load(r *cards.Registry, path string) (File, []*cards.Card, error)`, `func Stem(path string) string`. Tasks 9 and 16 use `deck.Load` and `cards.OpenCorpus`.

- [ ] **Step 1: Write the failing `deck` tests**

Create `deck/testdata/tiny.json`:

```json
{
  "name": "Tiny Test Deck",
  "format": "legacy",
  "cards": [
    { "name": "Mountain", "count": 2 },
    { "name": "Goblin Guide", "count": 1 }
  ]
}
```

Create `deck/deck_test.go`:

```go
package deck

import (
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

// fixtureRegistry holds two synthetic cards so the tests need no corpus.
// The scripts are authored here, not copied from Forge.
func fixtureRegistry(t *testing.T) *cards.Registry {
	t.Helper()
	r := cards.NewRegistry()
	for _, src := range []string{
		"Name:Mountain\nTypes:Basic Land Mountain\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ Add {R}.\nOracle:({T}: Add {R}.)\n",
		"Name:Goblin Guide\nManaCost:R\nTypes:Creature Goblin Scout\nPT:2/2\nK:Haste\nOracle:Haste\n",
	} {
		c, diags := cards.ParseBytes("fixture.txt", []byte(src))
		if len(diags) > 0 {
			t.Fatalf("fixture parse: %v", diags)
		}
		r.Add(c)
	}
	return r
}

func TestParseReadsNameFormatAndEntries(t *testing.T) {
	raw, err := os.ReadFile("testdata/tiny.json")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "Tiny Test Deck" || f.Format != "legacy" || len(f.Cards) != 2 {
		t.Fatalf("parsed %+v", f)
	}
	if f.Cards[0].Name != "Mountain" || f.Cards[0].Count != 2 {
		t.Fatalf("first entry %+v", f.Cards[0])
	}
}

func TestParseRejectsMalformedInput(t *testing.T) {
	for _, raw := range []string{"", "{", `{"cards":[{"name":"","count":1}]}`, `{"cards":[{"name":"Mountain","count":0}]}`} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Errorf("Parse(%q) accepted malformed input", raw)
		}
	}
}

func TestResolveExpandsCountsInOrder(t *testing.T) {
	r := fixtureRegistry(t)
	f := File{Cards: []Entry{{"Mountain", 2}, {"Goblin Guide", 1}}}
	cs, err := f.Resolve(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 3 || cs[0].Faces[0].Name != "Mountain" || cs[1] != cs[0] || cs[2].Faces[0].Name != "Goblin Guide" {
		t.Fatalf("resolved %d cards: %v", len(cs), cs)
	}
}

func TestResolveNamesTheFirstUnknownCard(t *testing.T) {
	r := fixtureRegistry(t)
	f := File{Name: "x", Cards: []Entry{{"Mountain", 1}, {"Black Lotus", 1}}}
	_, err := f.Resolve(r)
	if err == nil || !strings.Contains(err.Error(), "Black Lotus") {
		t.Fatalf("want an error naming Black Lotus, got %v", err)
	}
}

func TestLoadAndStem(t *testing.T) {
	r := fixtureRegistry(t)
	f, cs, err := Load(r, "testdata/tiny.json")
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "Tiny Test Deck" || len(cs) != 3 {
		t.Fatalf("Load: %+v, %d cards", f, len(cs))
	}
	if got := Stem("internal/testutil/decks/mono-red-goblins.json"); got != "mono-red-goblins" {
		t.Fatalf("Stem = %q", got)
	}
	if _, _, err := Load(r, "testdata/missing.json"); err == nil {
		t.Fatal("Load of a missing file succeeded")
	}
}
```

`cards.ParseBytes(path string, src []byte) (*Card, []Diag)` is the in-memory parser (`view/view_test.go` builds its fixtures the same way).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./deck/ -count=1`
Expected: FAIL — package has no non-test Go files / `Parse undefined`.

- [ ] **Step 3: Implement `deck`**

Create `deck/deck.go`:

```go
// Package deck reads the repo's deck-list JSON — a bare {name, count} card
// list — and resolves it against a cards.Registry into the flat, repeated
// []*cards.Card that rules.Config.Decks wants. It is the one parser both
// the test fixtures (internal/testutil) and the match host use, so the two
// can never disagree about what a deck file means. A deck list carries no
// card text and is not the licensing hazard cards/boundary_test.go guards.
package deck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/cards"
)

// File is the on-disk shape. Name and Format are authoring metadata; only
// Cards decides what is dealt.
type File struct {
	Name   string  `json:"name"`
	Format string  `json:"format"`
	Cards  []Entry `json:"cards"`
}

// Entry is one line of a deck list.
type Entry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Parse decodes a deck file and rejects the shapes that would otherwise
// fail later in a less obvious place: no cards, an unnamed entry, a
// non-positive count.
func Parse(raw []byte) (File, error) {
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return File{}, fmt.Errorf("deck: %w", err)
	}
	if len(f.Cards) == 0 {
		return File{}, fmt.Errorf("deck: no cards")
	}
	for i, e := range f.Cards {
		if e.Name == "" {
			return File{}, fmt.Errorf("deck: entry %d has no name", i)
		}
		if e.Count <= 0 {
			return File{}, fmt.Errorf("deck: %q has count %d", e.Name, e.Count)
		}
	}
	return f, nil
}

// Resolve looks every entry up in r (which normalises names itself) and
// expands it by its count, in file order. The first unknown card is named
// in the error.
func (f File) Resolve(r *cards.Registry) ([]*cards.Card, error) {
	var out []*cards.Card
	for _, e := range f.Cards {
		c, ok := r.Lookup(e.Name)
		if !ok {
			return nil, fmt.Errorf("deck %q: card %q is not in the registry", f.Name, e.Name)
		}
		for i := 0; i < e.Count; i++ {
			out = append(out, c)
		}
	}
	return out, nil
}

// Load reads, parses and resolves one deck file.
func Load(r *cards.Registry, path string) (File, []*cards.Card, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return File{}, nil, fmt.Errorf("deck: %w", err)
	}
	f, err := Parse(raw)
	if err != nil {
		return File{}, nil, fmt.Errorf("%s: %w", path, err)
	}
	cs, err := f.Resolve(r)
	if err != nil {
		return File{}, nil, err
	}
	return f, cs, nil
}

// Stem is the deck's short name: the file name without directory or
// extension ("decks/mono-red-goblins.json" -> "mono-red-goblins"). The host
// names seats after it.
func Stem(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}
```

- [ ] **Step 4: Run `deck` tests**

Run: `go test ./deck/ -count=1`
Expected: PASS.

- [ ] **Step 5: Write the failing `cards.OpenCorpus` test**

Create `cards/open_test.go`:

```go
package cards

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeScript writes one synthetic card script into dir/cardsfolder. The
// text is authored here; it is not a Forge file.
func writeScript(t *testing.T, dir, name, body string) {
	t.Helper()
	folder := CorpusDir(dir)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCorpusFailsCleanlyWhenNothingIsThere(t *testing.T) {
	if _, err := OpenCorpus(t.TempDir()); err == nil {
		t.Fatal("OpenCorpus on an empty dir returned no error")
	}
}

func TestOpenCorpusCompilesWhenThereIsNoCache(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "mountain.txt", "Name:Mountain\nTypes:Basic Land Mountain\nOracle:\n")
	r, err := OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Lookup("Mountain"); !ok {
		t.Fatal("compiled registry lacks Mountain")
	}
}

func TestOpenCorpusPrefersAFreshCacheAndRecompilesAStaleOne(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "mountain.txt", "Name:Mountain\nTypes:Basic Land Mountain\nOracle:\n")
	// A cache holding a different card: fresh cache wins over the folder.
	r := NewRegistry()
	c, _ := ParseBytes("island.txt", []byte("Name:Island\nTypes:Basic Land Island\nOracle:\n"))
	r.Add(c)
	cache := filepath.Join(dir, "ir.gob.gz")
	if err := r.Save(cache); err != nil {
		t.Fatal(err)
	}
	got, err := OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Lookup("Island"); !ok {
		t.Fatal("fresh cache was not used")
	}
	// Now a cards.lock newer than the cache marks it stale: recompile.
	lock := filepath.Join(dir, "cards.lock")
	if err := os.WriteFile(lock, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(lock, future, future); err != nil {
		t.Fatal(err)
	}
	got, err = OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Lookup("Mountain"); !ok {
		t.Fatal("stale cache was not recompiled from the folder")
	}
}
```

(`time` in a `cards` test file is allowed: Task 6's constraint walks non-test files only.)

- [ ] **Step 6: Run to verify it fails**

Run: `go test ./cards/ -run TestOpenCorpus -count=1`
Expected: FAIL — `OpenCorpus undefined`.

- [ ] **Step 7: Implement `cards.OpenCorpus` by moving the logic from testutil**

Create `cards/open.go` with the body of `internal/testutil.OpenCorpusRegistry` (verbatim logic, package-local names):

```go
package cards

import (
	"os"
	"path/filepath"
)

// OpenCorpus loads dir's compiled IR cache (dir/ir.gob.gz), or compiles
// dir/cardsfolder afresh when the cache is absent, unreadable, or older
// than dir/cards.lock — a fetch since the last compile invalidates it, the
// same staleness rule forgec's own fetch-then-compile pipeline assumes. It
// returns a plain error, never a panic, when neither cache nor corpus is
// present, so a clean checkout with nothing fetched is the caller's
// decision (tests Skip; a server refuses to start).
func OpenCorpus(dir string) (*Registry, error) {
	cache := filepath.Join(dir, "ir.gob.gz")
	cacheInfo, cacheErr := os.Stat(cache)
	lockInfo, lockErr := os.Stat(filepath.Join(dir, "cards.lock"))
	stale := cacheErr != nil || (lockErr == nil && lockInfo.ModTime().After(cacheInfo.ModTime()))
	if !stale {
		if r, err := LoadRegistry(cache); err == nil {
			return r, nil
		}
	}
	r, _, err := CompileDir(CorpusDir(dir))
	if err != nil {
		return nil, err
	}
	return r, nil
}
```

Then in `internal/testutil/decks.go`: replace the body of `OpenCorpusRegistry` with `return cards.OpenCorpus(dir)`, and replace the body of `LoadRepoDeck` with

```go
	raw, err := decksFS.ReadFile(path.Join(decksDir, name+".json"))
	if err != nil {
		return nil, fmt.Errorf("testutil: deck %q: %w", name, err)
	}
	f, err := deck.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("testutil: deck %q: %w", name, err)
	}
	cs, err := f.Resolve(r)
	if err != nil {
		return nil, fmt.Errorf("testutil: %w", err)
	}
	return cs, nil
```

(drop the now-unused `deckFile` type and `encoding/json` import; keep every doc comment that explains *why*, trimmed of the moved mechanics). `ModTime().After` uses a `time.Time` method without importing `time`; keep it that way.

- [ ] **Step 8: Run tests, gates, commit**

Run: `go test ./cards/ ./deck/ ./internal/... -count=1 && go test ./rules/ -run 'TestEveryRepoDeck|TestRepoDecks' -count=1`
Expected: PASS; the acceptance tests still load all 12 decks.
Run: `make lint && go build ./... && go test -count=1 ./... && make sim | grep -c 'replay OK'`
Expected: clean, `20`.

```bash
git add cards/open.go cards/open_test.go deck/ internal/testutil/decks.go
git commit -m "feat(deck): deck-list parser and cards.OpenCorpus as leaf packages

The match host and gorged need both without importing internal/testutil,
which now delegates to them.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 3: `view` visibility — `Public`, `Omniscient`, `NoSeat`

**Files:**
- Create: `view/visibility.go`, `view/visibility_test.go`
- Modify: `view/view.go` (`View.Visibility` field; `Project` becomes a wrapper), `view/redact.go` (`RedactEvents` becomes a wrapper)

**Interfaces:**
- Consumes: existing `Project`, `RedactEvents`, `Chars`.
- Produces: `type Visibility uint8` with `Seat`, `Public`, `Omniscient` and `String()`; `const NoSeat state.PlayerID = 255`; `func ParseVisibility(s string) (Visibility, error)`; `func ProjectFor(g *state.Game, ch Chars, viewer state.PlayerID, vis Visibility, d *decision.Decision) View`; `func RedactEventsFor(g *state.Game, evs []events.Event, viewer state.PlayerID, vis Visibility) []events.Event`; `View.Visibility string` (json `visibility`). Tasks 9–11 project every spectator view with `ProjectFor(g, e, view.NoSeat, table.Spectator, nil)`.

- [ ] **Step 1: Write the failing tests**

Create `view/visibility_test.go`:

```go
package view

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/state"
)

// playSome runs a real 4-seat game for n decisions and returns the engine,
// so visibility is tested against real hands, libraries and a real log.
func playSome(t *testing.T, seed uint64, n int) *rules.Engine {
	t.Helper()
	names, decks := testutil.SampleDecks(t, 4)
	e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks})
	e.Advance()
	b := seat.NewBot(seed)
	for i := 0; i < n && !e.G.Over && e.Pending() != nil; i++ {
		d := e.Pending()
		in, err := b.Decide(context.Background(), Project(e.G, e, d.Player, d), *d)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("intent %d: %v", i, err)
		}
	}
	return e
}

func TestVisibilityStringsRoundTrip(t *testing.T) {
	for _, v := range []Visibility{Seat, Public, Omniscient} {
		got, err := ParseVisibility(v.String())
		if err != nil || got != v {
			t.Errorf("ParseVisibility(%q) = %v, %v", v.String(), got, err)
		}
	}
	if _, err := ParseVisibility("godmode"); err == nil {
		t.Error("ParseVisibility accepted an unknown mode")
	}
	if Visibility(9).String() != "unknown" {
		t.Error("out-of-range Visibility does not print unknown")
	}
}

func TestProjectForPublicShowsNoHandNoPoolNoDecision(t *testing.T) {
	e := playSome(t, 5, 60)
	v := ProjectFor(e.G, e, NoSeat, Public, e.Pending())
	if v.Visibility != "public" || v.Viewer != NoSeat {
		t.Fatalf("header %+v", v)
	}
	for _, p := range v.Players {
		if p.Hand != nil || p.Pool != nil {
			t.Fatalf("public view exposes seat %d's hand or pool", p.ID)
		}
		if p.HandSize != len(e.G.Zone(state.ZHand, p.ID)) {
			t.Fatalf("seat %d hand size %d, want %d", p.ID, p.HandSize, len(e.G.Zone(state.ZHand, p.ID)))
		}
	}
	if v.Decision != nil {
		t.Fatal("public view carries a decision")
	}
}

func TestProjectForOmniscientShowsEveryHandAndPoolButNoLibraryOrder(t *testing.T) {
	e := playSome(t, 5, 60)
	v := ProjectFor(e.G, e, NoSeat, Omniscient, e.Pending())
	if v.Visibility != "omniscient" {
		t.Fatalf("visibility %q", v.Visibility)
	}
	for _, p := range v.Players {
		if p.Hand == nil || p.Pool == nil {
			t.Fatalf("omniscient view hides seat %d's hand or pool", p.ID)
		}
		if len(p.Hand) != len(e.G.Zone(state.ZHand, p.ID)) {
			t.Fatalf("seat %d: %d hand cards projected, %d in hand", p.ID, len(p.Hand), len(e.G.Zone(state.ZHand, p.ID)))
		}
		for _, cv := range p.Hand {
			if cv.Name == "" {
				t.Fatalf("seat %d has an unnamed hand card %+v", p.ID, cv)
			}
		}
		if p.LibrarySize != len(e.G.Zone(state.ZLibrary, p.ID)) {
			t.Fatalf("seat %d library size wrong", p.ID)
		}
	}
	// The View type has no library list at all; this pins that no field
	// was added that could carry one.
	if v.Decision != nil {
		t.Fatal("omniscient spectator carries a seat's decision")
	}
}

func TestProjectForSeatMatchesProject(t *testing.T) {
	e := playSome(t, 8, 40)
	d := e.Pending()
	for p := state.PlayerID(0); p < 4; p++ {
		a := Project(e.G, e, p, d)
		b := ProjectFor(e.G, e, p, Seat, d)
		a.Visibility, b.Visibility = "", ""
		if got, want := mustJSON(t, a), mustJSON(t, b); got != want {
			t.Fatalf("seat %d: ProjectFor(Seat) differs from Project", p)
		}
	}
	if Project(e.G, e, 0, d).Visibility != "seat" {
		t.Fatal("Project does not label itself seat")
	}
}

func TestRedactEventsForOmniscientHidesOnlyLibraryOrder(t *testing.T) {
	e := playSome(t, 5, 200)
	out := RedactEventsFor(e.G, e.L.Events, NoSeat, Omniscient)
	if len(out) != len(e.L.Events) {
		t.Fatal("omniscient redaction dropped events")
	}
	shuffles, notes, secretDraws := 0, 0, 0
	for i, ev := range out {
		orig := e.L.Events[i]
		switch {
		case orig.Kind == events.Shuffle:
			shuffles++
			if len(ev.IDs) != 0 {
				t.Fatalf("event %d: omniscient view sees library order", i)
			}
		case orig.Secret && orig.Kind == events.Note:
			notes++
			if ev.Text != "" || len(ev.IDs) != 0 {
				t.Fatalf("event %d: omniscient view sees a private library peek", i)
			}
		case orig.Secret:
			secretDraws++
			if ev.Obj != orig.Obj {
				t.Fatalf("event %d: a hidden-zone move was stripped in omniscient mode", i)
			}
		default:
			if string(ev.Append(nil)) != string(orig.Append(nil)) {
				t.Fatalf("event %d: a public event was altered in omniscient mode", i)
			}
		}
	}
	if shuffles == 0 || secretDraws == 0 {
		t.Fatalf("fixture exercised %d shuffles and %d secret draws; need both", shuffles, secretDraws)
	}
}

func TestRedactEventsForPublicMatchesRedactEventsForASpectator(t *testing.T) {
	e := playSome(t, 5, 200)
	a := RedactEvents(e.G, e.L.Events, NoSeat)
	b := RedactEventsFor(e.G, e.L.Events, NoSeat, Public)
	if len(a) != len(b) {
		t.Fatal("lengths differ")
	}
	for i := range a {
		if string(a[i].Append(nil)) != string(b[i].Append(nil)) {
			t.Fatalf("event %d differs between RedactEvents and RedactEventsFor(Public)", i)
		}
	}
}

func TestRedactEventsForNeverMutatesItsInput(t *testing.T) {
	e := playSome(t, 5, 100)
	before := make([]string, len(e.L.Events))
	for i, ev := range e.L.Events {
		before[i] = string(ev.Append(nil))
	}
	for _, vis := range []Visibility{Seat, Public, Omniscient} {
		out := RedactEventsFor(e.G, e.L.Events, NoSeat, vis)
		for i := range out {
			if len(out[i].IDs) > 0 {
				out[i].IDs[0] = 4242
			}
		}
	}
	for i, ev := range e.L.Events {
		if string(ev.Append(nil)) != before[i] {
			t.Fatalf("event %d in the engine's log was mutated through a redacted copy", i)
		}
	}
}
```

`mustJSON(t, v)` — add to this file if `view_test.go` does not already have one:

```go
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
```

(import `encoding/json`). `view` tests may import `rules` and `seat`: neither imports `view`'s test binary, and `view_test.go` already imports `rules` for its compile-time `Chars` assertion.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./view/ -run 'Visibility|ProjectFor|RedactEventsFor' -count=1`
Expected: FAIL — `Visibility undefined`.

- [ ] **Step 3: Implement `view/visibility.go`**

```go
package view

import (
	"fmt"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Visibility is how much of the hidden information a projection reveals.
// It is a property of the viewer's relationship to the table, not of the
// game: the same state.Game projects three different Views.
type Visibility uint8

const (
	// Seat is a player's own view: their hand, their mana pool, a decision
	// asked of them; every other seat's hidden zones are counts. This is
	// what Project has always produced.
	Seat Visibility = iota
	// Public is a spectator with no seat: every hidden zone is a count, no
	// decision is attached. Today's spectator redaction under a name.
	Public
	// Omniscient is a spectator who sees every hand and every mana pool —
	// the bot-table default — but never library order: it spoils draws and
	// teaches nothing (spec D12).
	Omniscient
)

var visibilityNames = [...]string{"seat", "public", "omniscient"}

// String is the wire name; an out-of-range value prints "unknown", the same
// total shape as state.Step.String.
func (v Visibility) String() string {
	if int(v) < len(visibilityNames) {
		return visibilityNames[v]
	}
	return "unknown"
}

// ParseVisibility is String's inverse for flags and table configs.
func ParseVisibility(s string) (Visibility, error) {
	for i, n := range visibilityNames {
		if n == s {
			return Visibility(i), nil
		}
	}
	return 0, fmt.Errorf("view: unknown visibility %q (want seat, public or omniscient)", s)
}

// NoSeat is the viewer id of a spectator. state.PlayerID is a uint8 and no
// table has 255 seats, so it can never collide with a real seat; Project's
// own "out-of-range viewer is a spectator" rule does the rest.
const NoSeat state.PlayerID = 255

// ProjectFor is Project with an explicit visibility. Seat is exactly
// Project. Public forces the spectator path regardless of viewer. Omniscient
// projects every seat's hand and pool; the decision is still attached only
// to the seat it was asked of, so a spectator never carries one.
func ProjectFor(g *state.Game, ch Chars, viewer state.PlayerID, vis Visibility, d *decision.Decision) View {
	switch vis {
	case Public:
		v := project(g, ch, NoSeat, nil)
		v.Viewer = viewer
		v.Visibility = vis.String()
		return v
	case Omniscient:
		v := project(g, ch, viewer, nil)
		if g != nil {
			for i := range v.Players {
				p := &g.Players[i]
				v.Players[i].Hand = cardViews(g, ch, g.Zone(state.ZHand, p.ID))
				v.Players[i].Pool = poolView(p.Pool)
			}
		}
		v.Visibility = vis.String()
		return v
	default:
		v := project(g, ch, viewer, d)
		v.Visibility = Seat.String()
		return v
	}
}

// RedactEventsFor is RedactEvents with an explicit visibility. Seat and
// Public are RedactEvents (a Public viewer is NoSeat, so every owner-only
// branch stays closed). Omniscient passes every event through unredacted
// except the two Secret kinds whose payload is library order — Shuffle
// (genesis order) and a Secret Note (a private look at the top of the
// library) — which keep only their shape. A Secret Draw or MoveZone out of
// the library passes: the card is now in a hand the omniscient viewer sees.
func RedactEventsFor(g *state.Game, evs []events.Event, viewer state.PlayerID, vis Visibility) []events.Event {
	switch vis {
	case Public:
		return RedactEvents(g, evs, NoSeat)
	case Omniscient:
		out := make([]events.Event, 0, len(evs))
		for _, e := range evs {
			e.IDs = append([]state.ObjID(nil), e.IDs...)
			e.Pairs = append([][2]state.ObjID(nil), e.Pairs...)
			if e.Secret && (e.Kind == events.Shuffle || e.Kind == events.Note) {
				out = append(out, events.Event{
					Seq: e.Seq, Kind: e.Kind, Player: e.Player,
					From: e.From, To: e.To, Step: e.Step, Secret: e.Secret,
				})
				continue
			}
			out = append(out, e)
		}
		return out
	default:
		return RedactEvents(g, evs, viewer)
	}
}

// poolView is the viewer-facing mana pool: only the symbols with mana in
// them, always non-nil so an empty pool marshals "{}" rather than null.
func poolView(m state.Mana) map[string]int32 {
	pool := map[string]int32{}
	for idx, sym := range [...]string{"W", "U", "B", "R", "G", "C"} {
		if n := m[idx]; n > 0 {
			pool[sym] = n
		}
	}
	return pool
}
```

Then in `view/view.go`:
- Rename the existing `Project` body to `func project(g *state.Game, ch Chars, viewer state.PlayerID, d *decision.Decision) View` (unexported) and replace its inline pool loop with `pv.Pool = poolView(p.Pool)`.
- Add `func Project(g *state.Game, ch Chars, viewer state.PlayerID, d *decision.Decision) View { return ProjectFor(g, ch, viewer, Seat, d) }` with its existing doc comment plus one sentence: "Project is ProjectFor with Seat visibility."
- Add a field to `View` right after `Viewer`:

```go
	// Visibility names which rule set built this view: "seat", "public" or
	// "omniscient" (see Visibility).
	Visibility string `json:"visibility"`
```

- [ ] **Step 4: Run the tests**

Run: `go test ./view/ -count=1 && go test -race -count=1 ./view/`
Expected: PASS; every pre-existing view test still passes (Project's behaviour is unchanged apart from the new `visibility` key — if a golden JSON test in `view_test.go` compares whole documents, update it to include `"visibility":"seat"` and say so in the commit body).

- [ ] **Step 5: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./... && make sim | grep -c 'replay OK'`
Expected: clean, `20`; chain heads unchanged (`view` changes nothing in the log).

```bash
git add view/visibility.go view/visibility_test.go view/view.go view/redact.go
git commit -m "feat(view): Public and Omniscient visibility for spectators

ProjectFor/RedactEventsFor take an explicit Visibility. Omniscient shows
every hand and pool and hides exactly library order (Secret Shuffle and
Secret Note keep only their shape). NoSeat names a spectator viewer.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 4: `view` identity — printing, token, mana cost, trigger kind, target labels

**Files:**
- Modify: `view/view.go`
- Create: `view/identity_test.go`

**Interfaces:**
- Consumes: `cards.Face.ManaCost`, `cards.Face.Triggers[i].Effect`, `cards.SA.Params["TgtPrompt"]`/`["ValidTgts"]`, `cards.SA.Sub`.
- Produces: `type Printing struct{ Name, Set, Number string }`; `CardView.ManaCost string` (json `mana_cost`), `CardView.Printing Printing` (json `printing`), `CardView.Token string` (json `token`, always `"#<ObjID>"`), `CardView.AttackingPlayer *state.PlayerID` (json `attacking_player`), `CardView.BlockedBy []state.ObjID` (json `blocked_by`); `StackView.Kind ∈ {"spell","ability","trigger"}`; `TargetView.Label string` (json `label`). Task 8 generates these into TypeScript; Tasks 20–23 render them.

- [ ] **Step 1: Write the failing tests**

Create `view/identity_test.go`:

```go
package view

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

const watcherSrc = "Name:Watcher\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigGain | TriggerDescription$ When CARDNAME enters, you gain 1 life.\n" +
	"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"

const boltSrc = "Name:Bolt\nManaCost:R\nTypes:Instant\n" +
	"A:SP$ DealDamage | ValidTgts$ Any | TgtPrompt$ Select any target | NumDmg$ 3 | SpellDescription$ Bolt deals 3 damage to any target.\nOracle:x\n"

const shockCreatureSrc = "Name:Shocker\nManaCost:R\nTypes:Instant\n" +
	"A:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2 | SpellDescription$ Shocker deals 2 damage to target creature.\nOracle:x\n"

// twoSeatWith builds a two-seat game with one copy of src in seat 0's hand
// and returns the game and that object's id.
func twoSeatWith(t *testing.T, src string) (*state.Game, state.ObjID) {
	t.Helper()
	c, diags := cards.ParseBytes("fixture.txt", []byte(src))
	if len(diags) > 0 {
		t.Fatalf("fixture parse: %v", diags)
	}
	g := state.NewGame([]string{"Ann", "Bob"})
	o := g.AddObject(c, 0)
	g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})
	return g, o.ID
}

func TestCardViewCarriesPrintingTokenAndManaCost(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	v := Project(g, flatChars{g}, 0, nil)
	if len(v.Players[0].Battlefield) != 1 {
		t.Fatalf("battlefield %+v", v.Players[0].Battlefield)
	}
	cv := v.Players[0].Battlefield[0]
	if cv.Printing.Name != "Watcher" || cv.Printing.Set != "" || cv.Printing.Number != "" {
		t.Fatalf("printing %+v", cv.Printing)
	}
	if cv.Token != "#1" {
		t.Fatalf("token %q, want #1", cv.Token)
	}
	if cv.ManaCost != "1 W" {
		t.Fatalf("mana cost %q", cv.ManaCost)
	}
}

func TestStackViewKindIsTriggerForATriggerPushObject(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	if g.Obj(id).Face().Triggers[0].Effect == nil {
		t.Fatal("fixture: the T: line's Execute$ did not link to its SVar; check cards/link.go's entry point")
	}
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	events.Apply(g, events.Event{Kind: events.TriggerPush, Player: 0, Obj: id, Amount: 0})
	v := Project(g, flatChars{g}, 1, nil)
	if len(v.Stack) != 1 {
		t.Fatalf("stack %+v", v.Stack)
	}
	sv := v.Stack[0]
	if sv.Kind != "trigger" || sv.Name != "Watcher" || sv.Source != id {
		t.Fatalf("stack view %+v", sv)
	}
	if sv.Text != "When CARDNAME enters, you gain 1 life." {
		t.Fatalf("text %q", sv.Text)
	}
}

func TestStackViewKindIsAbilityForANonTriggerAbilityObject(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	// An ability object whose SA is not one of the source's T: lines — the
	// shape an activated ability will have once the engine enumerates them.
	ab := g.AddObject(nil, 0)
	events.Move(g, ab.ID, state.ZLibrary, state.ZStack)
	ab.Ability = &cards.SA{Kind: "AB", API: "GainLife", Params: map[string]string{"SpellDescription": "Gain 1 life."}}
	ab.Source = id
	v := Project(g, flatChars{g}, 0, nil)
	if len(v.Stack) != 1 || v.Stack[0].Kind != "ability" || v.Stack[0].Text != "Gain 1 life." {
		t.Fatalf("stack %+v", v.Stack)
	}
}

func TestTargetLabelPrefersTgtPromptThenValidTgts(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{boltSrc, "Select any target"},
		{shockCreatureSrc, "Creature"},
	} {
		g, id := twoSeatWith(t, tc.src)
		events.Apply(g, events.Event{Kind: events.PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
		events.Apply(g, events.Event{Kind: events.TargetsChosen, Obj: id, Player: 1, Amount: 1})
		v := Project(g, flatChars{g}, 1, nil)
		if len(v.Stack) != 1 || len(v.Stack[0].Targets) != 1 {
			t.Fatalf("%s: stack %+v", tc.want, v.Stack)
		}
		tg := v.Stack[0].Targets[0]
		if !tg.IsPlayer || tg.Player != 1 || tg.Label != tc.want {
			t.Fatalf("target %+v, want label %q", tg, tc.want)
		}
	}
}

func TestCardViewCarriesCombatRelationships(t *testing.T) {
	g, attacker := twoSeatWith(t, watcherSrc)
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: attacker, From: state.ZHand, To: state.ZBattlefield})
	c, _ := cards.ParseBytes("b.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	blocker := g.AddObject(c, 1).ID
	g.SetZone(state.ZHand, 1, []state.ObjID{blocker})
	events.Apply(g, events.Event{Kind: events.MoveZone, Obj: blocker, From: state.ZHand, To: state.ZBattlefield})
	events.Apply(g, events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{attacker}})
	events.Apply(g, events.Event{Kind: events.DeclareBlockers, Pairs: [][2]state.ObjID{{attacker, blocker}}})
	v := Project(g, flatChars{g}, 0, nil)
	a := v.Players[0].Battlefield[0]
	if !a.Attacking || a.AttackingPlayer == nil || *a.AttackingPlayer != 1 {
		t.Fatalf("attacker %+v", a)
	}
	if len(a.BlockedBy) != 1 || a.BlockedBy[0] != blocker {
		t.Fatalf("blocked_by %v", a.BlockedBy)
	}
	b := v.Players[1].Battlefield[0]
	if b.Attacking || b.AttackingPlayer != nil || b.BlockedBy != nil {
		t.Fatalf("blocker carries attack state: %+v", b)
	}
	events.Apply(g, events.Event{Kind: events.EndCombatReset})
	if cv := Project(g, flatChars{g}, 0, nil).Players[0].Battlefield[0]; cv.AttackingPlayer != nil || cv.BlockedBy != nil {
		t.Fatalf("combat state survived EndCombatReset: %+v", cv)
	}
}

func TestTargetLabelIsEmptyWhenNothingDeclaresOne(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	events.Apply(g, events.Event{Kind: events.PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
	events.Apply(g, events.Event{Kind: events.TargetsChosen, Obj: id, Player: 1, Amount: 1})
	v := Project(g, flatChars{g}, 1, nil)
	if got := v.Stack[0].Targets[0].Label; got != "" {
		t.Fatalf("label %q for a creature spell with no ValidTgts", got)
	}
}
```

`flatChars` is `view_test.go`'s stand-in `Chars` (same package). If `cards.ParseBytes` does not link `Execute$` to its `SVar:` (the second test's guard fires), look at how `cards/link_test.go` links a single card and call that in `twoSeatWith`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./view/ -run 'Printing|Kind|TargetLabel' -count=1`
Expected: FAIL — `cv.Printing undefined`, `tg.Label undefined`.

- [ ] **Step 3: Implement**

In `view/view.go`:

```go
// Printing is the identity a client resolves an image by: the exact face
// name today. Set and Number stay empty until a printing table exists
// (roadmap open question 1); the fields are here so the wire shape does
// not change when it does.
type Printing struct {
	Name   string `json:"name"`
	Set    string `json:"set,omitempty"`
	Number string `json:"number,omitempty"`
}
```

Add to `CardView` after `Types`:

```go
	// ManaCost is the printed cost in Forge's notation ("1 W", "R", "X G").
	// Hand lists render it as symbols.
	ManaCost string `json:"mana_cost,omitempty"`
	// Printing is what an image lookup keys on; Token ("#12") tells two
	// copies of one card apart in the stack, the log and an arrow.
	Printing Printing `json:"printing"`
	Token    string   `json:"token"`
	// AttackingPlayer is the seat this creature is attacking while
	// Attacking is true, nil otherwise; BlockedBy lists the creatures
	// blocking it. Both exist for the arrow overlay (PL-17) and come
	// straight from the object's combat fields, which EndCombatReset clears.
	AttackingPlayer *state.PlayerID `json:"attacking_player,omitempty"`
	BlockedBy       []state.ObjID   `json:"blocked_by,omitempty"`
```

In `cardView`, inside `if f := o.Face(); f != nil {` add `cv.ManaCost = f.ManaCost` and `cv.Printing = Printing{Name: f.Name}`; before it, `cv.Token = "#" + strconv.FormatUint(uint64(id), 10)` (import `strconv`); after it:

```go
	if o.IsAttacking {
		p := o.Attacking
		cv.AttackingPlayer = &p
	}
	if len(o.BlockedBy) > 0 {
		cv.BlockedBy = append([]state.ObjID(nil), o.BlockedBy...)
	}
```

Change `StackView`'s doc to "Kind is "spell", "trigger" (an object minted by a TriggerPush) or "ability" (any other ability object)." Add to `TargetView`:

```go
	// Label is what the object was allowed to target, in the card's own
	// words: its TgtPrompt$ when it has one ("Select any target"), else its
	// ValidTgts$ ("Creature"). Empty when the object declares neither.
	Label string `json:"label,omitempty"`
```

Refactor the trigger lookup out of `abilityText` so `stackViews` can use it:

```go
// triggerLine finds the T: line an ability object was minted from: the
// trigger on its source's active face whose Effect is exactly o.Ability
// (events/apply.go's TriggerPush case sets it so). ok is false for an
// ability object that is not a trigger, or whose source has changed face.
func triggerLine(g *state.Game, o *state.Object) (cards.Trigger, bool) {
	src := g.Obj(o.Source)
	if src == nil {
		return cards.Trigger{}, false
	}
	f := src.Face()
	if f == nil {
		return cards.Trigger{}, false
	}
	for _, t := range f.Triggers {
		if t.Effect == o.Ability {
			return t, true
		}
	}
	return cards.Trigger{}, false
}
```

`abilityText` becomes: `if t, ok := triggerLine(g, o); ok { if d := t.Params["TriggerDescription"]; d != "" { return d } }` followed by its existing SpellDescription/StackDescription fallback. In `stackViews`' ability branch compute `kind := "ability"; if _, ok := triggerLine(g, o); ok { kind = "trigger" }` and use it. Both branches pass a label into `targetViews`:

```go
// targetLabel is the object's own description of what it targets, from the
// first SA in its chain (the SA itself, then SubAbility$ links) that
// declares TgtPrompt$, else the first that declares ValidTgts$.
func targetLabel(o *state.Object) string {
	var sa *cards.SA
	if o.Ability != nil {
		sa = o.Ability
	} else if f := o.Face(); f != nil {
		sa = f.SpellAbility()
	}
	valid := ""
	for s := sa; s != nil; s = s.Sub {
		if p := s.Params["TgtPrompt"]; p != "" {
			return p
		}
		if valid == "" {
			valid = s.Params["ValidTgts"]
		}
	}
	return valid
}
```

and `targetViews(targets []state.Target, label string)` sets `Label: label` on each. Update the one other `targetViews` caller if any (`grep -n targetViews view/`).

- [ ] **Step 4: Run the tests**

Run: `go test ./view/ -count=1 && go test -race -count=1 ./view/`
Expected: PASS.

- [ ] **Step 5: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./... && make sim | grep -c 'replay OK'`
Expected: clean, `20`.

```bash
git add view/view.go view/identity_test.go
git commit -m "feat(view): printing identity, object token, mana cost, trigger kind, target labels

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 5: `view.Describe` — one deterministic line per event

**Files:**
- Create: `view/describe.go`, `view/describe_test.go`

**Interfaces:**
- Consumes: `events.Event`, `state.Game`, `state.Zone.String`, `state.Step.String`.
- Produces: `func Describe(g *state.Game, ev events.Event) string`. Call it with the game as of the last event of the batch the event belongs to (the same convention as `RedactEvents`). Empty string only for `ClockTick`. Task 10 uses it for `widget.last` and `event.line`; Task 11 for `Events(...)`.

- [ ] **Step 1: Write the failing tests**

Create `view/describe_test.go`:

```go
package view

import (
	"context"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/state"
)

func describeFixture(t *testing.T) (*state.Game, state.ObjID, state.ObjID) {
	t.Helper()
	bear, _ := cards.ParseBytes("b.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	bolt, _ := cards.ParseBytes("l.txt", []byte(boltSrc))
	g := state.NewGame([]string{"Ann", "Bob"})
	b := g.AddObject(bear, 0)
	l := g.AddObject(bolt, 1)
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{b.ID})
	b.Zone = state.ZBattlefield
	g.SetZone(state.ZHand, 1, []state.ObjID{l.ID})
	l.Zone = state.ZHand
	g.Players[0].Life = 17
	return g, b.ID, l.ID
}

func TestDescribeTemplates(t *testing.T) {
	g, bear, bolt := describeFixture(t)
	cases := []struct {
		name string
		ev   events.Event
		want string
	}{
		{"game start", events.Event{Kind: events.GameStart, Amount: 4}, "Game starts with 4 players"},
		{"shuffle", events.Event{Kind: events.Shuffle, Player: 1}, "Bob shuffles their library"},
		{"move", events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard}, "Bear #1 moves from battlefield to graveyard"},
		{"hidden draw", events.Event{Kind: events.Draw, Player: 0}, "Ann draws a card"},
		{"visible draw", events.Event{Kind: events.Draw, Player: 1, Obj: bolt}, "Bob draws Bolt #2"},
		{"life gain", events.Event{Kind: events.LifeChange, Player: 0, Amount: 3}, "Ann gains 3 life (17)"},
		{"life loss", events.Event{Kind: events.LifeChange, Player: 0, Amount: -2}, "Ann loses 2 life (17)"},
		{"damage to creature", events.Event{Kind: events.Damage, Obj: bear, Amount: 2}, "Bear #1 takes 2 damage"},
		{"damage to player", events.Event{Kind: events.Damage, Player: 1, Amount: 3}, "Bob takes 3 damage"},
		{"tap", events.Event{Kind: events.Tap, Obj: bear}, "Bear #1 taps"},
		{"untap", events.Event{Kind: events.Untap, Obj: bear}, "Bear #1 untaps"},
		{"step", events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers}, "Step: declare-attackers"},
		{"turn", events.Event{Kind: events.TurnChange, Player: 1, Amount: 7}, "Turn 7: Bob"},
		{"priority", events.Event{Kind: events.Priority, Player: 0}, "Ann has priority"},
		{"cast", events.Event{Kind: events.PutOnStack, Player: 1, Obj: bolt}, "Bob casts Bolt #2"},
		{"resolve", events.Event{Kind: events.Resolve, Obj: bolt}, "Bolt #2 resolves"},
		{"mana add", events.Event{Kind: events.ManaAdd, Player: 0, Counter: "G", Amount: 2}, "Ann adds {G}{G}"},
		{"mana spend", events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: -1}, "Ann spends {R}"},
		{"mana clear", events.Event{Kind: events.ManaClear, Player: 0}, "Ann's mana pool empties"},
		{"counter add", events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 2}, "Bear #1 gets 2 P1P1 counters"},
		{"counter remove", events.Event{Kind: events.CounterChange, Obj: bear, Counter: "M1M1", Amount: -1}, "Bear #1 loses 1 M1M1 counter"},
		{"attackers", events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}}, "Bear #1 attacks Bob"},
		{"no attackers", events.Event{Kind: events.DeclareAttackers, Player: 1}, "No attackers"},
		{"blockers", events.Event{Kind: events.DeclareBlockers, Pairs: [][2]state.ObjID{{bear, bolt}}}, "Bolt #2 blocks Bear #1"},
		{"no blockers", events.Event{Kind: events.DeclareBlockers}, "No blocks"},
		{"player lost", events.Event{Kind: events.PlayerLost, Player: 0}, "Ann loses the game"},
		{"win", events.Event{Kind: events.GameOver, Player: 1}, "Bob wins the game"},
		{"draw game", events.Event{Kind: events.GameOver, Amount: 1}, "The game is a draw"},
		{"ask", events.Event{Kind: events.DecisionAsk, Player: 0, Text: "priority"}, "Ann is asked: priority"},
		{"answer", events.Event{Kind: events.DecisionMade, Player: 0, Text: "priority:[2]"}, "Ann answers priority:[2]"},
		{"note", events.Event{Kind: events.Note, Text: "Bob reveals Bolt"}, "Bob reveals Bolt"},
		{"redacted note", events.Event{Kind: events.Note, Secret: true, Player: 1}, "Bob looks at hidden cards"},
		{"land", events.Event{Kind: events.LandPlayed, Player: 0}, "Ann plays a land"},
		{"target player", events.Event{Kind: events.TargetsChosen, Obj: bolt, Player: 0, Amount: 1}, "Bolt #2 targets Ann"},
		{"target objects", events.Event{Kind: events.TargetsChosen, Obj: bolt, IDs: []state.ObjID{bear}}, "Bolt #2 targets Bear #1"},
		{"flip", events.Event{Kind: events.FlipFace, Obj: bear, Amount: 1}, "Bear #1 turns to face 1"},
		{"clock", events.Event{Kind: events.ClockTick}, ""},
		{"trigger", events.Event{Kind: events.TriggerPush, Player: 0, Obj: bear}, "Bear #1 triggers"},
		{"end combat", events.Event{Kind: events.EndCombatReset}, "Combat ends"},
		{"unknown kind", events.Event{Kind: 250}, "unknown event"},
		{"unknown seat", events.Event{Kind: events.Priority, Player: 9}, "seat 9 has priority"},
		{"unknown object", events.Event{Kind: events.Tap, Obj: 77}, "#77 taps"},
	}
	for _, tc := range cases {
		if got := Describe(g, tc.ev); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDescribeCoversEveryKind(t *testing.T) {
	g, bear, _ := describeFixture(t)
	for k := events.GameStart; k <= events.EndCombatReset; k++ {
		ev := events.Event{Kind: k, Player: 0, Obj: bear, Amount: 1, IDs: []state.ObjID{bear}, Pairs: [][2]state.ObjID{{bear, bear}}}
		got := Describe(g, ev)
		if k == events.ClockTick {
			if got != "" {
				t.Errorf("ClockTick should describe as empty, got %q", got)
			}
			continue
		}
		if got == "" || got == "unknown event" {
			t.Errorf("kind %s (%d) has no description", k, k)
		}
	}
}

func TestDescribeNeverPanics(t *testing.T) {
	g, _, _ := describeFixture(t)
	for _, gg := range []*state.Game{nil, g, state.NewGame(nil)} {
		for k := events.Kind(0); k < 40; k++ {
			ev := events.Event{Kind: k, Player: 250, Obj: 1 << 30, Amount: -7, Step: 99, From: 99, To: 99,
				IDs: []state.ObjID{0, 1 << 30}, Pairs: [][2]state.ObjID{{0, 1 << 30}}}
			_ = Describe(gg, ev)
		}
	}
}

func TestDescribeIsIdenticalAcrossTwoRunsOfTheSameMatch(t *testing.T) {
	run := func() []string {
		names, decks := testutil.SampleDecks(t, 4)
		e := rules.New(rules.Config{Seed: 21, Names: names, Decks: decks})
		e.Advance()
		b := seat.NewBot(21)
		var lines []string
		describeFrom := func(from int) {
			for _, ev := range e.L.Events[from:] {
				lines = append(lines, Describe(e.G, ev))
			}
		}
		describeFrom(0)
		for i := 0; i < 300 && !e.G.Over && e.Pending() != nil; i++ {
			d := e.Pending()
			from := len(e.L.Events)
			in, _ := b.Decide(context.Background(), Project(e.G, e, d.Player, d), *d)
			if err := e.Submit(in); err != nil {
				t.Fatal(err)
			}
			describeFrom(from)
		}
		return lines
	}
	a, b := run(), run()
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Fatal("Describe produced different lines for two runs of the same match")
	}
	if len(a) < 500 {
		t.Fatalf("fixture produced only %d lines", len(a))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./view/ -run Describe -count=1`
Expected: FAIL — `Describe undefined`.

- [ ] **Step 3: Implement `view/describe.go`**

```go
package view

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Describe renders one event as one line of rules transcript, the same
// line on every replay: it reads names from g and fields from ev and
// nothing else — no clock, no map iteration, no engine. Call it with the
// game as of the last event in ev's batch, the RedactEvents convention, so
// a redacted Obj (0) reads as "a card" and a visible one by name. The
// client never composes rules text; this is where the words come from.
//
// ClockTick describes as "" (the client hides empty lines); an unknown
// Kind as "unknown event" rather than a panic.
func Describe(g *state.Game, ev events.Event) string {
	switch ev.Kind {
	case events.GameStart:
		return "Game starts with " + itoa(int64(ev.Amount)) + " players"
	case events.Shuffle:
		return player(g, ev.Player) + " shuffles their library"
	case events.MoveZone:
		return obj(g, ev.Obj) + " moves from " + zone(ev.From) + " to " + zone(ev.To)
	case events.Draw:
		if ev.Obj == 0 {
			return player(g, ev.Player) + " draws a card"
		}
		return player(g, ev.Player) + " draws " + obj(g, ev.Obj)
	case events.LifeChange:
		verb, n := "gains", ev.Amount
		if n < 0 {
			verb, n = "loses", -n
		}
		return player(g, ev.Player) + " " + verb + " " + itoa(int64(n)) + " life (" + life(g, ev.Player) + ")"
	case events.Damage:
		if g != nil && g.Obj(ev.Obj) != nil {
			return obj(g, ev.Obj) + " takes " + itoa(int64(ev.Amount)) + " damage"
		}
		return player(g, ev.Player) + " takes " + itoa(int64(ev.Amount)) + " damage"
	case events.Tap:
		return obj(g, ev.Obj) + " taps"
	case events.Untap:
		return obj(g, ev.Obj) + " untaps"
	case events.StepChange:
		return "Step: " + ev.Step.String()
	case events.TurnChange:
		return "Turn " + itoa(int64(ev.Amount)) + ": " + player(g, ev.Player)
	case events.Priority:
		return player(g, ev.Player) + " has priority"
	case events.PutOnStack:
		return player(g, ev.Player) + " casts " + obj(g, ev.Obj)
	case events.Resolve:
		return obj(g, ev.Obj) + " resolves"
	case events.ManaAdd:
		if ev.Amount < 0 {
			return player(g, ev.Player) + " spends " + mana(ev.Counter, -ev.Amount)
		}
		return player(g, ev.Player) + " adds " + mana(ev.Counter, ev.Amount)
	case events.ManaClear:
		return player(g, ev.Player) + "'s mana pool empties"
	case events.CounterChange:
		verb, n := "gets", ev.Amount
		if n < 0 {
			verb, n = "loses", -n
		}
		s := obj(g, ev.Obj) + " " + verb + " " + itoa(int64(n)) + " " + ev.Counter + " counter"
		if n != 1 {
			s += "s"
		}
		return s
	case events.DeclareAttackers:
		if len(ev.IDs) == 0 {
			return "No attackers"
		}
		return objs(g, ev.IDs) + " " + plural(len(ev.IDs), "attacks", "attack") + " " + player(g, ev.Player)
	case events.DeclareBlockers:
		if len(ev.Pairs) == 0 {
			return "No blocks"
		}
		parts := make([]string, 0, len(ev.Pairs))
		for _, p := range ev.Pairs {
			parts = append(parts, obj(g, p[1])+" blocks "+obj(g, p[0]))
		}
		return strings.Join(parts, "; ")
	case events.PlayerLost:
		return player(g, ev.Player) + " loses the game"
	case events.GameOver:
		if ev.Amount == 1 {
			return "The game is a draw"
		}
		return player(g, ev.Player) + " wins the game"
	case events.DecisionAsk:
		return player(g, ev.Player) + " is asked: " + ev.Text
	case events.DecisionMade:
		return player(g, ev.Player) + " answers " + ev.Text
	case events.Note:
		if ev.Text == "" && ev.Secret {
			return player(g, ev.Player) + " looks at hidden cards"
		}
		return ev.Text
	case events.LandPlayed:
		return player(g, ev.Player) + " plays a land"
	case events.TargetsChosen:
		if ev.Amount == 1 {
			return obj(g, ev.Obj) + " targets " + player(g, ev.Player)
		}
		return obj(g, ev.Obj) + " targets " + objs(g, ev.IDs)
	case events.FlipFace:
		return obj(g, ev.Obj) + " turns to face " + itoa(int64(ev.Amount))
	case events.ClockTick:
		return ""
	case events.TriggerPush:
		return obj(g, ev.Obj) + " triggers"
	case events.EndCombatReset:
		return "Combat ends"
	}
	return "unknown event"
}

// obj names an object as "Name #id", "an ability #id" for a faceless
// stack object, "a card" for the redacted id 0, and "#id" for an id the
// game cannot resolve (nil g, stale or tampered data).
func obj(g *state.Game, id state.ObjID) string {
	if id == 0 {
		return "a card"
	}
	tag := "#" + strconv.FormatUint(uint64(id), 10)
	if g == nil {
		return tag
	}
	o := g.Obj(id)
	if o == nil {
		return tag
	}
	if f := o.Face(); f != nil && f.Name != "" {
		return f.Name + " " + tag
	}
	return "an ability " + tag
}

func objs(g *state.Game, ids []state.ObjID) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, obj(g, id))
	}
	return strings.Join(parts, ", ")
}

// player is the seat's name, or "seat N" when g cannot resolve it.
func player(g *state.Game, p state.PlayerID) string {
	if g != nil && int(p) < len(g.Players) && g.Players[p].Name != "" {
		return g.Players[p].Name
	}
	return "seat " + strconv.Itoa(int(p))
}

// life is the seat's life total as of g, or "?" when unresolvable.
func life(g *state.Game, p state.PlayerID) string {
	if g != nil && int(p) < len(g.Players) {
		return itoa(int64(g.Players[p].Life))
	}
	return "?"
}

// zone is the zone's name, total over out-of-range values.
func zone(z state.Zone) string {
	if !z.Valid() {
		return "nowhere"
	}
	return z.String()
}

// mana renders n symbols of one colour: "{G}{G}". An empty symbol is
// colourless.
func mana(sym string, n int32) string {
	if sym == "" {
		sym = "C"
	}
	if n <= 0 {
		return ""
	}
	if n > 20 {
		return itoa(int64(n)) + " {" + sym + "}"
	}
	return strings.Repeat("{"+sym+"}", int(n))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
```

- [ ] **Step 4: Run the tests**

Run: `go test ./view/ -count=1 && go test -race -count=1 ./view/`
Expected: PASS. If a template test fails on wording, change the *test* only when the implementation's wording is better English; the two must agree exactly — these strings are what the client shows.

- [ ] **Step 5: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./... && make sim | grep -c 'replay OK'`
Expected: clean, `20`.

```bash
git add view/describe.go view/describe_test.go
git commit -m "feat(view): Describe renders one deterministic transcript line per event

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 6: Architecture constraints as a test

**Files:**
- Create: `internal/archtest/arch_test.go`, `internal/archtest/doc.go`

**Interfaces:**
- Consumes: `go list` (via `os/exec`, the same way `internal/testutil` shells out to `git`).
- Produces: a test package that fails the build on an import-graph violation. Task 24 extends it with the client-vocabulary check.

- [ ] **Step 1: Write the test (it passes today; it exists to fail later)**

Create `internal/archtest/doc.go`:

```go
// Package archtest holds no code, only tests that walk the module's import
// graph and fail when a dependency-order or determinism constraint from the
// engine spec is broken: time outside the host, effects importing rules,
// the wire types importing the engine, the host importing test fixtures.
package archtest
```

Create `internal/archtest/arch_test.go`:

```go
package archtest

import (
	"os/exec"
	"strings"
	"testing"
)

const module = "github.com/adams-shaun/gorge"

type pkg struct {
	path    string
	imports map[string]bool // direct, non-test
	deps    map[string]bool // transitive, non-test
}

// packages lists every package in the module with its direct and transitive
// non-test imports. Test files are excluded on purpose: tests may import
// anything (view's tests import rules; cards' tests import time).
func packages(t *testing.T) map[string]pkg {
	t.Helper()
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}}|{{join .Imports " "}}|{{join .Deps " "}}`, module+"/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	pkgs := map[string]pkg{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			t.Fatalf("unexpected go list line %q", line)
		}
		p := pkg{path: parts[0], imports: set(parts[1]), deps: set(parts[2])}
		pkgs[p.path] = p
	}
	if len(pkgs) < 10 {
		t.Fatalf("go list found only %d packages", len(pkgs))
	}
	return pkgs
}

func set(s string) map[string]bool {
	m := map[string]bool{}
	for _, f := range strings.Fields(s) {
		m[f] = true
	}
	return m
}

// TestTimeIsImportedOnlyByTheHost is spec D16: the host's injected sleep,
// the SSE writer's ticker/keep-alive and gorged's shutdown timeout are the
// only clocks in the system. Every other package must be a pure function
// of its inputs.
func TestTimeIsImportedOnlyByTheHost(t *testing.T) {
	allowed := map[string]bool{
		module + "/host":         true,
		module + "/host/httpapi": true,
		module + "/cmd/gorged":   true,
	}
	for path, p := range packages(t) {
		if p.imports["time"] && !allowed[path] {
			t.Errorf("%s imports time; only host, host/httpapi and cmd/gorged may", path)
		}
	}
}

// TestDependencyOrderHolds pins the arrows that must never appear, direct or
// transitive.
func TestDependencyOrderHolds(t *testing.T) {
	pkgs := packages(t)
	forbidden := []struct{ from, to string }{
		{module + "/effects", module + "/rules"},
		{module + "/view", module + "/rules"},
		{module + "/protocol", module + "/rules"},
		{module + "/host", module + "/internal/testutil"},
		{module + "/host/httpapi", module + "/internal/testutil"},
		{module + "/cmd/gorged", module + "/internal/testutil"},
		{module + "/cards", module + "/state"},
		{module + "/deck", module + "/rules"},
	}
	for _, f := range forbidden {
		p, ok := pkgs[f.from]
		if !ok {
			continue // not built yet; the constraint binds once it is
		}
		if p.deps[f.to] {
			t.Errorf("%s depends on %s (transitively); the dependency order forbids it", f.from, f.to)
		}
	}
}

// TestNoLegacyMathRand: math/rand/v2 with an explicit seeded source is the
// only randomness (rules/rng.go, seat/bot.go). The v1 package's global
// functions are exactly the ambient randomness the engine spec forbids.
func TestNoLegacyMathRand(t *testing.T) {
	for path, p := range packages(t) {
		if p.imports["math/rand"] {
			t.Errorf("%s imports math/rand; use math/rand/v2 with a seeded source", path)
		}
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/archtest/ -count=1 -v`
Expected: PASS (three tests). Then prove it bites: temporarily add `import _ "time"` to `view/describe.go`, run again — expected FAIL naming `view`; revert.

- [ ] **Step 3: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./...`

```bash
git add internal/archtest/
git commit -m "test(arch): pin the time, dependency-order and math/rand constraints

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

## Phase 1 — the wire

### Task 7: `protocol` — envelope, frame bodies, wire event, goldens

**Files:**
- Create: `protocol/protocol.go`, `protocol/event.go`, `protocol/protocol_test.go`, `protocol/testdata/*.json`

**Interfaces:**
- Consumes: `view.View`, `events.Event`, `state.Zone.String`, `state.Step.String`, `events.Kind.String`.
- Produces (every name below is used verbatim by Tasks 8–23):

```go
const Version = 1
type FrameType string
const (
	THello FrameType = "hello"; TWidget = "widget"; TMatchStart = "match_start"; TSnapshot = "snapshot"
	TEvent = "event"; TDecision = "decision"; TMatchEnd = "match_end"; TTableHalted = "table_halted"
	TOverflow = "overflow"; TError = "error"
)
const (ModeOverview = "overview"; ModeFocus = "focus"; TableAll = "*")
const (TableIdle = "idle"; TableLive = "live"; TableCooldown = "cooldown"; TableHalted = "halted")
const (MatchLive = "live"; MatchFinished = "finished"; MatchAborted = "aborted"; MatchCrashed = "crashed")
type Frame struct { V int; T FrameType; ID uint64; Table string; Match int; Seq uint64; Body json.RawMessage }
func NewFrame(t FrameType, table string, match int, seq uint64, body any) (Frame, error)
func (f Frame) Decode(into any) error
type Hello struct { Session string; Tables []TableInfo }
type TableInfo struct { ID, Name string; Seats int; Spectator string; State string; Match int; Perpetual bool }
type Widget struct { Turn int32; Step, Phase string; Active, Priority uint8; Life []int32; Lost []bool; StackDepth int; Last string; State string }
type SeatInfo struct { Name, Deck, Colour string }
type MatchStart struct { Seats []SeatInfo; Seed uint64; Spectator string }
type Snapshot struct { View view.View; TurnStarts []uint64; Head uint64 }
type Event struct { Seq uint64; Kind string; Player uint8; Obj uint32; From, To string; Amount int32; Step, Counter, Text string; IDs []uint32; Pairs [][2]uint32; Secret bool }
func EventFrom(e events.Event) Event
type EventBody struct { Event Event; Line string }
type DecisionBody struct { Player uint8; Kind, Prompt string }
type MatchEnd struct { Result string; Winner *uint8; Head string }
type TableHaltedBody struct { Reason string }
type Overflow struct { Dropped int }
type ErrorBody struct { Code, Message string; Head uint64 /* json head,omitempty: the last valid seq on a 409 */ }
type MatchInfo struct { Table string; Match int; Seed uint64; Seats []SeatInfo; State, Result string; Winner *uint8; Head string; Events int; Turns int32 }
type Subscribe struct { Session, Table, Mode string }
type Unsubscribe struct { Session, Table string }
var SeatColours = [...]string{"#e5484d", "#3b82f6", "#22c55e", "#eab308", "#a855f7", "#f97316", "#14b8a6", "#ec4899"}
```

- [ ] **Step 1: Write the failing tests**

Create `protocol/protocol_test.go`:

```go
package protocol

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

var update = flag.Bool("update", false, "rewrite protocol/testdata goldens")

func seat(n uint8) *uint8 { return &n }

// fixtures is one frame per type with every field populated, so a golden
// pins the whole wire shape. Change a golden only with a protocol change
// (and a Version bump when it is not additive).
func fixtures(t *testing.T) []Frame {
	t.Helper()
	mk := func(ft FrameType, seq uint64, body any) Frame {
		f, err := NewFrame(ft, "t1", 7, seq, body)
		if err != nil {
			t.Fatal(err)
		}
		f.ID = 4182
		return f
	}
	v := view.View{Viewer: view.NoSeat, Visibility: "omniscient", Turn: 3, Step: "main1", Phase: "main1",
		Players: []view.PlayerView{{ID: 0, Name: "mono-red-goblins", Life: 20, Hand: []view.CardView{}, Battlefield: []view.CardView{
			{ID: 12, Name: "Goblin Guide", Types: "Creature Goblin Scout", ManaCost: "R", Power: 2, Toughness: 2,
				Printing: view.Printing{Name: "Goblin Guide"}, Token: "#12", Keywords: []string{"Haste"}}},
			Graveyard: []view.CardView{}, Exile: []view.CardView{}, Pool: map[string]int32{"R": 1}}},
		Stack: []view.StackView{{ID: 40, Kind: "trigger", Name: "Watcher", Text: "When CARDNAME enters, you gain 1 life.", Controller: 1, Source: 30,
			Targets: []view.TargetView{{Player: 0, IsPlayer: true, Label: "Select any target"}}}},
		Pending: []view.PendingView{{Source: 30, Controller: 1, Label: "Watcher: gain 1 life", Optional: true, Decider: seat(1)}}}
	return []Frame{
		mk(THello, 0, Hello{Session: "s3", Tables: []TableInfo{{ID: "t1", Name: "Table 1", Seats: 4, Spectator: "omniscient", State: TableLive, Match: 7, Perpetual: true}}}),
		mk(TWidget, 9130, Widget{Turn: 3, Step: "main1", Phase: "main1", Active: 0, Priority: 2, Life: []int32{20, 17, 12, 20}, Lost: []bool{false, false, false, false}, StackDepth: 1, Last: "Bob casts Bolt #2", State: MatchLive}),
		mk(TMatchStart, 0, MatchStart{Seats: []SeatInfo{{Name: "mono-red-goblins", Deck: "mono-red-goblins", Colour: SeatColours[0]}}, Seed: 12345, Spectator: "omniscient"}),
		mk(TSnapshot, 9130, Snapshot{View: v, TurnStarts: []uint64{0, 402, 1180}, Head: 9130}),
		mk(TEvent, 9131, EventBody{Event: EventFrom(events.Event{Seq: 9131, Kind: events.MoveZone, Player: 1, Obj: 12, From: state.ZHand, To: state.ZBattlefield}), Line: "Goblin Guide #12 moves from hand to battlefield"}),
		mk(TDecision, 9131, DecisionBody{Player: 2, Kind: "priority", Prompt: "You have priority."}),
		mk(TMatchEnd, 9500, MatchEnd{Result: "win", Winner: seat(2), Head: "6c8f9e4512366476"}),
		mk(TTableHalted, 9500, TableHaltedBody{Reason: "intent rejected: choice 3 out of range (2 options)"}),
		mk(TOverflow, 0, Overflow{Dropped: 17}),
		mk(TError, 0, ErrorBody{Code: "unknown_table", Message: "no table t9"}),
	}
}

func TestGoldens(t *testing.T) {
	for _, f := range fixtures(t) {
		got, err := json.MarshalIndent(f, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, '\n')
		path := filepath.Join("testdata", string(f.T)+".json")
		if *update {
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v (run go test ./protocol/ -update to create it)", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from golden:\n%s\nwant:\n%s", f.T, got, want)
		}
	}
}

func TestFramesRoundTrip(t *testing.T) {
	for _, f := range fixtures(t) {
		raw, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		var back Frame
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatal(err)
		}
		if back.V != Version || back.T != f.T || back.ID != f.ID || back.Table != f.Table || back.Match != f.Match || back.Seq != f.Seq {
			t.Fatalf("%s: envelope changed in transit: %+v", f.T, back)
		}
		if !bytes.Equal(bytes.TrimSpace(back.Body), bytes.TrimSpace(f.Body)) {
			t.Fatalf("%s: body changed in transit", f.T)
		}
	}
}

func TestDecodeIntoTypedBody(t *testing.T) {
	f, err := NewFrame(TDecision, "t1", 1, 5, DecisionBody{Player: 3, Kind: "target", Prompt: "Pick"})
	if err != nil {
		t.Fatal(err)
	}
	var d DecisionBody
	if err := f.Decode(&d); err != nil || d.Player != 3 || d.Kind != "target" || d.Prompt != "Pick" {
		t.Fatalf("decoded %+v, %v", d, err)
	}
}

func TestEventFromNamesKindsZonesAndSteps(t *testing.T) {
	mv := EventFrom(events.Event{Seq: 1, Kind: events.MoveZone, Player: 2, Obj: 9, From: state.ZLibrary, To: state.ZHand, Secret: true})
	if mv.Kind != "move_zone" || mv.From != "library" || mv.To != "hand" || !mv.Secret || mv.Player != 2 || mv.Obj != 9 {
		t.Fatalf("%+v", mv)
	}
	st := EventFrom(events.Event{Kind: events.StepChange, Step: state.StepMain2})
	if st.Kind != "step" || st.Step != "main2" || st.From != "" || st.To != "" {
		t.Fatalf("%+v", st)
	}
	// A kind that carries no zone/step must not print the zero value's name.
	tp := EventFrom(events.Event{Kind: events.Tap, Obj: 4})
	if tp.From != "" || tp.To != "" || tp.Step != "" {
		t.Fatalf("Tap leaked zone/step zero values: %+v", tp)
	}
	bl := EventFrom(events.Event{Kind: events.DeclareBlockers, Pairs: [][2]state.ObjID{{3, 4}}})
	if len(bl.Pairs) != 1 || bl.Pairs[0] != [2]uint32{3, 4} {
		t.Fatalf("%+v", bl)
	}
	if EventFrom(events.Event{Kind: 250}).Kind != "unknown" {
		t.Fatal("unknown kind not named unknown")
	}
}

func TestSeatColoursCoverEightSeats(t *testing.T) {
	if len(SeatColours) < 8 {
		t.Fatalf("%d seat colours; the engine plays up to 8 seats", len(SeatColours))
	}
	seen := map[string]bool{}
	for _, c := range SeatColours {
		if seen[c] {
			t.Fatalf("duplicate colour %s", c)
		}
		seen[c] = true
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./protocol/ -count=1`
Expected: FAIL — no non-test Go files.

- [ ] **Step 3: Implement `protocol/protocol.go`**

```go
// Package protocol is the versioned wire between a match host and its
// clients: one envelope, a closed set of frame types, and JSON bodies whose
// TypeScript twins cmd/gentypes generates. It is types only — it never
// imports rules, so a client library can depend on it without pulling in
// the engine.
package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/adams-shaun/gorge/view"
)

// Version bumps only on a breaking change to a frame body. Additive fields
// do not bump it.
const Version = 1

// FrameType is the envelope's discriminator.
type FrameType string

const (
	THello       FrameType = "hello"
	TWidget      FrameType = "widget"
	TMatchStart  FrameType = "match_start"
	TSnapshot    FrameType = "snapshot"
	TEvent       FrameType = "event"
	TDecision    FrameType = "decision"
	TMatchEnd    FrameType = "match_end"
	TTableHalted FrameType = "table_halted"
	TOverflow    FrameType = "overflow"
	TError       FrameType = "error"
)

// Subscription modes and the wildcard table.
const (
	ModeOverview = "overview"
	ModeFocus    = "focus"
	TableAll     = "*"
)

// Table states as shown in TableInfo.State / Widget.State.
const (
	TableIdle     = "idle"
	TableLive     = "live"
	TableCooldown = "cooldown"
	TableHalted   = "halted"
)

// Match states as recorded in a sidecar and shown in MatchInfo.State.
const (
	MatchLive     = "live"
	MatchFinished = "finished"
	MatchAborted  = "aborted"
	MatchCrashed  = "crashed"
)

// Frame is the envelope. ID is the session-wide frame counter and the SSE
// id; 0 means "not resumable" (widgets). Table/Match/Seq locate the body in
// a match's chain: Seq is the engine's own event sequence, the number the
// hash chain covers.
type Frame struct {
	V     int             `json:"v"`
	T     FrameType       `json:"t"`
	ID    uint64          `json:"id,omitempty"`
	Table string          `json:"table,omitempty"`
	Match int             `json:"match,omitempty"`
	Seq   uint64          `json:"seq"`
	Body  json.RawMessage `json:"body"`
}

// NewFrame marshals body into an envelope of the current Version.
func NewFrame(t FrameType, table string, match int, seq uint64, body any) (Frame, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return Frame{}, fmt.Errorf("protocol: encode %s body: %w", t, err)
	}
	return Frame{V: Version, T: t, Table: table, Match: match, Seq: seq, Body: raw}, nil
}

// Decode unmarshals the body into a typed struct.
func (f Frame) Decode(into any) error {
	if err := json.Unmarshal(f.Body, into); err != nil {
		return fmt.Errorf("protocol: decode %s body: %w", f.T, err)
	}
	return nil
}

// Hello opens every stream: the session id the client echoes in POSTs and
// the table list as of now.
type Hello struct {
	Session string      `json:"session"`
	Tables  []TableInfo `json:"tables"`
}

// TableInfo is one row of the overview.
type TableInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Seats     int    `json:"seats"`
	Spectator string `json:"spectator"`
	State     string `json:"state"`
	Match     int    `json:"match"`
	Perpetual bool   `json:"perpetual"`
}

// Widget is the overview cell: enough to draw a 2x2 life grid, a turn
// marker and a stack-depth badge, plus the last transcript line.
type Widget struct {
	Turn       int32   `json:"turn"`
	Step       string  `json:"step"`
	Phase      string  `json:"phase"`
	Active     uint8   `json:"active"`
	Priority   uint8   `json:"priority"`
	Life       []int32 `json:"life"`
	Lost       []bool  `json:"lost"`
	StackDepth int     `json:"stack_depth"`
	Last       string  `json:"last"`
	State      string  `json:"state"`
}

// SeatInfo names a seat for the identity bars; Colour is the seat colour
// the client keeps consistent from overview to focused view.
type SeatInfo struct {
	Name   string `json:"name"`
	Deck   string `json:"deck"`
	Colour string `json:"colour"`
}

// MatchStart announces match k on a subscribed table.
type MatchStart struct {
	Seats     []SeatInfo `json:"seats"`
	Seed      uint64     `json:"seed"`
	Spectator string     `json:"spectator"`
}

// Snapshot is the whole view at Head plus the turn-start seqs so far — the
// DVR's scrub ticks.
type Snapshot struct {
	View       view.View `json:"view"`
	TurnStarts []uint64  `json:"turn_starts"`
	Head       uint64    `json:"head"`
}

// EventBody is one redacted event with its transcript line.
type EventBody struct {
	Event Event  `json:"event"`
	Line  string `json:"line"`
}

// DecisionBody says who is being asked what; options come with the player
// seat (M2b), not here.
type DecisionBody struct {
	Player uint8  `json:"player"`
	Kind   string `json:"kind"`
	Prompt string `json:"prompt"`
}

// MatchEnd closes a match: Result is "win" or "draw"; Winner is null for a
// draw; Head is the chain head the .events file replays to.
type MatchEnd struct {
	Result string `json:"result"`
	Winner *uint8 `json:"winner"`
	Head   string `json:"head"`
}

// TableHaltedBody is spec D15: a crashed table stops and says why.
type TableHaltedBody struct {
	Reason string `json:"reason"`
}

// Overflow is the last frame on a stream whose session channel filled.
type Overflow struct {
	Dropped int `json:"dropped"`
}

// ErrorBody is every error reply and the error frame. Head is set only on
// a 409 "seq beyond head" reply: the last valid seq, so a client can clamp.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Head    uint64 `json:"head,omitempty"`
}

// MatchInfo is one row of a table's match list, from its sidecar.
type MatchInfo struct {
	Table  string     `json:"table"`
	Match  int        `json:"match"`
	Seed   uint64     `json:"seed"`
	Seats  []SeatInfo `json:"seats"`
	State  string     `json:"state"`
	Result string     `json:"result,omitempty"`
	Winner *uint8     `json:"winner"`
	Head   string     `json:"head,omitempty"`
	Events int        `json:"events"`
	Turns  int32      `json:"turns"`
}

// Subscribe and Unsubscribe are the POST bodies.
type Subscribe struct {
	Session string `json:"session"`
	Table   string `json:"table"`
	Mode    string `json:"mode"`
}

type Unsubscribe struct {
	Session string `json:"session"`
	Table   string `json:"table"`
}

// SeatColours are assigned by seat index and never change during a match.
var SeatColours = [...]string{"#e5484d", "#3b82f6", "#22c55e", "#eab308", "#a855f7", "#f97316", "#14b8a6", "#ec4899"}
```

Create `protocol/event.go`:

```go
package protocol

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Event is events.Event for the wire: the same fields, with Kind, zones and
// step as their names. Zone and step names are set only for the kinds that
// carry them, so a Tap never reads as "from library". events.Event's own
// JSON is untouched — it is what the .events file holds.
type Event struct {
	Seq     uint64      `json:"seq"`
	Kind    string      `json:"kind"`
	Player  uint8       `json:"player"`
	Obj     uint32      `json:"obj,omitempty"`
	From    string      `json:"from,omitempty"`
	To      string      `json:"to,omitempty"`
	Amount  int32       `json:"amount,omitempty"`
	Step    string      `json:"step,omitempty"`
	Counter string      `json:"counter,omitempty"`
	Text    string      `json:"text,omitempty"`
	IDs     []uint32    `json:"ids,omitempty"`
	Pairs   [][2]uint32 `json:"pairs,omitempty"`
	Secret  bool        `json:"secret,omitempty"`
}

// EventFrom converts one (already redacted) engine event.
func EventFrom(e events.Event) Event {
	w := Event{
		Seq: e.Seq, Kind: e.Kind.String(), Player: uint8(e.Player), Obj: uint32(e.Obj),
		Amount: e.Amount, Counter: e.Counter, Text: e.Text, Secret: e.Secret,
	}
	switch e.Kind {
	case events.MoveZone, events.Draw, events.PutOnStack:
		w.From, w.To = zoneName(e.From), zoneName(e.To)
	case events.StepChange:
		w.Step = e.Step.String()
	}
	if len(e.IDs) > 0 {
		w.IDs = make([]uint32, len(e.IDs))
		for i, id := range e.IDs {
			w.IDs[i] = uint32(id)
		}
	}
	if len(e.Pairs) > 0 {
		w.Pairs = make([][2]uint32, len(e.Pairs))
		for i, p := range e.Pairs {
			w.Pairs[i] = [2]uint32{uint32(p[0]), uint32(p[1])}
		}
	}
	return w
}

func zoneName(z state.Zone) string {
	if !z.Valid() {
		return "unknown"
	}
	return z.String()
}
```

- [ ] **Step 4: Create the goldens, then run the suite**

Run: `mkdir -p protocol/testdata && go test ./protocol/ -run TestGoldens -update -count=1 && go test ./protocol/ -count=1`
Expected: ten files appear under `protocol/testdata/`; PASS. Open `protocol/testdata/snapshot.json` and read it once end to end: every key must be the snake_case name the client will see, no `null` where the spec promises `[]`.

- [ ] **Step 5: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./...`

```bash
git add protocol/
git commit -m "feat(protocol): versioned envelope, frame bodies, wire event, goldens

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 8: `cmd/gentypes` — TypeScript types from the Go structs

**Files:**
- Create: `internal/tsgen/tsgen.go`, `internal/tsgen/tsgen_test.go`, `internal/tsgen/testdata/fixture.ts`
- Create: `cmd/gentypes/main.go`, `cmd/gentypes/main_test.go`
- Create: `web/src/protocol.ts` (generated, committed)
- Modify: `Makefile` (`gentypes` target; `lint` runs the check)

**Interfaces:**
- Consumes: the `protocol` types (Task 7), `view.View`, `decision.Decision`.
- Produces: `func tsgen.Generate(o tsgen.Options) (string, error)` with `type Options struct { Roots []reflect.Type; Unions map[string][]string; Header string }`; `web/src/protocol.ts` exporting an interface per struct and a string-literal union per entry in `Unions`. Task 17's client imports `./protocol`.

- [ ] **Step 1: Write the failing generator tests**

Create `internal/tsgen/tsgen_test.go`:

```go
package tsgen

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type inner struct {
	N    int32             `json:"n"`
	Tags map[string]int32  `json:"tags,omitempty"`
	Pair [2]uint32         `json:"pair"`
	Raw  json.RawMessage   `json:"raw"`
	Skip string            `json:"-"`
}

type outer struct {
	ID       uint64   `json:"id"`
	Name     string   `json:"name,omitempty"`
	Flag     bool     `json:"flag"`
	Inner    inner    `json:"inner"`
	Inners   []inner  `json:"inners"`
	MaybeN   *int32   `json:"maybe_n"`
	Kind     kindT    `json:"kind"`
	Bytes    []byte   `json:"bytes"`
	Any      any      `json:"any"`
	Optional *inner   `json:"optional,omitempty"`
}

type kindT string

func TestGenerateMatchesFixture(t *testing.T) {
	got, err := Generate(Options{
		Roots:  []reflect.Type{reflect.TypeOf(outer{})},
		Unions: map[string][]string{"Kind": {"a", "b"}},
		Header: "// test header\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/fixture.ts")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("generated:\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateRejectsTwoStructsWithOneName(t *testing.T) {
	_, err := Generate(Options{Roots: []reflect.Type{reflect.TypeOf(outer{}), reflect.TypeOf(struct{ X int }{})}})
	if err == nil {
		t.Fatal("anonymous struct accepted")
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	o := Options{Roots: []reflect.Type{reflect.TypeOf(outer{})}, Unions: map[string][]string{"Z": {"z"}, "A": {"a"}}}
	a, _ := Generate(o)
	b, _ := Generate(o)
	if a != b {
		t.Fatal("two runs differ")
	}
	if strings.Index(a, "export type A") > strings.Index(a, "export type Z") {
		t.Fatal("unions are not emitted in sorted order")
	}
}
```

Create `internal/tsgen/testdata/fixture.ts`:

```ts
// test header
// Code generated by cmd/gentypes; DO NOT EDIT.

export type Kind = "a" | "b";

export interface inner {
  n: number;
  tags?: Record<string, number>;
  pair: [number, number];
  raw: unknown;
}

export interface outer {
  id: number;
  name?: string;
  flag: boolean;
  inner: inner;
  inners: inner[];
  maybe_n: number | null;
  kind: string;
  bytes: string;
  any: unknown;
  optional?: inner | null;
}
```

(Struct names are emitted as-is; the protocol's exported names are capitalised, the fixture's are not — the generator does not rename.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tsgen/ -count=1`
Expected: FAIL — `Generate undefined`.

- [ ] **Step 3: Implement `internal/tsgen/tsgen.go`**

```go
// Package tsgen turns Go structs into TypeScript interfaces by reflection:
// json tags name the fields, omitempty makes them optional, pointers admit
// null, slices become arrays, string-keyed maps become Records. It exists
// so web/src/protocol.ts can never drift from package protocol — the
// committed file is regenerated by cmd/gentypes and diff-checked in lint.
package tsgen

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Options drives one generation. Roots are walked transitively; Unions are
// emitted as string-literal union types (name -> values), sorted by name.
type Options struct {
	Roots  []reflect.Type
	Unions map[string][]string
	Header string
}

var rawMessage = reflect.TypeOf(json.RawMessage{})

type gen struct {
	order []reflect.Type
	seen  map[reflect.Type]bool
	names map[string]reflect.Type
	err   error
}

// Generate renders the header, the unions, then one interface per struct in
// first-encounter order (roots first, nested types as they are met). Two
// distinct struct types with the same Name() are an error, as is an
// anonymous struct: the output needs a stable, unique name for each.
func Generate(o Options) (string, error) {
	g := &gen{seen: map[reflect.Type]bool{}, names: map[string]reflect.Type{}}
	for _, r := range o.Roots {
		g.visit(r)
		if g.err != nil {
			return "", g.err
		}
	}
	var b strings.Builder
	b.WriteString(o.Header)
	b.WriteString("// Code generated by cmd/gentypes; DO NOT EDIT.\n")
	unionNames := make([]string, 0, len(o.Unions))
	for n := range o.Unions {
		unionNames = append(unionNames, n)
	}
	sort.Strings(unionNames)
	for _, n := range unionNames {
		vals := make([]string, len(o.Unions[n]))
		for i, v := range o.Unions[n] {
			vals[i] = fmt.Sprintf("%q", v)
		}
		fmt.Fprintf(&b, "\nexport type %s = %s;\n", n, strings.Join(vals, " | "))
	}
	for _, t := range g.order {
		b.WriteString("\n")
		g.writeStruct(&b, t)
	}
	return b.String(), nil
}

// visit registers t (a struct) and every struct reachable from its fields.
func (g *gen) visit(t reflect.Type) {
	t = deref(t)
	if t.Kind() != reflect.Struct || t == rawMessage || g.seen[t] || g.err != nil {
		return
	}
	if t.Name() == "" {
		g.err = fmt.Errorf("tsgen: anonymous struct in %v cannot be named", t)
		return
	}
	if prev, ok := g.names[t.Name()]; ok && prev != t {
		g.err = fmt.Errorf("tsgen: two structs named %s: %v and %v", t.Name(), prev, t)
		return
	}
	g.seen[t] = true
	g.names[t.Name()] = t
	// Fields are visited before t is appended so nested types are declared
	// first; TypeScript does not need that, but a reader does.
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if name, _ := jsonName(f); name == "" {
			continue
		}
		g.visit(elemStruct(f.Type))
	}
	g.order = append(g.order, t)
}

func (g *gen) writeStruct(b *strings.Builder, t reflect.Type) {
	fmt.Fprintf(b, "export interface %s {\n", t.Name())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, omitempty := jsonName(f)
		if name == "" {
			continue
		}
		q := ""
		if omitempty {
			q = "?"
		}
		fmt.Fprintf(b, "  %s%s: %s;\n", name, q, tsType(f.Type))
	}
	b.WriteString("}\n")
}

// jsonName is the wire name and whether omitempty is set; "" means skip
// (json:"-" or an unexported field).
func jsonName(f reflect.StructField) (string, bool) {
	if f.PkgPath != "" {
		return "", false
	}
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	name, opts, _ := strings.Cut(tag, ",")
	if name == "" {
		name = f.Name
	}
	return name, strings.Contains(opts, "omitempty")
}

// tsType maps one Go type to TypeScript.
func tsType(t reflect.Type) string {
	switch {
	case t == rawMessage:
		return "unknown"
	case t.Kind() == reflect.Pointer:
		return tsType(t.Elem()) + " | null"
	case t.Kind() == reflect.Interface:
		return "unknown"
	case t.Kind() == reflect.Bool:
		return "boolean"
	case t.Kind() == reflect.String:
		return "string"
	case t.Kind() >= reflect.Int && t.Kind() <= reflect.Float64:
		return "number"
	case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8:
		return "string" // encoding/json base64-encodes []byte
	case t.Kind() == reflect.Slice:
		return tsType(t.Elem()) + "[]"
	case t.Kind() == reflect.Array:
		parts := make([]string, t.Len())
		for i := range parts {
			parts[i] = tsType(t.Elem())
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case t.Kind() == reflect.Map:
		return "Record<string, " + tsType(t.Elem()) + ">"
	case t.Kind() == reflect.Struct:
		return t.Name()
	}
	return "unknown"
}

func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// elemStruct digs through pointers, slices, arrays and maps to the struct a
// field ultimately holds, if any.
func elemStruct(t reflect.Type) reflect.Type {
	for {
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			t = t.Elem()
		default:
			return t
		}
	}
}
```

- [ ] **Step 4: Run generator tests**

Run: `go test ./internal/tsgen/ -count=1`
Expected: PASS. If the fixture differs only in whitespace or field order, fix the generator, not the fixture — the fixture is the contract.

- [ ] **Step 5: Write `cmd/gentypes` and its freshness test**

Create `cmd/gentypes/main.go`:

```go
// Command gentypes writes web/src/protocol.ts from package protocol's
// structs. `make gentypes` regenerates; `make lint` runs it with -check and
// fails when the committed file is stale, so the client's types can never
// drift from the server's.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/adams-shaun/gorge/internal/tsgen"
	"github.com/adams-shaun/gorge/protocol"
)

const header = "// TypeScript twins of package protocol (github.com/adams-shaun/gorge/protocol).\n" +
	"// Regenerate with `make gentypes`; `make lint` fails when this file is stale.\n"

// Render is the whole generation, shared by main and the freshness test.
func Render() (string, error) {
	roots := []reflect.Type{
		reflect.TypeOf(protocol.Frame{}), reflect.TypeOf(protocol.Hello{}), reflect.TypeOf(protocol.TableInfo{}),
		reflect.TypeOf(protocol.Widget{}), reflect.TypeOf(protocol.SeatInfo{}), reflect.TypeOf(protocol.MatchStart{}),
		reflect.TypeOf(protocol.Snapshot{}), reflect.TypeOf(protocol.Event{}), reflect.TypeOf(protocol.EventBody{}),
		reflect.TypeOf(protocol.DecisionBody{}), reflect.TypeOf(protocol.MatchEnd{}), reflect.TypeOf(protocol.TableHaltedBody{}),
		reflect.TypeOf(protocol.Overflow{}), reflect.TypeOf(protocol.ErrorBody{}), reflect.TypeOf(protocol.MatchInfo{}),
		reflect.TypeOf(protocol.Subscribe{}), reflect.TypeOf(protocol.Unsubscribe{}),
	}
	unions := map[string][]string{
		"FrameType":  {"hello", "widget", "match_start", "snapshot", "event", "decision", "match_end", "table_halted", "overflow", "error"},
		"Mode":       {protocol.ModeOverview, protocol.ModeFocus},
		"TableState": {protocol.TableIdle, protocol.TableLive, protocol.TableCooldown, protocol.TableHalted},
		"MatchState": {protocol.MatchLive, protocol.MatchFinished, protocol.MatchAborted, protocol.MatchCrashed},
		"Visibility": {"seat", "public", "omniscient"},
		"StackKind":  {"spell", "ability", "trigger"},
		"Phase":      {"beginning", "main1", "combat", "main2", "ending", ""},
	}
	return tsgen.Generate(tsgen.Options{Roots: roots, Unions: unions, Header: header})
}

func main() {
	out := flag.String("o", "web/src/protocol.ts", "output path")
	check := flag.Bool("check", false, "exit 1 if the output file is stale instead of writing it")
	flag.Parse()
	src, err := Render()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gentypes:", err)
		os.Exit(1)
	}
	if *check {
		cur, err := os.ReadFile(*out)
		if err != nil || !bytes.Equal(cur, []byte(src)) {
			fmt.Fprintf(os.Stderr, "gentypes: %s is stale; run make gentypes\n", *out)
			os.Exit(1)
		}
		return
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "gentypes:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gentypes:", err)
		os.Exit(1)
	}
}
```

The `FrameType` list is spelled out rather than reflected because `FrameType` constants are not enumerable by reflection; Task 7's fixtures pin the same ten names, and a test below cross-checks them.

Create `cmd/gentypes/main_test.go`:

```go
package main

import (
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/protocol"
)

func TestCommittedProtocolTSIsFresh(t *testing.T) {
	want, err := Render()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../web/src/protocol.ts")
	if err != nil {
		t.Fatalf("%v — run make gentypes", err)
	}
	if string(got) != want {
		t.Fatal("web/src/protocol.ts is stale — run make gentypes")
	}
}

func TestFrameTypeUnionListsEveryConstant(t *testing.T) {
	src, _ := Render()
	for _, ft := range []protocol.FrameType{protocol.THello, protocol.TWidget, protocol.TMatchStart, protocol.TSnapshot,
		protocol.TEvent, protocol.TDecision, protocol.TMatchEnd, protocol.TTableHalted, protocol.TOverflow, protocol.TError} {
		if !strings.Contains(src, `"`+string(ft)+`"`) {
			t.Errorf("FrameType union lacks %q", ft)
		}
	}
	for _, name := range []string{"View", "PlayerView", "CardView", "StackView", "TargetView", "PendingView", "Printing", "Decision", "Option"} {
		if !strings.Contains(src, "export interface "+name+" {") {
			t.Errorf("view/decision type %s missing from the generated output", name)
		}
	}
}
```

- [ ] **Step 6: Generate, wire the Makefile, run**

Add to `Makefile` (after the `report` target):

```make
.PHONY: gentypes
gentypes:
	go run ./cmd/gentypes -o web/src/protocol.ts
```

and extend `lint` with a final line `go run ./cmd/gentypes -check`. Update the `help` text with `make gentypes`.

Run: `make gentypes && go test ./cmd/gentypes/ ./internal/tsgen/ -count=1 && make lint`
Expected: `web/src/protocol.ts` written; PASS; lint clean. Read the generated file once: `View`, `CardView`, `Decision`, `Option`, `Frame` and every protocol body present; `winner: number | null`; `hand: CardView[]` (the null-vs-[] distinction is a runtime value, not a type, and `PlayerView` documents it in Go).

- [ ] **Step 7: Gates and commit**

Run: `go build ./... && go test -count=1 ./...`

```bash
git add internal/tsgen/ cmd/gentypes/ web/src/protocol.ts Makefile
git commit -m "feat(gentypes): generate web/src/protocol.ts from package protocol

make lint fails when the committed TypeScript is stale.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

## Phase 2 — the host

The `host` package is built in five tasks against unit tests only (no HTTP). Every test injects `Sleep: func(time.Duration) {}` and runs with `Perpetual: false` unless it is testing perpetual behaviour, so a whole test match takes tens of milliseconds. Tests may import `internal/testutil` for `SampleDecks`; the package itself must not (Task 6 enforces it).

### Task 9: `host` core — tables, seeds, the match loop, `Wait`, `Matches`

**Files:**
- Create: `host/doc.go`, `host/seed.go`, `host/table.go`, `host/match.go`, `host/registry.go`, `host/seed_test.go`, `host/host_test.go`

**Interfaces:**
- Consumes: `rules.New/Advance/Pending/Submit`, `seat.NewBot`, `view.Project`, `protocol.*` constants and `SeatInfo`/`TableInfo`/`MatchInfo`, `deck` (types only — the host never reads files itself).
- Produces (used verbatim by Tasks 10–16):

```go
type TableID string
type Deck struct { Name string; Cards []*cards.Card }
type TableConfig struct { ID TableID; Name string; Seats int; Decks []string; Seed uint64; Pace time.Duration; Spectator view.Visibility; Perpetual bool }
type Options struct {
	Dir      string                                   // persistence root; "" = in memory (Task 12)
	LoadDeck func(name string) (Deck, error)          // required
	Sleep    func(time.Duration)                      // required; tests pass a no-op
	Seats    func(names []string, seed uint64) []seat.Seat // nil = one seat.NewBot per seat (PL-14)
	Sync       bool          // fsync per decision (Task 12)
	Ring       int           // frames per session ring; 0 = 256 (Task 10)
	Cooldown   time.Duration // between matches on a perpetual table; 0 = none
	MaxIntents int           // per match; 0 = 400000
}
func MatchSeed(tableSeed uint64, k int) uint64
func New(o Options) (*Registry, error)
func (r *Registry) AddTable(c TableConfig) error      // registers (and persists, Task 12); does not start
func (r *Registry) Start(id TableID) error            // launches the table's goroutine
func (r *Registry) StartAll() error
func (r *Registry) Wait(id TableID)                   // blocks until the table's loop has exited (idle, halted or closed)
func (r *Registry) Done() <-chan struct{}             // closed once Close has told every table to stop (before it waits for them)
func (r *Registry) Tables() []protocol.TableInfo      // sorted by ID
func (r *Registry) Matches(id TableID) ([]protocol.MatchInfo, error) // ascending k; live match last
func (r *Registry) Close() error                      // stops every table, aborts live matches, waits
var ErrNotFound = errors.New("host: not found")
```

Match indices `k` are 1-based. A table's `state` is one of the `protocol.Table*` constants; a match's one of `protocol.Match*`.

- [ ] **Step 1: Write the failing seed test**

Create `host/seed_test.go`:

```go
package host

import "testing"

func TestMatchSeedIsAPureFunctionOfTableSeedAndIndex(t *testing.T) {
	if MatchSeed(1, 1) != MatchSeed(1, 1) {
		t.Fatal("not deterministic")
	}
	seen := map[uint64]bool{}
	for k := 1; k <= 1000; k++ {
		s := MatchSeed(42, k)
		if seen[s] {
			t.Fatalf("seed collision at k=%d", k)
		}
		seen[s] = true
	}
	if MatchSeed(1, 1) == MatchSeed(2, 1) || MatchSeed(1, 1) == MatchSeed(1, 2) {
		t.Fatal("seed does not depend on both inputs")
	}
	// Pinned so a table's history cannot silently change under a refactor.
	if got := MatchSeed(0, 1); got != 0xe220a8397b1dcdaf {
		t.Fatalf("MatchSeed(0,1) = %#x; if the formula changed on purpose, update this pin and the sidecar goldens", got)
	}
}
```

The pinned value is splitmix64's finaliser of `0 ^ (1 * 0x9E3779B97F4A7C15)`; compute it once with the implementation below, replace the literal if it differs, and never change it again.

- [ ] **Step 2: Implement `host/seed.go` and `host/doc.go`**

```go
// Package host keeps tables running: a registry of table configurations,
// one goroutine per table that plays match after match with bot seats,
// sessions that subscribe to overview widgets or a focused table's event
// stream, turn-start snapshots that answer "view at seq N", append-only
// persistence, and crash handling that halts a table rather than hiding a
// bug. It imports rules and seat but exposes neither: a client sees
// protocol frames and view.Views only. It is the first package allowed to
// import time, and the only clock it uses is the injected Sleep.
package host
```

```go
package host

// MatchSeed derives match k's engine seed from its table's seed with
// splitmix64's finaliser over tableSeed XOR k·φ, so a table's whole history
// is a pure function of its configuration (spec D14) and consecutive
// matches share nothing but the table.
func MatchSeed(tableSeed uint64, k int) uint64 {
	z := tableSeed ^ (uint64(k) * 0x9E3779B97F4A7C15)
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
```

Run: `go test ./host/ -run MatchSeed -count=1` — PASS (after fixing the pin literal if needed).

- [ ] **Step 3: Write the failing registry tests**

Create `host/host_test.go`:

```go
package host

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// sampleLoader serves testutil.SampleDecks' four decks under the names
// "a".."d", so every host test is corpus-free and deterministic.
func sampleLoader(t *testing.T) func(string) (Deck, error) {
	t.Helper()
	names, decks := testutil.SampleDecks(t, 4)
	byName := map[string][]*cards.Card{}
	for i, n := range names {
		byName[n] = decks[i]
	}
	return func(name string) (Deck, error) {
		cs, ok := byName[name]
		if !ok {
			return Deck{}, ErrNotFound
		}
		return Deck{Name: name, Cards: cs}, nil
	}
}

func testOptions(t *testing.T) Options {
	t.Helper()
	return Options{LoadDeck: sampleLoader(t), Sleep: func(time.Duration) {}}
}

func fourSeatTable(id TableID, perpetual bool) TableConfig {
	return TableConfig{ID: id, Name: "Table " + string(id), Seats: 4, Decks: []string{"a", "b", "c", "d"},
		Seed: 99, Pace: 0, Spectator: view.Omniscient, Perpetual: perpetual}
}

func TestATablePlaysOneMatchToCompletionAndGoesIdle(t *testing.T) {
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	if got := r.Tables(); len(got) != 1 || got[0].State != protocol.TableIdle || got[0].Match != 0 {
		t.Fatalf("before start: %+v", got)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	ti := r.Tables()[0]
	if ti.State != protocol.TableIdle || ti.Match != 1 || ti.Seats != 4 || ti.Spectator != "omniscient" {
		t.Fatalf("after one match: %+v", ti)
	}
	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 {
		t.Fatalf("matches %v, %v", ms, err)
	}
	m := ms[0]
	if m.State != protocol.MatchFinished || m.Match != 1 || m.Seed != MatchSeed(99, 1) || m.Head == "" || m.Events < 100 || m.Turns < 2 {
		t.Fatalf("match info %+v", m)
	}
	if m.Result != "win" && m.Result != "draw" {
		t.Fatalf("result %q", m.Result)
	}
	if (m.Result == "win") != (m.Winner != nil) {
		t.Fatalf("winner/result disagree: %+v", m)
	}
	if len(m.Seats) != 4 || m.Seats[0].Colour != protocol.SeatColours[0] || m.Seats[1].Deck != "c" {
		// k=1: seat i plays Decks[(i+1)%4] → a,b,c,d rotate to b,c,d,a.
		t.Fatalf("seats %+v", m.Seats)
	}
}

func TestTheSameConfigurationPlaysTheSameMatch(t *testing.T) {
	run := func() protocol.MatchInfo {
		r, _ := New(testOptions(t))
		defer r.Close()
		if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
			t.Fatal(err)
		}
		if err := r.Start("t1"); err != nil {
			t.Fatal(err)
		}
		r.Wait("t1")
		ms, _ := r.Matches("t1")
		return ms[0]
	}
	a, b := run(), run()
	if a.Head != b.Head || a.Events != b.Events || a.Turns != b.Turns {
		t.Fatalf("two runs differ: %+v vs %+v", a, b)
	}
}

func TestAPerpetualTableStartsTheNextMatchWithTheDerivedSeed(t *testing.T) {
	cooled := 0
	o := testOptions(t)
	o.Cooldown = time.Second
	var r *Registry
	o.Sleep = func(d time.Duration) {
		if d == time.Second {
			cooled++
			if cooled == 2 {
				// Stop after two matches. Close is asynchronous (it waits for
				// this very goroutine), so wait for its stop signal before
				// returning, or the loop could start a third match first.
				go r.Close()
				<-r.Done()
			}
		}
	}
	r, _ = New(o)
	if err := r.AddTable(fourSeatTable("t1", true)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	ms, _ := r.Matches("t1")
	if len(ms) < 2 {
		t.Fatalf("perpetual table played %d matches", len(ms))
	}
	if ms[0].Seed != MatchSeed(99, 1) || ms[1].Seed != MatchSeed(99, 2) || ms[0].Head == ms[1].Head {
		t.Fatalf("seeds/heads %+v %+v", ms[0], ms[1])
	}
	if ms[1].Seats[0].Deck != "c" { // k=2: seat 0 plays Decks[(0+2)%4]
		t.Fatalf("decks did not rotate: %+v", ms[1].Seats)
	}
}

func TestCloseAbortsALiveMatch(t *testing.T) {
	o := testOptions(t)
	var r *Registry
	n := 0
	o.Sleep = func(time.Duration) {
		n++
		if n == 50 {
			go r.Close()
			<-r.Done()
		}
	}
	r, _ = New(o)
	if err := r.AddTable(fourSeatTable("t1", true)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	ms, _ := r.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchAborted || ms[0].Result != "" {
		t.Fatalf("after Close: %+v", ms)
	}
	if r.Tables()[0].State != protocol.TableIdle {
		t.Fatalf("table state %s after Close", r.Tables()[0].State)
	}
}

func TestConfigurationIsValidated(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	bad := []TableConfig{
		{},
		{ID: "x", Seats: 0, Decks: []string{"a"}},
		{ID: "x", Seats: 9, Decks: []string{"a"}},
		{ID: "x", Seats: 2},
		{ID: "x", Seats: 2, Decks: []string{"a"}, Spectator: view.Seat},
		{ID: "x", Seats: 2, Decks: []string{"nope"}},
	}
	for i, c := range bad {
		if err := r.AddTable(c); err == nil {
			t.Errorf("config %d accepted: %+v", i, c)
		}
	}
	good := TableConfig{ID: "x", Seats: 2, Decks: []string{"a"}, Spectator: view.Public}
	if err := r.AddTable(good); err != nil {
		t.Fatal(err)
	}
	if err := r.AddTable(good); err == nil {
		t.Fatal("duplicate table id accepted")
	}
	if err := r.Start("missing"); err == nil {
		t.Fatal("Start of an unknown table succeeded")
	}
	if _, err := r.Matches("missing"); err == nil {
		t.Fatal("Matches of an unknown table succeeded")
	}
}

func TestNewRequiresLoadDeckAndSleep(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("New accepted empty Options")
	}
}
```

- [ ] **Step 4: Run to verify it fails**

Run: `go test ./host/ -count=1`
Expected: FAIL — `New undefined` etc.

- [ ] **Step 5: Implement `host/table.go`**

```go
package host

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// TableID is stable for the life of a registry directory; match indices
// under it increment from 1.
type TableID string

// Deck is a resolved deck list: the name a seat is called and the cards it
// is dealt. Options.LoadDeck produces it; the host never reads files for
// decks itself.
type Deck struct {
	Name  string
	Cards []*cards.Card
}

// TableConfig is everything a table needs; it is persisted verbatim in
// tables.json, so a table's whole history is reproducible from it.
type TableConfig struct {
	ID    TableID  `json:"id"`
	Name  string   `json:"name"`
	Seats int      `json:"seats"`
	Decks []string `json:"decks"` // deck names for Options.LoadDeck; seat i of match k plays Decks[(i+k)%len]
	Seed  uint64   `json:"seed"`
	// Pace is the sleep after every decision; 0 plays as fast as possible.
	Pace      time.Duration   `json:"pace"`
	Spectator view.Visibility `json:"spectator"` // Public or Omniscient
	Perpetual bool            `json:"perpetual"`
}

var ErrNotFound = errors.New("host: not found")

func (c TableConfig) validate(load func(string) (Deck, error)) error {
	switch {
	case c.ID == "":
		return fmt.Errorf("host: table has no id")
	case c.Seats < 1 || c.Seats > 8:
		return fmt.Errorf("host: table %s: seats %d, want 1..8", c.ID, c.Seats)
	case len(c.Decks) == 0:
		return fmt.Errorf("host: table %s: no decks", c.ID)
	case c.Spectator != view.Public && c.Spectator != view.Omniscient:
		return fmt.Errorf("host: table %s: spectator visibility must be public or omniscient", c.ID)
	}
	for _, d := range c.Decks {
		if _, err := load(d); err != nil {
			return fmt.Errorf("host: table %s: deck %q: %w", c.ID, d, err)
		}
	}
	return nil
}

// table is one registry entry and the goroutine that plays it. mu guards
// every field below cfg; the run loop and every reader take it.
type table struct {
	cfg TableConfig

	mu      sync.RWMutex
	state   string // protocol.Table*
	k       int    // index of the current or most recent match; 0 before any
	cur     *match // the live match, or nil
	history []*match
	started bool
	stop    chan struct{} // closed by Registry.Close
	done    chan struct{} // closed when the run loop exits
}

func newTable(cfg TableConfig) *table {
	return &table{cfg: cfg, state: protocol.TableIdle, stop: make(chan struct{}), done: make(chan struct{})}
}

func (t *table) info() protocol.TableInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return protocol.TableInfo{ID: string(t.cfg.ID), Name: t.cfg.Name, Seats: t.cfg.Seats,
		Spectator: t.cfg.Spectator.String(), State: t.state, Match: t.k, Perpetual: t.cfg.Perpetual}
}

func (t *table) setState(s string) {
	t.mu.Lock()
	t.state = s
	t.mu.Unlock()
}
```

- [ ] **Step 6: Implement `host/match.go`**

```go
package host

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

const defaultMaxIntents = 400000

// match is one game on a table: the engine, the intent boundaries and
// turn starts a view request needs, and the outcome. mu guards everything
// below cfg: the run loop holds it for the duration of each Submit and its
// bookkeeping; readers (ViewAt, Events, fan-out) hold it for reads only and
// never drive the engine.
type match struct {
	table *table
	k     int
	seed  uint64
	cfg   rules.Config
	seats []protocol.SeatInfo
	decks []string

	mu sync.RWMutex
	e  *rules.Engine
	// bounds[j] is len(e.L.Events) after j intents: the seq one past the
	// end of the j-th burst. bounds[0] is genesis plus the first Advance.
	bounds []uint64
	// turnStarts is the seq of every TurnChange so far — the DVR's ticks.
	turnStarts []uint64
	snaps      []snapshot // Task 11
	intents    int
	state      string // protocol.Match*
	result     string // "win", "draw" or ""
	winner     *uint8
	head       string
	reason     string // crash reason (Task 13)
}

// snapshot is a cloned engine at an intent boundary that began a turn.
type snapshot struct {
	intent int
	seq    uint64
	e      *rules.Engine
}

// newMatch resolves decks, seeds and builds the engine through genesis and
// the first Advance, so the returned match is at intent boundary 0.
func (r *Registry) newMatch(t *table, k int) (*match, error) {
	c := t.cfg
	seed := MatchSeed(c.Seed, k)
	names := make([]string, c.Seats)
	decks := make([][]*cards.Card, c.Seats)
	deckNames := make([]string, c.Seats)
	infos := make([]protocol.SeatInfo, c.Seats)
	for i := 0; i < c.Seats; i++ {
		dn := c.Decks[(i+k)%len(c.Decks)]
		d, err := r.opts.LoadDeck(dn)
		if err != nil {
			return nil, fmt.Errorf("host: table %s match %d: deck %q: %w", c.ID, k, dn, err)
		}
		if d.Name == "" {
			d.Name = dn
		}
		names[i], decks[i], deckNames[i] = d.Name, d.Cards, dn
		infos[i] = protocol.SeatInfo{Name: d.Name, Deck: dn, Colour: protocol.SeatColours[i%len(protocol.SeatColours)]}
	}
	cfg := rules.Config{Seed: seed, Names: names, Decks: decks}
	e := rules.New(cfg)
	e.Advance()
	m := &match{table: t, k: k, seed: seed, cfg: cfg, seats: infos, decks: deckNames, e: e, state: protocol.MatchLive}
	m.bounds = []uint64{uint64(len(e.L.Events))}
	m.turnStarts = turnStartsIn(e.L.Events, 0)
	return m, nil
}

// turnStartsIn lists the seq of every TurnChange in evs[from:].
func turnStartsIn(evs []events.Event, from int) []uint64 {
	var out []uint64
	for _, ev := range evs[from:] {
		if ev.Kind == events.TurnChange {
			out = append(out, ev.Seq)
		}
	}
	return out
}

// locked runs fn under the write lock and releases it even if fn panics —
// a panicking Submit must not leave the mutex held, or crash (which takes
// the lock to record the failure) would deadlock the table.
func (m *match) locked(fn func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn()
}

// afterSubmit records the burst that a successful Submit just produced.
// Called with m.mu held.
func (m *match) afterSubmit(before int) {
	m.intents++
	m.bounds = append(m.bounds, uint64(len(m.e.L.Events)))
	m.turnStarts = append(m.turnStarts, turnStartsIn(m.e.L.Events, before)...)
}

// info is the sidecar/wire summary. Called with m.mu held for reading.
func (m *match) info() protocol.MatchInfo {
	return protocol.MatchInfo{Table: string(m.table.cfg.ID), Match: m.k, Seed: m.seed, Seats: m.seats,
		State: m.state, Result: m.result, Winner: m.winner, Head: m.head,
		Events: len(m.e.L.Events), Turns: m.e.G.Turn}
}

// defaultSeats is PL-14: one bot per seat, seeded from the match seed.
func defaultSeats(names []string, seed uint64) []seat.Seat {
	out := make([]seat.Seat, len(names))
	for i := range names {
		out[i] = seat.NewBot(seed ^ uint64(i+1))
	}
	return out
}

// play drives m to completion, abort or crash on the table's goroutine and
// returns the final match state. A panic anywhere in a decision or Submit
// is a crash (spec D15), never a dead goroutine.
func (r *Registry) play(t *table, m *match) (final string) {
	defer func() {
		if p := recover(); p != nil {
			final = r.crash(t, m, fmt.Errorf("panic: %v\n%s", p, debug.Stack()))
		}
	}()
	seats := r.opts.Seats(m.cfg.Names, m.seed)
	ctx := context.Background()
	maxIntents := r.opts.MaxIntents
	if maxIntents == 0 {
		maxIntents = defaultMaxIntents
	}
	for n := 0; ; n++ {
		select {
		case <-t.stop:
			return r.abort(m)
		default:
		}
		// The loop is the only writer, so reading without the lock here
		// is safe; readers on other goroutines take RLock and see either
		// the state before or after the Lock section below.
		if m.e.G.Over {
			return r.finish(t, m)
		}
		d := m.e.Pending()
		if d == nil {
			return r.crash(t, m, fmt.Errorf("engine stalled: game not over and no decision pending"))
		}
		if n >= maxIntents {
			return r.crash(t, m, fmt.Errorf("did not terminate after %d intents (turn %d)", n, m.e.G.Turn))
		}
		v := view.Project(m.e.G, m.e, d.Player, d)
		in, err := seats[d.Player].Decide(ctx, v, *d)
		if err != nil {
			return r.crash(t, m, fmt.Errorf("seat %d: %w", d.Player, err))
		}
		var before int
		err = m.locked(func() error {
			before = len(m.e.L.Events)
			if err := m.e.Submit(in); err != nil {
				return fmt.Errorf("intent %d rejected: %w", n, err)
			}
			m.afterSubmit(before)
			if err := r.afterBurst(t, m, before); err != nil { // Tasks 11, 12
				return fmt.Errorf("persist: %w", err)
			}
			return nil
		})
		if err != nil {
			return r.crash(t, m, err)
		}
		r.fanout(t, m, before) // Task 10
		r.opts.Sleep(t.cfg.Pace)
	}
}

// finish records a natural end.
func (r *Registry) finish(t *table, m *match) string {
	m.mu.Lock()
	m.state = protocol.MatchFinished
	m.head = m.e.L.Head()
	if m.e.G.Draw {
		m.result = "draw"
	} else {
		m.result = "win"
		w := uint8(m.e.G.Winner)
		m.winner = &w
	}
	m.mu.Unlock()
	r.onMatchEnd(t, m) // Tasks 10, 12
	return protocol.MatchFinished
}

// abort records a match cut short by Close.
func (r *Registry) abort(m *match) string {
	m.mu.Lock()
	m.state = protocol.MatchAborted
	m.head = m.e.L.Head()
	m.mu.Unlock()
	r.onMatchEnd(m.table, m)
	return protocol.MatchAborted
}

// crash is spec D15's first half: the match is marked crashed with its
// reason. Task 13 adds the crash report, the table halt frame and tests.
func (r *Registry) crash(t *table, m *match, err error) string {
	m.mu.Lock()
	m.state = protocol.MatchCrashed
	m.reason = err.Error()
	m.head = m.e.L.Head()
	m.mu.Unlock()
	r.onMatchEnd(t, m)
	return protocol.MatchCrashed
}

// Hooks the later tasks fill in. They exist now so play's shape is final.
func (r *Registry) afterBurst(t *table, m *match, before int) error { return nil }
func (r *Registry) fanout(t *table, m *match, before int)           {}
func (r *Registry) onMatchEnd(t *table, m *match)                   {}
```

- [ ] **Step 7: Implement `host/registry.go`**

```go
package host

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/seat"
)

// Options configures a Registry. LoadDeck and Sleep are required so a
// caller can never forget that the host reads no files for decks and owns
// no clock of its own.
type Options struct {
	Dir        string
	LoadDeck   func(name string) (Deck, error)
	Sleep      func(time.Duration)
	Seats      func(names []string, seed uint64) []seat.Seat
	Sync       bool
	Ring       int
	Cooldown   time.Duration
	MaxIntents int
}

// Registry owns the tables and the sessions watching them.
type Registry struct {
	opts Options

	mu       sync.RWMutex
	tables   map[TableID]*table
	sessions map[string]*Session // Task 10
	nextSess int
	closed   bool
	done     chan struct{} // closed by Close once every table has been told to stop
	wg       sync.WaitGroup
}

// New validates Options and, when Dir is set, reads the registry back from
// disk (Task 12). Nothing starts running until Start/StartAll.
func New(o Options) (*Registry, error) {
	if o.LoadDeck == nil || o.Sleep == nil {
		return nil, fmt.Errorf("host: Options.LoadDeck and Options.Sleep are required")
	}
	if o.Seats == nil {
		o.Seats = defaultSeats
	}
	if o.Ring == 0 {
		o.Ring = 256
	}
	r := &Registry{opts: o, tables: map[TableID]*table{}, sessions: map[string]*Session{}, done: make(chan struct{})}
	if o.Dir != "" {
		if err := r.load(); err != nil { // Task 12
			return nil, err
		}
	}
	return r, nil
}

// AddTable registers (and persists) a table without starting it.
func (r *Registry) AddTable(c TableConfig) error {
	if err := c.validate(r.opts.LoadDeck); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return fmt.Errorf("host: registry is closed")
	}
	if _, dup := r.tables[c.ID]; dup {
		return fmt.Errorf("host: table %s already exists", c.ID)
	}
	r.tables[c.ID] = newTable(c)
	return r.save() // Task 12; a no-op in memory mode
}

// Start launches the table's goroutine; a second Start is a no-op.
func (r *Registry) Start(id TableID) error {
	r.mu.Lock()
	t, ok := r.tables[id]
	if !ok {
		r.mu.Unlock()
		return ErrNotFound
	}
	if r.closed {
		r.mu.Unlock()
		return fmt.Errorf("host: registry is closed")
	}
	if t.started {
		r.mu.Unlock()
		return nil
	}
	t.started = true
	r.wg.Add(1)
	r.mu.Unlock()
	go r.run(t)
	return nil
}

// StartAll starts every registered table, in ID order.
func (r *Registry) StartAll() error {
	for _, id := range r.ids() {
		if err := r.Start(id); err != nil {
			return err
		}
	}
	return nil
}

// Wait blocks until the table's goroutine has exited: the table is idle,
// halted or the registry was closed. An unknown or never-started table
// returns at once.
func (r *Registry) Wait(id TableID) {
	r.mu.RLock()
	t, ok := r.tables[id]
	started := ok && t.started
	r.mu.RUnlock()
	if !started {
		return
	}
	<-t.done
}

// run is the table's goroutine: match after match while perpetual, until
// a non-perpetual match ends, the registry closes, or a crash halts it.
func (r *Registry) run(t *table) {
	defer r.wg.Done()
	defer close(t.done)
	for {
		t.mu.Lock()
		k := t.k + 1
		t.mu.Unlock()
		m, err := r.newMatch(t, k)
		if err != nil {
			r.halt(t, err) // Task 13; sets state halted
			return
		}
		t.mu.Lock()
		t.k, t.cur, t.state = k, m, protocol.TableLive
		t.mu.Unlock()
		r.onMatchStart(t, m) // Tasks 10, 12
		final := r.play(t, m)
		t.mu.Lock()
		t.cur = nil
		t.history = append(t.history, m)
		t.mu.Unlock()
		switch final {
		case protocol.MatchCrashed:
			r.halt(t, fmt.Errorf("%s", m.reason))
			return
		case protocol.MatchAborted:
			t.setState(protocol.TableIdle)
			return
		}
		if !t.cfg.Perpetual {
			t.setState(protocol.TableIdle)
			return
		}
		t.setState(protocol.TableCooldown)
		r.opts.Sleep(r.opts.Cooldown)
		select {
		case <-t.stop:
			t.setState(protocol.TableIdle)
			return
		default:
		}
	}
}

// halt is D15's second half for the table: it stops and stays stopped.
// Task 13 adds the crash file and the table_halted frame.
func (r *Registry) halt(t *table, err error) {
	t.mu.Lock()
	t.state = protocol.TableHalted
	t.mu.Unlock()
}

// Tables lists every table, sorted by ID.
func (r *Registry) Tables() []protocol.TableInfo {
	out := make([]protocol.TableInfo, 0)
	for _, id := range r.ids() {
		r.mu.RLock()
		t := r.tables[id]
		r.mu.RUnlock()
		out = append(out, t.info())
	}
	return out
}

// Matches lists a table's matches in ascending order; the live one last.
func (r *Registry) Matches(id TableID) ([]protocol.MatchInfo, error) {
	r.mu.RLock()
	t, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	t.mu.RLock()
	ms := append([]*match(nil), t.history...)
	if t.cur != nil {
		ms = append(ms, t.cur)
	}
	t.mu.RUnlock()
	out := make([]protocol.MatchInfo, 0, len(ms))
	for _, m := range ms {
		m.mu.RLock()
		out = append(out, m.info())
		m.mu.RUnlock()
	}
	return out, nil
}

// Close stops every table, aborts in-progress matches, closes sessions and
// waits for the goroutines. Idempotent.
func (r *Registry) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	for _, t := range r.tables {
		close(t.stop)
	}
	close(r.done)
	r.mu.Unlock()
	r.wg.Wait()
	r.closeSessions() // Task 10
	return nil
}

// Done is closed once Close has signalled every table to stop — before Close
// waits for them — so a Sleep hook that triggers Close can wait for the
// signal without deadlocking on its own goroutine.
func (r *Registry) Done() <-chan struct{} { return r.done }

// ids is the sorted table list every enumeration walks, so no map order
// ever reaches a frame or a file.
func (r *Registry) ids() []TableID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]TableID, 0, len(r.tables))
	for id := range r.tables {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// Stubs the later tasks replace.
func (r *Registry) load() error                        { return nil }
func (r *Registry) save() error                        { return nil }
func (r *Registry) onMatchStart(t *table, m *match)    {}
func (r *Registry) closeSessions()                     {}
```

`Session` is declared in Task 10; for this task to compile, add a placeholder `type Session struct{}` in `registry.go` with a `// Task 10 replaces this.` comment — and remove it in Task 10.

- [ ] **Step 8: Run the tests, with `-race`**

Run: `go test -race ./host/ -count=1 -v`
Expected: PASS, six tests, each match under a second. If `TestAPerpetualTable…` hangs, the `stop` check after cooldown is missing or `Close` was called before `Start` registered the goroutine — fix the code, not the test.

- [ ] **Step 9: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./...`

```bash
git add host/
git commit -m "feat(host): table registry and the match loop

One goroutine per table plays bot matches with seeds derived from the
table's own; the same configuration yields the same chain head twice.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 10: `host` sessions — subscriptions, frames, ring, overflow

**Files:**
- Create: `host/session.go`, `host/fanout.go`, `host/session_test.go`
- Modify: `host/registry.go` (replace the `Session` placeholder and the `fanout`/`onMatchStart`/`onMatchEnd`/`closeSessions` stubs; `halt` sends `table_halted`)

**Interfaces:**
- Consumes: Task 9's `match`/`table`, `view.ProjectFor`, `view.RedactEventsFor`, `view.Describe`, `protocol.NewFrame`.
- Produces:

```go
type Session struct { ID string /* ... */ }
func (r *Registry) OpenSession() *Session
func (r *Registry) Session(id string) (*Session, bool)
func (r *Registry) CloseSession(id string)
func (r *Registry) Hello(s *Session) protocol.Frame                  // id-less; the http layer sends it first
func (r *Registry) Subscribe(s *Session, id TableID, mode string) error // TableAll only with ModeOverview; focus pushes a snapshot immediately
func (r *Registry) Unsubscribe(s *Session, id TableID) error
func (s *Session) Out() <-chan protocol.Frame        // ring-buffered frames with IDs; closed on overflow or CloseSession
func (s *Session) TakeWidgets() []protocol.Frame     // latest widget per subscribed table, sorted by table id; clears
func (s *Session) Since(id uint64) ([]protocol.Frame, bool) // frames with ID > id from the ring; false if id predates the ring
func (s *Session) Overflowed() (dropped int, overflowed bool)
```

- [ ] **Step 1: Write the failing tests**

Create `host/session_test.go`:

```go
package host

import (
	"testing"

	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// drain reads every frame until Out closes or the table finishes, returning
// them in order. Sessions are drained on the test goroutine while the
// table plays on its own, the same shape as the SSE writer.
func drain(t *testing.T, r *Registry, s *Session, id TableID) []protocol.Frame {
	t.Helper()
	var out []protocol.Frame
	done := make(chan struct{})
	go func() { r.Wait(id); close(done) }()
	for {
		select {
		case f, ok := <-s.Out():
			if !ok {
				return out
			}
			out = append(out, f)
		case <-done:
			for {
				select {
				case f, ok := <-s.Out():
					if !ok {
						return out
					}
					out = append(out, f)
				default:
					return out
				}
			}
		}
	}
}

func decode[T any](t *testing.T, f protocol.Frame) T {
	t.Helper()
	var v T
	if err := f.Decode(&v); err != nil {
		t.Fatalf("%s: %v", f.T, err)
	}
	return v
}

func TestFocusSubscriptionStreamsSnapshotThenEventsInChainOrder(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	s := r.OpenSession()
	hello := decode[protocol.Hello](t, r.Hello(s))
	if hello.Session != s.ID || len(hello.Tables) != 1 || hello.Tables[0].ID != "t1" {
		t.Fatalf("hello %+v", hello)
	}
	if err := r.Subscribe(s, "t1", protocol.ModeFocus); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	frames := drain(t, r, s, "t1")
	if len(frames) < 100 {
		t.Fatalf("only %d frames", len(frames))
	}
	// match_start first, then the snapshot, then events/decisions, then match_end.
	if frames[0].T != protocol.TMatchStart || frames[1].T != protocol.TSnapshot || frames[len(frames)-1].T != protocol.TMatchEnd {
		t.Fatalf("shape: first %s, second %s, last %s", frames[0].T, frames[1].T, frames[len(frames)-1].T)
	}
	snap := decode[protocol.Snapshot](t, frames[1])
	if snap.View.Visibility != "omniscient" || len(snap.TurnStarts) < 1 || snap.View.Players[0].Hand == nil {
		t.Fatalf("snapshot %+v", snap.View)
	}
	var lastSeq uint64
	var lastID uint64
	events, decisions := 0, 0
	for i, f := range frames {
		if f.ID == 0 || f.ID <= lastID {
			t.Fatalf("frame %d: id %d not monotonic after %d", i, f.ID, lastID)
		}
		lastID = f.ID
		if f.Table != "t1" || f.Match != 1 {
			t.Fatalf("frame %d addressed to %s/%d", i, f.Table, f.Match)
		}
		switch f.T {
		case protocol.TEvent:
			eb := decode[protocol.EventBody](t, f)
			if eb.Event.Seq != f.Seq || (events > 0 && f.Seq != lastSeq+1) {
				t.Fatalf("frame %d: event seq %d after %d", i, f.Seq, lastSeq)
			}
			if eb.Event.Kind == "shuffle" && len(eb.Event.IDs) != 0 {
				t.Fatalf("frame %d leaks library order", i)
			}
			if eb.Line == "" && eb.Event.Kind != "clock_tick" {
				t.Fatalf("frame %d: no line for %s", i, eb.Event.Kind)
			}
			lastSeq = f.Seq
			events++
		case protocol.TDecision:
			d := decode[protocol.DecisionBody](t, f)
			if d.Kind == "" || d.Prompt == "" {
				t.Fatalf("decision %+v", d)
			}
			decisions++
		}
	}
	if events == 0 || decisions == 0 {
		t.Fatalf("%d events, %d decisions", events, decisions)
	}
	end := decode[protocol.MatchEnd](t, frames[len(frames)-1])
	ms, _ := r.Matches("t1")
	if end.Head != ms[0].Head || end.Result != ms[0].Result {
		t.Fatalf("match_end %+v vs %+v", end, ms[0])
	}
	// The event stream after the snapshot covers exactly the events after
	// the snapshot's head.
	if firstEvent := frames[2]; firstEvent.T == protocol.TEvent && firstEvent.Seq != snap.Head+1 {
		t.Fatalf("first event seq %d, snapshot head %d", firstEvent.Seq, snap.Head)
	}
}

func TestPublicTableRedactsHandsInEventsAndSnapshot(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	cfg := fourSeatTable("t1", false)
	cfg.Spectator = view.Public
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	s := r.OpenSession()
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	_ = r.Start("t1")
	frames := drain(t, r, s, "t1")
	snap := decode[protocol.Snapshot](t, frames[1])
	for _, p := range snap.View.Players {
		if p.Hand != nil {
			t.Fatal("public snapshot shows a hand")
		}
	}
	for _, f := range frames {
		if f.T != protocol.TEvent {
			continue
		}
		eb := decode[protocol.EventBody](t, f)
		if eb.Event.Kind == "draw" && eb.Event.Obj != 0 {
			// A draw's card is hidden unless it has since become public
			// (played/cast) — with state-at-burst redaction the very draw
			// burst still hides it.
			t.Fatalf("public draw event names the card: %+v", eb)
		}
	}
}

func TestOverviewSubscriptionCoalescesWidgets(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	_ = r.AddTable(fourSeatTable("t2", false))
	s := r.OpenSession()
	if err := r.Subscribe(s, protocol.TableAll, protocol.ModeOverview); err != nil {
		t.Fatal(err)
	}
	if err := r.Subscribe(s, protocol.TableAll, protocol.ModeFocus); err == nil {
		t.Fatal("focus on * accepted")
	}
	_ = r.StartAll()
	r.Wait("t1")
	r.Wait("t2")
	ws := s.TakeWidgets()
	if len(ws) != 2 || ws[0].Table != "t1" || ws[1].Table != "t2" {
		t.Fatalf("widgets %+v", ws)
	}
	w := decode[protocol.Widget](t, ws[0])
	if len(w.Life) != 4 || len(w.Lost) != 4 || w.State != protocol.MatchFinished || w.Last == "" {
		t.Fatalf("widget %+v", w)
	}
	if ws[0].ID != 0 {
		t.Fatal("widgets must not consume frame ids (PL-5)")
	}
	if again := s.TakeWidgets(); len(again) != 0 {
		t.Fatal("TakeWidgets did not clear")
	}
	// Overview frames with ids: match_start and match_end per table only.
	for _, f := range drainNow(s) {
		if f.T != protocol.TMatchStart && f.T != protocol.TMatchEnd {
			t.Fatalf("overview stream carried %s", f.T)
		}
	}
}

// drainNow returns whatever is already buffered without blocking.
func drainNow(s *Session) []protocol.Frame {
	var out []protocol.Frame
	for {
		select {
		case f, ok := <-s.Out():
			if !ok {
				return out
			}
			out = append(out, f)
		default:
			return out
		}
	}
}

func TestRingResumesExactlyTheMissedFrames(t *testing.T) {
	o := testOptions(t)
	o.Ring = 64
	r, _ := New(o)
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	s := r.OpenSession()
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	_ = r.Start("t1")
	frames := drain(t, r, s, "t1")
	if _, ok := s.Overflowed(); ok {
		t.Fatal("a drained session overflowed")
	}
	last := frames[len(frames)-1]
	missed, ok := s.Since(last.ID - 10)
	if !ok || len(missed) != 10 || missed[0].ID != last.ID-9 || missed[9].ID != last.ID {
		t.Fatalf("Since: ok=%v, %d frames, first %d last %d", ok, len(missed), missed[0].ID, missed[len(missed)-1].ID)
	}
	if _, ok := s.Since(last.ID - 1000); ok {
		t.Fatal("Since reported success for an id older than the ring")
	}
	if got, ok := s.Since(last.ID); !ok || len(got) != 0 {
		t.Fatalf("Since(head) = %d frames, ok=%v", len(got), ok)
	}
}

func TestASessionThatNeverReadsIsDroppedAndTheMatchStillFinishes(t *testing.T) {
	o := testOptions(t)
	o.Ring = 16
	r, _ := New(o)
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	s := r.OpenSession()
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	_ = r.Start("t1")
	r.Wait("t1")
	dropped, overflowed := s.Overflowed()
	if !overflowed || dropped == 0 {
		t.Fatalf("overflowed=%v dropped=%d", overflowed, dropped)
	}
	if _, open := <-s.Out(); open {
		// The channel still holds up to Ring frames; drain to the close.
		for range s.Out() {
		}
	}
	if _, ok := r.Session(s.ID); ok {
		t.Fatal("overflowed session still registered")
	}
	ms, _ := r.Matches("t1")
	if ms[0].State != protocol.MatchFinished {
		t.Fatalf("match %s; a slow subscriber must never stall the engine", ms[0].State)
	}
}

func TestUnsubscribeStopsFramesAndCloseSessionClosesOut(t *testing.T) {
	r, _ := New(testOptions(t))
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	s := r.OpenSession()
	if err := r.Subscribe(s, "t9", protocol.ModeFocus); err == nil {
		t.Fatal("subscribe to unknown table succeeded")
	}
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	if err := r.Unsubscribe(s, "t1"); err != nil {
		t.Fatal(err)
	}
	_ = r.Start("t1")
	r.Wait("t1")
	n := len(drainNow(s))
	if n > 1 { // at most the snapshot pushed by Subscribe before Unsubscribe
		t.Fatalf("%d frames after unsubscribe", n)
	}
	r.CloseSession(s.ID)
	if _, open := <-s.Out(); open {
		for range s.Out() {
		}
	}
	if _, ok := r.Session(s.ID); ok {
		t.Fatal("closed session still registered")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./host/ -run 'Focus|Public|Overview|Ring|NeverReads|Unsubscribe' -count=1`
Expected: FAIL — `OpenSession undefined`.

- [ ] **Step 3: Implement `host/session.go`**

```go
package host

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/adams-shaun/gorge/protocol"
)

// Session is one client connection's worth of subscriptions: a bounded
// outbound channel, a ring of the frames it has sent (for Last-Event-ID
// resume), and the latest widget per table (PL-5: widgets are coalesced,
// never ring-buffered, never given an id).
type Session struct {
	ID string

	mu         sync.Mutex
	out        chan protocol.Frame
	ring       []protocol.Frame // oldest first, len <= cap(out)
	nextID     uint64
	subs       map[TableID]string // table -> mode; TableAll for "every table, overview"
	widgets    map[TableID]protocol.Frame
	dropped    int
	overflowed bool
	closed     bool
}

// OpenSession registers a new session. IDs are a counter ("s1", "s2", …):
// they are not secrets (the authorizer hook, not the session id, is what
// gates access), so no randomness is needed or wanted.
func (r *Registry) OpenSession() *Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextSess++
	s := &Session{ID: "s" + strconv.Itoa(r.nextSess), out: make(chan protocol.Frame, r.opts.Ring),
		subs: map[TableID]string{}, widgets: map[TableID]protocol.Frame{}}
	r.sessions[s.ID] = s
	return s
}

// Session looks a session up by id.
func (r *Registry) Session(id string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	return s, ok
}

// CloseSession drops the session's subscriptions and closes Out.
func (r *Registry) CloseSession(id string) {
	r.mu.Lock()
	s, ok := r.sessions[id]
	delete(r.sessions, id)
	r.mu.Unlock()
	if ok {
		s.close()
	}
}

func (r *Registry) closeSessions() {
	r.mu.Lock()
	ss := r.sessions
	r.sessions = map[string]*Session{}
	r.mu.Unlock()
	for _, s := range ss {
		s.close()
	}
}

func (s *Session) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.out)
	}
}

// Out is the resumable frame stream. It is closed after an overflow (the
// http layer then sends the overflow frame itself) or CloseSession.
func (s *Session) Out() <-chan protocol.Frame { return s.out }

// Overflowed reports whether the channel ever filled and how many frames
// were dropped before Out was closed.
func (s *Session) Overflowed() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped, s.overflowed
}

// push assigns the next id, records the frame in the ring and hands it to
// the channel without ever blocking: a full channel means the reader is
// too slow, so the session overflows and is closed (spec: "the engine loop
// never waited"). Returns false once the session is closed or overflowed.
func (s *Session) push(f protocol.Frame) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.overflowed {
		return false
	}
	s.nextID++
	f.ID = s.nextID
	if len(s.ring) == cap(s.out) {
		copy(s.ring, s.ring[1:])
		s.ring = s.ring[:len(s.ring)-1]
	}
	s.ring = append(s.ring, f)
	select {
	case s.out <- f:
		return true
	default:
		s.dropped++
		s.overflowed = true
		s.closed = true
		close(s.out)
		return false
	}
}

// setWidget replaces the latest widget for a table.
func (s *Session) setWidget(id TableID, f protocol.Frame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.widgets[id] = f
}

// TakeWidgets returns and clears the latest widget per table, in table-id
// order. The SSE writer calls it on every tick.
func (s *Session) TakeWidgets() []protocol.Frame {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.widgets))
	for id := range s.widgets {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]protocol.Frame, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.widgets[TableID(id)])
	}
	s.widgets = map[TableID]protocol.Frame{}
	return out
}

// Since returns every ring frame with ID > id, in order. ok is false when
// id is older than the oldest frame still in the ring (or the ring is
// empty and id is not the current head), in which case the caller must
// start the client over with a fresh hello and snapshots.
func (s *Session) Since(id uint64) ([]protocol.Frame, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == s.nextID {
		return nil, true
	}
	if len(s.ring) == 0 || id < s.ring[0].ID-1 || id > s.nextID {
		return nil, false
	}
	var out []protocol.Frame
	for _, f := range s.ring {
		if f.ID > id {
			out = append(out, f)
		}
	}
	return out, true
}

// subscribed reports the session's mode for a table: its own entry, else
// the wildcard's.
func (s *Session) modeFor(id TableID) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.subs[id]; ok {
		return m, true
	}
	if m, ok := s.subs[TableAll]; ok {
		return m, true
	}
	return "", false
}

// TableAll as a TableID, for the subscription map.
const TableAll TableID = protocol.TableAll

// Hello is the stream's first frame: the session id and the table list.
func (r *Registry) Hello(s *Session) protocol.Frame {
	f, err := protocol.NewFrame(protocol.THello, "", 0, 0, protocol.Hello{Session: s.ID, Tables: r.Tables()})
	if err != nil {
		panic("host: hello frame: " + err.Error()) // a marshal failure of our own struct is a bug
	}
	return f
}

// Subscribe adds a table (or every table, overview only) to the session. A
// focus subscription on a live table pushes a snapshot at once so the
// client has a board before the first event frame arrives.
func (r *Registry) Subscribe(s *Session, id TableID, mode string) error {
	if mode != protocol.ModeOverview && mode != protocol.ModeFocus {
		return fmt.Errorf("host: unknown mode %q", mode)
	}
	if id == TableAll {
		if mode != protocol.ModeOverview {
			return fmt.Errorf("host: %q may only be subscribed in overview mode", protocol.TableAll)
		}
		s.mu.Lock()
		s.subs[TableAll] = mode
		s.mu.Unlock()
		return nil
	}
	r.mu.RLock()
	t, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		return ErrNotFound
	}
	s.mu.Lock()
	s.subs[id] = mode
	s.mu.Unlock()
	if mode == protocol.ModeFocus {
		t.mu.RLock()
		m := t.cur
		t.mu.RUnlock()
		if m != nil {
			m.mu.RLock()
			f := r.snapshotFrame(t, m)
			m.mu.RUnlock()
			s.push(f)
		}
	}
	return nil
}

// Unsubscribe removes one table (or the wildcard).
func (r *Registry) Unsubscribe(s *Session, id TableID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subs[id]; !ok {
		return ErrNotFound
	}
	delete(s.subs, id)
	delete(s.widgets, id)
	return nil
}

// sessionsFor lists the sessions subscribed to a table with their modes,
// in session-id order (creation order), so fan-out order is deterministic.
func (r *Registry) sessionsFor(id TableID) (ss []*Session, modes []string) {
	r.mu.RLock()
	all := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		all = append(all, s)
	}
	r.mu.RUnlock()
	sort.Slice(all, func(i, j int) bool { return sessionNum(all[i].ID) < sessionNum(all[j].ID) })
	for _, s := range all {
		if m, ok := s.modeFor(id); ok {
			ss = append(ss, s)
			modes = append(modes, m)
		}
	}
	return ss, modes
}

func sessionNum(id string) int {
	n, _ := strconv.Atoi(id[1:])
	return n
}

// dropOverflowed unregisters sessions that overflowed during a fan-out.
func (r *Registry) dropOverflowed(ss []*Session) {
	for _, s := range ss {
		if _, of := s.Overflowed(); of {
			r.mu.Lock()
			delete(r.sessions, s.ID)
			r.mu.Unlock()
		}
	}
}
```

- [ ] **Step 4: Implement `host/fanout.go`**

```go
package host

import (
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// frame builds an addressed frame; a marshal failure of our own structs is
// a bug, not a runtime condition.
func frame(t FrameType, tab *table, k int, seq uint64, body any) protocol.Frame {
	f, err := protocol.NewFrame(t, string(tab.cfg.ID), k, seq, body)
	if err != nil {
		panic("host: frame: " + err.Error())
	}
	return f
}

type FrameType = protocol.FrameType

// head is the seq of the last event, or 0 for an empty log.
func head(m *match) uint64 {
	if n := len(m.e.L.Events); n > 0 {
		return uint64(n - 1)
	}
	return 0
}

// snapshotFrame is the whole board at head in the table's visibility plus
// the turn starts. Called with m.mu held for reading.
func (r *Registry) snapshotFrame(t *table, m *match) protocol.Frame {
	v := view.ProjectFor(m.e.G, m.e, view.NoSeat, t.cfg.Spectator, nil)
	return frame(protocol.TSnapshot, t, m.k, head(m), protocol.Snapshot{
		View: v, TurnStarts: append([]uint64(nil), m.turnStarts...), Head: head(m)})
}

// widgetFrame is the overview cell. Called with m.mu held for reading.
func (r *Registry) widgetFrame(t *table, m *match, last string) protocol.Frame {
	g := m.e.G
	w := protocol.Widget{Turn: g.Turn, Step: g.Step.String(), Phase: view.PhaseOf(g.Step),
		Active: uint8(g.Active), Priority: uint8(g.Priority), StackDepth: len(g.Stack), Last: last, State: m.state}
	for _, p := range g.Players {
		w.Life = append(w.Life, p.Life)
		w.Lost = append(w.Lost, p.Lost)
	}
	if w.Life == nil {
		w.Life, w.Lost = []int32{}, []bool{}
	}
	return frame(protocol.TWidget, t, m.k, head(m), w)
}

// eventBodies redacts and describes the events from index from, against
// the state that produced them (RedactEvents' convention). Called with
// m.mu held for reading. Describe runs on the REDACTED event so a hidden
// card's name never reaches the line.
func (r *Registry) eventBodies(t *table, m *match, from int) []protocol.EventBody {
	evs := view.RedactEventsFor(m.e.G, m.e.L.Events[from:], view.NoSeat, t.cfg.Spectator)
	out := make([]protocol.EventBody, 0, len(evs))
	for _, ev := range evs {
		out = append(out, protocol.EventBody{Event: protocol.EventFrom(ev), Line: view.Describe(m.e.G, ev)})
	}
	return out
}

// lastLine is the transcript line of the most recent described event that
// has one, for widget.last.
func lastLine(bodies []protocol.EventBody, prev string) string {
	for i := len(bodies) - 1; i >= 0; i-- {
		if bodies[i].Line != "" {
			return bodies[i].Line
		}
	}
	return prev
}

// fanout delivers one burst to every subscribed session: focus sessions
// get the events (and the pending decision, if any) as frames; overview
// sessions get a coalesced widget. It never blocks on a client.
func (r *Registry) fanout(t *table, m *match, before int) {
	ss, modes := r.sessionsFor(t.cfg.ID)
	if len(ss) == 0 {
		return
	}
	m.mu.RLock()
	bodies := r.eventBodies(t, m, before)
	var evFrames []protocol.Frame
	for _, b := range bodies {
		evFrames = append(evFrames, frame(protocol.TEvent, t, m.k, b.Event.Seq, b))
	}
	var decision *protocol.Frame
	if d := m.e.Pending(); d != nil {
		f := frame(protocol.TDecision, t, m.k, head(m), protocol.DecisionBody{Player: uint8(d.Player), Kind: string(d.Kind), Prompt: d.Prompt})
		decision = &f
	}
	widget := r.widgetFrame(t, m, lastLine(bodies, ""))
	m.mu.RUnlock()
	for i, s := range ss {
		switch modes[i] {
		case protocol.ModeFocus:
			for _, f := range evFrames {
				if !s.push(f) {
					break
				}
			}
			if decision != nil {
				s.push(*decision)
			}
		case protocol.ModeOverview:
			s.setWidget(t.cfg.ID, widget)
		}
	}
	r.dropOverflowed(ss)
}

// onMatchStart announces a match to every subscriber and gives focus
// subscribers their first snapshot.
func (r *Registry) onMatchStart(t *table, m *match) {
	ss, modes := r.sessionsFor(t.cfg.ID)
	if len(ss) == 0 {
		return
	}
	m.mu.RLock()
	start := frame(protocol.TMatchStart, t, m.k, 0, protocol.MatchStart{Seats: m.seats, Seed: m.seed, Spectator: t.cfg.Spectator.String()})
	snap := r.snapshotFrame(t, m)
	bodies := r.eventBodies(t, m, 0)
	var evFrames []protocol.Frame
	for _, b := range bodies {
		evFrames = append(evFrames, frame(protocol.TEvent, t, m.k, b.Event.Seq, b))
	}
	widget := r.widgetFrame(t, m, lastLine(bodies, ""))
	m.mu.RUnlock()
	for i, s := range ss {
		s.push(start)
		if modes[i] == protocol.ModeFocus {
			s.push(snap)
			// Genesis events are already inside the snapshot's head; a
			// focus client starts from the snapshot, so they are not
			// re-sent as frames.
		} else {
			s.setWidget(t.cfg.ID, widget)
		}
	}
	r.dropOverflowed(ss)
}

// onMatchEnd sends match_end (any final state) to every subscriber and a
// final widget to overview ones.
func (r *Registry) onMatchEnd(t *table, m *match) {
	ss, modes := r.sessionsFor(t.cfg.ID)
	if len(ss) == 0 {
		return
	}
	m.mu.RLock()
	end := frame(protocol.TMatchEnd, t, m.k, head(m), protocol.MatchEnd{Result: m.result, Winner: m.winner, Head: m.head})
	widget := r.widgetFrame(t, m, "")
	m.mu.RUnlock()
	for i, s := range ss {
		s.push(end)
		if modes[i] == protocol.ModeOverview {
			s.setWidget(t.cfg.ID, widget)
		}
	}
	r.dropOverflowed(ss)
}

// haltFrame is sent by halt (Task 13 wires the report file).
func (r *Registry) sendHalted(t *table, k int, reason string) {
	ss, _ := r.sessionsFor(t.cfg.ID)
	for _, s := range ss {
		s.push(frame(protocol.TTableHalted, t, k, 0, protocol.TableHaltedBody{Reason: reason}))
	}
	r.dropOverflowed(ss)
}
```

`view.PhaseOf` does not exist yet: in `view/view.go` rename `phaseOf` to `PhaseOf` and export it with the doc "PhaseOf groups a Step into the five phases a client shows: beginning, main1, combat, main2, ending; "" for an invalid Step." (one-line change, no behaviour change; `go test ./view/` must stay green).

Then delete Task 9's stubs that these files now define — `fanout` and `onMatchEnd` in `match.go`, `onMatchStart` and `closeSessions` in `registry.go` — and the `Session` placeholder in `registry.go`, and make `halt` call `r.sendHalted(t, t.k, err.Error())` after setting the state (read `t.k` under the lock). In `match.go` the `widgetFrame` needs the match's `state` — it reads `m.state` under the caller's lock, which `finish/abort/crash` set before calling `onMatchEnd`.

- [ ] **Step 5: Run the tests with `-race`**

Run: `go test -race ./host/ -count=1 -v`
Expected: PASS. The overflow test must finish in well under a second: a full channel drops, it never waits.

- [ ] **Step 6: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./...`

```bash
git add host/ view/view.go
git commit -m "feat(host): sessions, subscriptions, frame fan-out, ring resume, overflow

Focus subscribers get a snapshot then events and decisions in chain
order; overview subscribers get one coalesced widget per table; a slow
reader overflows and is dropped without the engine ever waiting.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 11: `host` snapshots — `ViewAt` and `Events`

**Files:**
- Create: `host/snapshot.go`, `host/viewat.go`, `host/viewat_test.go`
- Modify: `host/match.go` (`afterBurst` takes turn-start snapshots)

**Interfaces:**
- Consumes: `rules.Engine.Clone` (Task 1), `replay.ReplayTo`, `events.Apply`, `view.ProjectFor`, `view.RedactEventsFor`, `view.Describe`.
- Produces:

```go
func (r *Registry) ViewAt(id TableID, k int, seq uint64) (view.View, error)          // ErrNotFound; ErrBeyondHead
func (r *Registry) Events(id TableID, k int, since uint64) ([]protocol.EventBody, error)
type ErrBeyondHead struct{ Head uint64 }
func (e ErrBeyondHead) Error() string
// internal, shared with Task 12's finished-match path:
func boundsOf(evs []events.Event) []uint64
func viewAt(cfg rules.Config, l *events.Log, snaps []snapshot, seq uint64, vis view.Visibility) (view.View, error)
```

- [ ] **Step 1: Write the failing tests**

Create `host/viewat_test.go`:

```go
package host

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

func finishedTable(t *testing.T) (*Registry, *match) {
	t.Helper()
	r, _ := New(testOptions(t))
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	_ = r.Start("t1")
	r.Wait("t1")
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	return r, tb.history[0]
}

func viewJSON(t *testing.T, v view.View) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBoundsOfMatchesTheLoopsOwnBookkeeping(t *testing.T) {
	_, m := finishedTable(t)
	got := boundsOf(m.e.L.Events)
	if len(got) != len(m.bounds) {
		t.Fatalf("boundsOf found %d boundaries, the loop recorded %d", len(got), len(m.bounds))
	}
	for i := range got {
		if got[i] != m.bounds[i] {
			t.Fatalf("boundary %d: derived %d, recorded %d", i, got[i], m.bounds[i])
		}
	}
	// One snapshot at genesis plus one per burst that began a turn; a burst
	// can contain two turn changes only in degenerate games, so at most one
	// snapshot per turn start.
	if len(m.snaps) < 3 || len(m.snaps) > len(m.turnStarts) {
		t.Fatalf("%d snapshots for %d turn starts", len(m.snaps), len(m.turnStarts))
	}
}

func TestViewAtFromSnapshotsEqualsViewAtFromGenesis(t *testing.T) {
	_, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	rng := rand.New(rand.NewPCG(1, 2))
	seqs := []uint64{0, m.bounds[0] - 1, m.bounds[0], head}
	for i := 0; i < 40; i++ {
		seqs = append(seqs, uint64(rng.IntN(int(head)+1)))
	}
	for _, ts := range m.turnStarts {
		seqs = append(seqs, ts, ts+1)
	}
	for _, seq := range seqs {
		if seq > head {
			continue
		}
		fast, err := viewAt(m.cfg, m.e.L, m.snaps, seq, view.Omniscient)
		if err != nil {
			t.Fatalf("seq %d (snapshots): %v", seq, err)
		}
		slow, err := viewAt(m.cfg, m.e.L, nil, seq, view.Omniscient)
		if err != nil {
			t.Fatalf("seq %d (genesis): %v", seq, err)
		}
		if a, b := viewJSON(t, fast), viewJSON(t, slow); a != b {
			t.Fatalf("seq %d: snapshot path differs from full replay\n%s\n%s", seq, a, b)
		}
	}
}

func TestViewAtHeadEqualsTheLiveProjection(t *testing.T) {
	r, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	got, err := r.ViewAt("t1", 1, head)
	if err != nil {
		t.Fatal(err)
	}
	want := view.ProjectFor(m.e.G, m.e, view.NoSeat, view.Omniscient, nil)
	if viewJSON(t, got) != viewJSON(t, want) {
		t.Fatal("ViewAt(head) differs from projecting the live engine")
	}
	if got.Over != true {
		t.Fatal("finished match's head view is not Over")
	}
}

func TestViewAtTracksIntraBurstStateChanges(t *testing.T) {
	_, m := finishedTable(t)
	// Find a LifeChange or Damage-to-player event and check the life total
	// moves at exactly that seq — inside a burst, not at its boundary.
	for _, ev := range m.e.L.Events {
		if ev.Kind != events.LifeChange || ev.Amount == 0 {
			continue
		}
		before, err := viewAt(m.cfg, m.e.L, m.snaps, ev.Seq-1, view.Omniscient)
		if err != nil {
			t.Fatal(err)
		}
		after, err := viewAt(m.cfg, m.e.L, m.snaps, ev.Seq, view.Omniscient)
		if err != nil {
			t.Fatal(err)
		}
		if after.Players[ev.Player].Life-before.Players[ev.Player].Life != ev.Amount {
			t.Fatalf("seq %d: life moved %d, event says %d", ev.Seq,
				after.Players[ev.Player].Life-before.Players[ev.Player].Life, ev.Amount)
		}
		return
	}
	t.Skip("fixture produced no life change")
}

func TestViewAtAndEventsErrors(t *testing.T) {
	r, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	_, err := r.ViewAt("t1", 1, head+1)
	var beyond ErrBeyondHead
	if !errors.As(err, &beyond) || beyond.Head != head {
		t.Fatalf("beyond head: %v", err)
	}
	if _, err := r.ViewAt("t9", 1, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown table: %v", err)
	}
	if _, err := r.ViewAt("t1", 2, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown match: %v", err)
	}
	if _, err := r.Events("t1", 1, head+1); !errors.As(err, &beyond) {
		t.Fatalf("events beyond head: %v", err)
	}
}

func TestEventsSinceReturnsRedactedDescribedTail(t *testing.T) {
	r, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	all, err := r.Events("t1", 1, 0)
	if err != nil || len(all) != int(head)+1 || all[0].Event.Seq != 0 || all[len(all)-1].Event.Seq != head {
		t.Fatalf("Events(0): %d bodies, %v", len(all), err)
	}
	tail, _ := r.Events("t1", 1, head-4)
	if len(tail) != 5 || tail[0].Event.Seq != head-4 {
		t.Fatalf("Events(head-4): %d bodies from %d", len(tail), tail[0].Event.Seq)
	}
	for _, b := range all {
		if b.Event.Kind == "shuffle" && len(b.Event.IDs) != 0 {
			t.Fatal("Events leaks library order")
		}
		if b.Line == "" && b.Event.Kind != "clock_tick" {
			t.Fatalf("seq %d (%s) has no line", b.Event.Seq, b.Event.Kind)
		}
	}
	_ = protocol.TEvent
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./host/ -run 'Bounds|ViewAt|Events' -count=1`
Expected: FAIL — `boundsOf undefined`.

- [ ] **Step 3: Implement `host/snapshot.go`**

```go
package host

import "github.com/adams-shaun/gorge/events"

// afterBurst runs inside the loop's Lock after a successful Submit: when
// the burst began a turn, the engine is cloned at this boundary (spec D11:
// turn-start snapshots, dropped with the match). Task 12 appends the
// burst to disk here too.
func (r *Registry) afterBurst(t *table, m *match, before int) error {
	if len(turnStartsIn(m.e.L.Events, before)) > 0 {
		m.snaps = append(m.snaps, snapshot{intent: m.intents, seq: m.bounds[len(m.bounds)-1] - 1, e: m.e.Clone()})
	}
	return r.persistBurst(t, m, before) // Task 12
}

// snapshotGenesis clones the engine at boundary 0, so a view inside turn 1
// never replays from scratch either.
func (m *match) snapshotGenesis() {
	m.snaps = append(m.snaps, snapshot{intent: 0, seq: m.bounds[0] - 1, e: m.e.Clone()})
}

// boundsOf derives the intent boundaries from the log alone: every burst
// but the last ends with the DecisionAsk of the next decision (Submit's
// handle/checkStateBased/Advance run until ask), so bounds[j] is one past
// the (j+1)-th DecisionAsk; a finished log's final burst ends at len. It
// equals the loop's own bookkeeping (viewat_test pins that) and lets a
// finished match served from files answer ViewAt without persisting bounds.
func boundsOf(evs []events.Event) []uint64 {
	var out []uint64
	for _, ev := range evs {
		if ev.Kind == events.DecisionAsk {
			out = append(out, ev.Seq+1)
		}
	}
	n := uint64(len(evs))
	if len(out) == 0 || out[len(out)-1] != n {
		out = append(out, n)
	}
	return out
}

func (r *Registry) persistBurst(t *table, m *match, before int) error { return nil } // Task 12 replaces
```

In `match.go`'s `newMatch`, call `m.snapshotGenesis()` right after `m.turnStarts` is set, and delete the old `afterBurst` stub.

On `boundsOf`'s last element: after the final Submit of a finished game there is no DecisionAsk, so the loop's `bounds` ends with `len(events)`; for a live game the loop's last boundary is also `len(events)` and equals `lastAsk+1`. Step 1's test pins equality with the loop's `bounds` in both shapes; if it fails, fix `boundsOf` — the loop's bookkeeping is the ground truth.

- [ ] **Step 4: Implement `host/viewat.go`**

```go
package host

import (
	"fmt"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/view"
)

// ErrBeyondHead says a requested seq is past the match's last event; Head
// is the last valid seq, which the http layer returns with a 409.
type ErrBeyondHead struct{ Head uint64 }

func (e ErrBeyondHead) Error() string { return fmt.Sprintf("host: seq beyond head %d", e.Head) }

// ViewAt is the board as of event seq (inclusive) in the table's
// visibility. For a live match it uses the turn-start snapshots; a finished
// match (Task 12) replays from its files.
func (r *Registry) ViewAt(id TableID, k int, seq uint64) (view.View, error) {
	t, m, err := r.lookup(id, k)
	if err != nil {
		return view.View{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return viewAt(m.cfg, m.e.L, m.snaps, seq, t.cfg.Spectator)
}

// Events is every redacted, described event from since (inclusive) to the
// head, in chain order. Redaction is against the state at head (PL-15).
func (r *Registry) Events(id TableID, k int, since uint64) ([]protocol.EventBody, error) {
	t, m, err := r.lookup(id, k)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := uint64(len(m.e.L.Events))
	if n == 0 {
		return nil, ErrBeyondHead{Head: 0}
	}
	if since >= n {
		return nil, ErrBeyondHead{Head: n - 1}
	}
	return r.eventBodies(t, m, int(since)), nil
}

// lookup finds a table's live or retained match. Task 12 teaches it to
// load finished matches from disk.
func (r *Registry) lookup(id TableID, k int) (*table, *match, error) {
	r.mu.RLock()
	t, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		return nil, nil, ErrNotFound
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.cur != nil && t.cur.k == k {
		return t, t.cur, nil
	}
	for _, m := range t.history {
		if m.k == k {
			return t, m, nil
		}
	}
	return nil, nil, ErrNotFound
}

// viewAt is PL-1: find the last intent boundary j with bounds[j] <= seq+1,
// reach it from the nearest snapshot (or genesis) by re-submitting the
// recorded intents, apply the rest of that burst's events onto the clone's
// own game, and project. Zones, life, damage, counters and the stack are
// exact at every seq; derived P/T from continuous effects and the pending
// tray are as of the burst's start (at most one resolution stale).
func viewAt(cfg rules.Config, l *events.Log, snaps []snapshot, seq uint64, vis view.Visibility) (view.View, error) {
	n := uint64(len(l.Events))
	if n == 0 {
		return view.View{}, ErrBeyondHead{Head: 0}
	}
	if seq >= n {
		return view.View{}, ErrBeyondHead{Head: n - 1}
	}
	bounds := boundsOf(l.Events)
	j := 0
	for j+1 < len(bounds) && bounds[j+1] <= seq+1 {
		j++
	}
	// Nearest snapshot at or before boundary j; snaps are in ascending order.
	var e *rules.Engine
	from := 0
	for i := len(snaps) - 1; i >= 0; i-- {
		if snaps[i].intent <= j {
			e = snaps[i].e.Clone()
			from = snaps[i].intent
			break
		}
	}
	if e == nil {
		var err error
		e, err = replay.ReplayTo(l, cfg, j)
		if err != nil {
			return view.View{}, fmt.Errorf("host: replay to boundary %d: %w", j, err)
		}
		from = j
	}
	for i := from; i < j && i < len(l.Intents); i++ {
		if err := e.Submit(l.Intents[i]); err != nil {
			return view.View{}, fmt.Errorf("host: replay intent %d: %w", i, err)
		}
	}
	if got := uint64(len(e.L.Events)); got != bounds[j] {
		return view.View{}, fmt.Errorf("host: replay reached seq %d, boundary %d is %d", got, j, bounds[j])
	}
	for s := bounds[j]; s <= seq; s++ {
		events.Apply(e.G, l.Events[s])
	}
	return view.ProjectFor(e.G, e, view.NoSeat, vis, nil), nil
}
```

- [ ] **Step 5: Run the tests with `-race`**

Run: `go test -race ./host/ -count=1 -v -run 'Bounds|ViewAt|Events'`
Expected: PASS. `TestViewAtFromSnapshotsEqualsViewAtFromGenesis` compares ~60 views; it should take under two seconds (each genesis replay is ~100 ms at most).

- [ ] **Step 6: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./...`

```bash
git add host/
git commit -m "feat(host): turn-start snapshots, ViewAt and Events

View at seq N replays at most one turn from a cloned engine and applies
the rest of the burst onto the clone; the property test pins it equal to
a full replay from genesis.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 12: `host` persistence — files, sidecars, `tables.json`, restart

**Files:**
- Create: `host/persist.go`, `host/restart.go`, `host/persist_test.go`
- Modify: `host/registry.go` (`load`/`save`), `host/snapshot.go` (`persistBurst` replaces its stub), `host/match.go` (`files` field; `newMatch` opens files and writes the live sidecar), `host/fanout.go` (`onMatchEnd` archives before fanning out), `host/viewat.go` (`lookup` loads finished matches), `view/visibility.go` (`MarshalText`/`UnmarshalText` so `TableConfig.Spectator` is a string in JSON)

**Interfaces:**
- Consumes: `os`, `encoding/json`, `bufio`; `deck` names via `Options.LoadDeck`.
- Produces the on-disk layout:

```
<Dir>/tables.json                 {"tables":[{"config":TableConfig,"match":k}, …]}   sorted by id
<Dir>/<table>/<k>.events          one events.Event JSON per line, append-only
<Dir>/<table>/<k>.intents         one decision.Intent JSON per line, append-only
<Dir>/<table>/<k>.json            sidecar (below), rewritten at start and end
<Dir>/crash/<table>-<k>.txt       Task 13
```

```go
type sidecar struct {
	Table string; Match int; Seed uint64; Seats []protocol.SeatInfo; Names []string; Decks []string
	Spectator string; State string; Result string; Winner *uint8; Head string; Events int; Turns int32; Reason string
}
func readLog(dir string, table TableID, k int) (*events.Log, error)   // tolerates a trailing partial line
```

- [ ] **Step 1: Write the failing tests**

Create `host/persist_test.go`:

```go
package host

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

func diskOptions(t *testing.T, dir string) Options {
	o := testOptions(t)
	o.Dir = dir
	return o
}

func playOneToDisk(t *testing.T, dir string) {
	t.Helper()
	r, err := New(diskOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	_ = r.Start("t1")
	r.Wait("t1")
}

func TestFilesAreWrittenAndByteIdenticalAcrossRuns(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	playOneToDisk(t, a)
	playOneToDisk(t, b)
	for _, name := range []string{"tables.json", "t1/1.events", "t1/1.intents", "t1/1.json"} {
		x, err := os.ReadFile(filepath.Join(a, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		y, _ := os.ReadFile(filepath.Join(b, name))
		if string(x) != string(y) {
			t.Fatalf("%s differs between two runs of the same configuration", name)
		}
		if len(x) == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
	var sc sidecar
	raw, _ := os.ReadFile(filepath.Join(a, "t1/1.json"))
	if err := json.Unmarshal(raw, &sc); err != nil {
		t.Fatal(err)
	}
	if sc.State != protocol.MatchFinished || sc.Head == "" || sc.Events < 100 || sc.Spectator != "omniscient" || len(sc.Decks) != 4 {
		t.Fatalf("sidecar %+v", sc)
	}
	var tj struct {
		Tables []struct {
			Config TableConfig `json:"config"`
			Match  int         `json:"match"`
		} `json:"tables"`
	}
	raw, _ = os.ReadFile(filepath.Join(a, "tables.json"))
	if err := json.Unmarshal(raw, &tj); err != nil {
		t.Fatal(err)
	}
	if len(tj.Tables) != 1 || tj.Tables[0].Match != 1 || tj.Tables[0].Config.Spectator != view.Omniscient {
		t.Fatalf("tables.json %+v", tj)
	}
	if !strings.Contains(string(raw), `"spectator": "omniscient"`) {
		t.Fatalf("spectator is not serialised as its name:\n%s", raw)
	}
}

func TestAFinishedMatchIsServedFromDiskAfterRestart(t *testing.T) {
	dir := t.TempDir()
	playOneToDisk(t, dir)
	var liveHead string
	{
		r, _ := New(diskOptions(t, dir))
		ms, _ := r.Matches("t1")
		liveHead = ms[0].Head
		r.Close()
	}
	r, err := New(diskOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	tabs := r.Tables()
	if len(tabs) != 1 || tabs[0].Match != 1 || tabs[0].State != protocol.TableIdle {
		t.Fatalf("tables after restart %+v", tabs)
	}
	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 || ms[0].State != protocol.MatchFinished || ms[0].Head != liveHead {
		t.Fatalf("matches after restart %+v, %v", ms, err)
	}
	evs, err := r.Events("t1", 1, 0)
	if err != nil || len(evs) != ms[0].Events {
		t.Fatalf("events after restart: %d, %v", len(evs), err)
	}
	v, err := r.ViewAt("t1", 1, uint64(ms[0].Events-1))
	if err != nil || !v.Over {
		t.Fatalf("ViewAt head after restart: over=%v, %v", v.Over, err)
	}
	mid, err := r.ViewAt("t1", 1, uint64(ms[0].Events/2))
	if err != nil || mid.Over || mid.Turn == 0 {
		t.Fatalf("ViewAt mid after restart: %+v, %v", mid, err)
	}
}

func TestRestartMarksLiveMatchesAbortedAndContinuesAPerpetualTable(t *testing.T) {
	dir := t.TempDir()
	// Simulate a process that died mid-match: write the registry and a live
	// sidecar by hand, the same shape the host writes.
	r, _ := New(diskOptions(t, dir))
	if err := r.AddTable(fourSeatTable("t1", true)); err != nil {
		t.Fatal(err)
	}
	r.Close()
	if err := os.MkdirAll(filepath.Join(dir, "t1"), 0o755); err != nil {
		t.Fatal(err)
	}
	live := sidecar{Table: "t1", Match: 3, Seed: MatchSeed(99, 3), State: protocol.MatchLive, Spectator: "omniscient"}
	raw, _ := json.MarshalIndent(live, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "t1", "3.json"), raw, 0o644)
	_ = os.WriteFile(filepath.Join(dir, "t1", "3.events"), []byte(`{"seq":0,"kind":0,"amount":4}`+"\n"+`{"seq":1,"kind":1,`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "t1", "3.intents"), nil, 0o644)
	rewriteTablesJSON(t, dir, 3)

	stops := 0
	o := diskOptions(t, dir)
	var r2 *Registry
	o.Cooldown = 1
	o.Sleep = func(d time.Duration) {
		if d == 1 {
			stops++
			if stops == 1 {
				go r2.Close()
			}
		}
	}
	r2, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	ms, _ := r2.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchAborted || ms[0].Match != 3 {
		t.Fatalf("after restart: %+v", ms)
	}
	if err := r2.StartAll(); err != nil {
		t.Fatal(err)
	}
	r2.Wait("t1")
	ms, _ = r2.Matches("t1")
	if len(ms) != 2 || ms[1].Match != 4 || ms[1].Seed != MatchSeed(99, 4) || ms[1].State != protocol.MatchFinished {
		t.Fatalf("perpetual table did not continue at k+1: %+v", ms)
	}
	raw, _ = os.ReadFile(filepath.Join(dir, "t1", "3.json"))
	if !strings.Contains(string(raw), `"aborted"`) {
		t.Fatalf("sidecar 3 not rewritten as aborted:\n%s", raw)
	}
}

// rewriteTablesJSON sets the recorded match index for t1.
func rewriteTablesJSON(t *testing.T, dir string, k int) {
	t.Helper()
	p := filepath.Join(dir, "tables.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var tj struct {
		Tables []struct {
			Config TableConfig `json:"config"`
			Match  int         `json:"match"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(raw, &tj); err != nil {
		t.Fatal(err)
	}
	tj.Tables[0].Match = k
	raw, _ = json.MarshalIndent(tj, "", "  ")
	_ = os.WriteFile(p, raw, 0o644)
}

func TestReadLogIgnoresATrailingPartialLine(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "t1"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "t1", "1.events"), []byte(`{"seq":0,"kind":0,"amount":2}`+"\n"+`{"seq":1,"kind":9,"player":1,"amount":1}`+"\n"+`{"seq":2,"ki`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "t1", "1.intents"), []byte(`{"seq":5,"player":0,"choices":[1]}`+"\n"+`{"seq":`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "t1", "1.json"), []byte(`{"table":"t1","match":1,"seed":7,"state":"live"}`), 0o644)
	l, err := readLog(dir, "t1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if l.Seed != 7 || len(l.Events) != 2 || len(l.Intents) != 1 || l.Events[1].Amount != 1 {
		t.Fatalf("log %+v", l)
	}
	if _, err := readLog(dir, "t1", 2); err == nil {
		t.Fatal("missing match read without error")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./host/ -run 'Files|Restart|ReadLog|Disk' -count=1`
Expected: FAIL — `sidecar undefined`.

- [ ] **Step 3: `view.Visibility` as text in JSON**

Append to `view/visibility.go`:

```go
// MarshalText/UnmarshalText make a Visibility its name in JSON and flags,
// so a table configuration on disk reads "omniscient", not 2.
func (v Visibility) MarshalText() ([]byte, error) { return []byte(v.String()), nil }

func (v *Visibility) UnmarshalText(b []byte) error {
	p, err := ParseVisibility(string(b))
	if err != nil {
		return err
	}
	*v = p
	return nil
}
```

and to `view/visibility_test.go`:

```go
func TestVisibilityJSONIsItsName(t *testing.T) {
	b, _ := json.Marshal(struct{ V Visibility }{Omniscient})
	if string(b) != `{"V":"omniscient"}` {
		t.Fatalf("%s", b)
	}
	var back struct{ V Visibility }
	if err := json.Unmarshal([]byte(`{"V":"public"}`), &back); err != nil || back.V != Public {
		t.Fatalf("%v %v", back, err)
	}
	if err := json.Unmarshal([]byte(`{"V":"x"}`), &back); err == nil {
		t.Fatal("unknown name accepted")
	}
}
```

Run: `go test ./view/ -count=1` — PASS.

- [ ] **Step 4: Implement `host/persist.go`**

```go
package host

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
)

// sidecar is <k>.json: everything about a match except its events and
// intents. Rewritten whole at start and at end; never carries a timestamp
// (PL-11), so two runs of one configuration write identical files.
type sidecar struct {
	Table     string              `json:"table"`
	Match     int                 `json:"match"`
	Seed      uint64              `json:"seed"`
	Seats     []protocol.SeatInfo `json:"seats"`
	Names     []string            `json:"names"`
	Decks     []string            `json:"decks"`
	Spectator string              `json:"spectator"`
	State     string              `json:"state"`
	Result    string              `json:"result,omitempty"`
	Winner    *uint8              `json:"winner"`
	Head      string              `json:"head,omitempty"`
	Events    int                 `json:"events"`
	Turns     int32               `json:"turns"`
	Reason    string              `json:"reason,omitempty"`
}

func (sc sidecar) info() protocol.MatchInfo {
	return protocol.MatchInfo{Table: sc.Table, Match: sc.Match, Seed: sc.Seed, Seats: sc.Seats, State: sc.State,
		Result: sc.Result, Winner: sc.Winner, Head: sc.Head, Events: sc.Events, Turns: sc.Turns}
}

// matchFiles are a live match's append-only logs.
type matchFiles struct {
	events, intents *os.File
	sync            bool
}

func tableDir(dir string, t TableID) string { return filepath.Join(dir, string(t)) }

func matchPath(dir string, t TableID, k int, ext string) string {
	return filepath.Join(tableDir(dir, t), strconv.Itoa(k)+ext)
}

// openMatchFiles creates (truncating) a match's two logs.
func openMatchFiles(dir string, t TableID, k int, sync bool) (*matchFiles, error) {
	if err := os.MkdirAll(tableDir(dir, t), 0o755); err != nil {
		return nil, err
	}
	ev, err := os.Create(matchPath(dir, t, k, ".events"))
	if err != nil {
		return nil, err
	}
	in, err := os.Create(matchPath(dir, t, k, ".intents"))
	if err != nil {
		ev.Close()
		return nil, err
	}
	return &matchFiles{events: ev, intents: in, sync: sync}, nil
}

// append writes one burst: the new events and the intent that produced
// them (nil for genesis), one JSON object per line, then fsyncs when
// configured (PL-13).
func (f *matchFiles) append(evs []events.Event, in *decision.Intent) error {
	w := bufio.NewWriter(f.events)
	for _, e := range evs {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		w.Write(b)
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		if _, err := f.intents.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	if f.sync {
		if err := f.events.Sync(); err != nil {
			return err
		}
		if err := f.intents.Sync(); err != nil {
			return err
		}
	}
	return nil
}

func (f *matchFiles) close() {
	if f == nil {
		return
	}
	f.events.Close()
	f.intents.Close()
}

// writeSidecar writes <k>.json atomically (temp file + rename).
func writeSidecar(dir string, sc sidecar) error {
	if err := os.MkdirAll(tableDir(dir, TableID(sc.Table)), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return err
	}
	p := matchPath(dir, TableID(sc.Table), sc.Match, ".json")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func readSidecar(dir string, t TableID, k int) (sidecar, error) {
	var sc sidecar
	raw, err := os.ReadFile(matchPath(dir, t, k, ".json"))
	if err != nil {
		return sc, err
	}
	if err := json.Unmarshal(raw, &sc); err != nil {
		return sc, fmt.Errorf("host: %s/%d.json: %w", t, k, err)
	}
	return sc, nil
}

// readSidecars lists a table's matches from its sidecars, ascending by k.
func readSidecars(dir string, t TableID) ([]sidecar, error) {
	entries, err := os.ReadDir(tableDir(dir, t))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []sidecar
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		k, err := strconv.Atoi(name[:len(name)-len(".json")])
		if err != nil {
			continue
		}
		sc, err := readSidecar(dir, t, k)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Match < out[j].Match })
	return out, nil
}

// readLog rebuilds a match's events.Log from its files. A trailing partial
// line (a crash mid-write) is ignored; anything malformed before it is an
// error. The Seed comes from the sidecar.
func readLog(dir string, t TableID, k int) (*events.Log, error) {
	sc, err := readSidecar(dir, t, k)
	if err != nil {
		return nil, err
	}
	l := events.NewLog(sc.Seed)
	if err := readLines(matchPath(dir, t, k, ".events"), func(b []byte) error {
		var e events.Event
		if err := json.Unmarshal(b, &e); err != nil {
			return err
		}
		l.Append(e)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := readLines(matchPath(dir, t, k, ".intents"), func(b []byte) error {
		var in decision.Intent
		if err := json.Unmarshal(b, &in); err != nil {
			return err
		}
		l.Intents = append(l.Intents, in)
		return nil
	}); err != nil {
		return nil, err
	}
	return l, nil
}

// readLines calls fn per complete, newline-terminated record. A file that
// ends with '\n' splits into a trailing empty element; one cut off
// mid-write ends with a partial record. Either way the final element is
// not a complete record and is dropped.
func readLines(path string, fn func([]byte) error) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := bytes.Split(raw, []byte{'\n'})
	for _, line := range lines[:len(lines)-1] {
		if len(line) == 0 {
			continue
		}
		if err := fn(line); err != nil {
			return fmt.Errorf("host: %s: %w", path, err)
		}
	}
	return nil
}

// tablesFile is tables.json.
type tablesFile struct {
	Tables []tableRecord `json:"tables"`
}

type tableRecord struct {
	Config TableConfig `json:"config"`
	Match  int         `json:"match"`
}

// save writes tables.json (sorted by id) when persistence is on. Called
// with r.mu held.
func (r *Registry) save() error {
	if r.opts.Dir == "" {
		return nil
	}
	var tf tablesFile
	ids := make([]string, 0, len(r.tables))
	for id := range r.tables {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		t := r.tables[TableID(id)]
		t.mu.RLock()
		tf.Tables = append(tf.Tables, tableRecord{Config: t.cfg, Match: t.k})
		t.mu.RUnlock()
	}
	if err := os.MkdirAll(r.opts.Dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(r.opts.Dir, "tables.json")
	if err := os.WriteFile(p+".tmp", append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}
```

Note `Registry.save` is called by `AddTable` with `r.mu` held, and by the match loop when `t.k` changes (below) — the loop must take `r.mu` itself before calling it (add a `saveLocked` split: `save()` takes the lock, `saveLocked()` assumes it; `AddTable` calls `saveLocked`).

- [ ] **Step 5: Implement `host/restart.go` and wire the loop**

```go
package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/protocol"
)

// load reads tables.json and every sidecar. A match still marked live was
// cut off by a crash or kill: it is rewritten as aborted (spec: restart
// aborts in-progress matches; resume is M5). Perpetual tables are left
// ready for StartAll to begin match k+1.
func (r *Registry) load() error {
	raw, err := os.ReadFile(filepath.Join(r.opts.Dir, "tables.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var tf tablesFile
	if err := json.Unmarshal(raw, &tf); err != nil {
		return fmt.Errorf("host: tables.json: %w", err)
	}
	for _, rec := range tf.Tables {
		if err := rec.Config.validate(r.opts.LoadDeck); err != nil {
			return err
		}
		t := newTable(rec.Config)
		t.k = rec.Match
		scs, err := readSidecars(r.opts.Dir, rec.Config.ID)
		if err != nil {
			return err
		}
		for _, sc := range scs {
			if sc.State == protocol.MatchLive {
				sc.State = protocol.MatchAborted
				if err := writeSidecar(r.opts.Dir, sc); err != nil {
					return err
				}
			}
			t.archived = append(t.archived, sc)
			if sc.Match > t.k {
				t.k = sc.Match
			}
		}
		r.tables[rec.Config.ID] = t
	}
	return nil
}
```

Add to `table` a field `archived []sidecar // finished matches known only from disk, ascending` and to `Matches` a merge: archived sidecars first (via `sc.info()`), then in-memory `history` entries whose `k` is not already archived, then `cur`.

Wire the loop (`match.go`, `snapshot.go`):
- Add `files *matchFiles` to `match`.
- `newMatch` — after building `m`, when `r.opts.Dir != ""`: `m.files, err = openMatchFiles(r.opts.Dir, t.cfg.ID, k, r.opts.Sync)`; write the sidecar with `State: MatchLive` via `writeSidecar(r.opts.Dir, m.sidecar())`; append genesis events with `m.files.append(e.L.Events, nil)`. Any error is returned from `newMatch` (the table halts).
- Replace Task 11's `persistBurst` stub in `snapshot.go` with:

```go
// persistBurst appends the burst and its intent (PL-2). Called with m.mu
// held; an error propagates through afterBurst to play, which crashes the
// match (D15) — a table whose files cannot be written must not keep going.
func (r *Registry) persistBurst(t *table, m *match, before int) error {
	if m.files == nil {
		return nil
	}
	in := m.e.L.Intents[len(m.e.L.Intents)-1]
	return m.files.append(m.e.L.Events[before:], &in)
}
```
- `onMatchEnd` (in `fanout.go`) — before fanning out: `r.archive(t, m)`:

```go
// archive writes the final sidecar, closes the files, and drops the
// engine and snapshots: a finished match is served from disk (spec:
// snapshots dropped when the match finishes). In memory mode the engine is
// kept so ViewAt still works.
func (r *Registry) archive(t *table, m *match) {
	if r.opts.Dir == "" {
		return
	}
	m.mu.Lock()
	sc := m.sidecar()
	m.files.close()
	m.files = nil
	m.snaps = nil
	m.mu.Unlock()
	if err := writeSidecar(r.opts.Dir, sc); err != nil {
		m.mu.Lock()
		m.reason = "sidecar: " + err.Error()
		m.mu.Unlock()
	}
	t.mu.Lock()
	t.archived = append(t.archived, sc)
	t.mu.Unlock()
	r.mu.Lock()
	r.saveLocked()
	r.mu.Unlock()
}
```

and `m.sidecar()`:

```go
func (m *match) sidecar() sidecar {
	return sidecar{Table: string(m.table.cfg.ID), Match: m.k, Seed: m.seed, Seats: m.seats, Names: m.cfg.Names,
		Decks: m.decks, Spectator: m.table.cfg.Spectator.String(), State: m.state, Result: m.result, Winner: m.winner,
		Head: m.head, Events: len(m.e.L.Events), Turns: m.e.G.Turn, Reason: m.reason}
}
```

In `run`, after `t.k, t.cur, t.state = …`, call `r.mu.Lock(); r.saveLocked(); r.mu.Unlock()` so `tables.json` records the new match index before it plays.

`lookup` (in `viewat.go`) gains a disk path: when `t.cur`/`history` do not hold `k` but `t.archived` does, build a transient match from disk:

```go
// loadArchived rebuilds enough of a finished match to serve ViewAt/Events:
// the log from its files and a Config from the sidecar's names and decks.
// The engine is the replay's end state, so head views are exact and mid
// views take the genesis path of viewAt (no snapshots on disk).
func (r *Registry) loadArchived(t *table, sc sidecar) (*match, error) {
	l, err := readLog(r.opts.Dir, t.cfg.ID, sc.Match)
	if err != nil {
		return nil, err
	}
	decks := make([][]*cards.Card, len(sc.Decks))
	for i, dn := range sc.Decks {
		d, err := r.opts.LoadDeck(dn)
		if err != nil {
			return nil, fmt.Errorf("host: %s/%d: deck %q: %w", t.cfg.ID, sc.Match, dn, err)
		}
		decks[i] = d.Cards
	}
	cfg := rules.Config{Seed: sc.Seed, Names: sc.Names, Decks: decks}
	e, err := replay.Replay(l, cfg)
	if err != nil {
		return nil, fmt.Errorf("host: %s/%d does not replay: %w", t.cfg.ID, sc.Match, err)
	}
	return &match{table: t, k: sc.Match, seed: sc.Seed, cfg: cfg, seats: sc.Seats, decks: sc.Decks, e: e,
		bounds: boundsOf(l.Events), turnStarts: turnStartsIn(l.Events, 0), state: sc.State, result: sc.Result,
		winner: sc.Winner, head: sc.Head}, nil
}
```

`lookup` returns the loaded match; cache the last loaded one per table (`t.loaded *match`) so a DVR session stepping through a finished match does not replay it per request. `Matches` for archived entries uses `sc.info()`.

An aborted match (Head recorded, log shorter than a full game) replays fine: `replay.Replay` compares the events that exist.

- [ ] **Step 6: Run the tests with `-race`**

Run: `go test -race ./host/ -count=1 -v`
Expected: PASS (all host tests, including Tasks 9–11's — the in-memory paths must still work with `Dir == ""`).

- [ ] **Step 7: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./...`

```bash
git add host/ view/visibility.go view/visibility_test.go
git commit -m "feat(host): append-only persistence, sidecars, tables.json, restart

Two runs of one configuration write byte-identical files; a restart marks
live matches aborted, serves finished ones from disk, and continues a
perpetual table at k+1.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 13: `host` crash handling — D15

**Files:**
- Create: `host/crash.go`, `host/crash_test.go`
- Modify: `host/match.go` (`crash` writes the report), `host/registry.go` (`halt` sends the frame; no restart)

**Interfaces:**
- Consumes: Task 9's `crash`/`halt`, Task 10's `sendHalted`, Task 12's `Dir`.
- Produces: `<Dir>/crash/<table>-<k>.txt` with reason, chain head, seq, and stack; `TableInfo.State == halted`; `table_halted` frame; the sidecar's `state: crashed` and `reason`.

- [ ] **Step 1: Write the failing tests**

Create `host/crash_test.go`:

```go
package host

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// faultySeat wraps a bot and misbehaves at decision n: it panics, or
// returns an intent the engine must reject.
type faultySeat struct {
	inner seat.Seat
	at    int
	n     *int
	mode  string
}

func (f *faultySeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	*f.n++
	if *f.n == f.at {
		switch f.mode {
		case "panic":
			panic("seat exploded on purpose")
		case "reject":
			return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{len(d.Options) + 5}}, nil
		}
	}
	return f.inner.Decide(ctx, v, d)
}

func faultyOptions(t *testing.T, dir, mode string) Options {
	o := testOptions(t)
	o.Dir = dir
	o.Seats = func(names []string, seed uint64) []seat.Seat {
		n := 0
		out := make([]seat.Seat, len(names))
		for i := range names {
			out[i] = &faultySeat{inner: seat.NewBot(seed ^ uint64(i+1)), at: 25, n: &n, mode: mode}
		}
		return out
	}
	return o
}

func TestAPanickingSeatCrashesTheMatchAndHaltsTheTable(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(faultyOptions(t, dir, "panic"))
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", true)) // perpetual: must NOT restart
	s := r.OpenSession()
	_ = r.Subscribe(s, "t1", protocol.ModeFocus)
	_ = r.Start("t1")
	r.Wait("t1")

	tab := r.Tables()[0]
	if tab.State != protocol.TableHalted || tab.Match != 1 {
		t.Fatalf("table %+v", tab)
	}
	ms, _ := r.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchCrashed {
		t.Fatalf("matches %+v", ms)
	}
	report, err := os.ReadFile(filepath.Join(dir, "crash", "t1-1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"seat exploded on purpose", "head:", "seq:", "goroutine"} {
		if !strings.Contains(string(report), want) {
			t.Fatalf("crash report lacks %q:\n%s", want, report)
		}
	}
	sc, _ := readSidecar(dir, "t1", 1)
	if sc.State != protocol.MatchCrashed || !strings.Contains(sc.Reason, "panic") {
		t.Fatalf("sidecar %+v", sc)
	}
	var halted, ended bool
	for _, f := range drainNow(s) {
		switch f.T {
		case protocol.TTableHalted:
			halted = true
		case protocol.TMatchEnd:
			ended = true
		}
	}
	if !halted || !ended {
		t.Fatalf("frames: halted=%v match_end=%v", halted, ended)
	}
}

func TestARejectedIntentCrashesTheMatch(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(faultyOptions(t, dir, "reject"))
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	_ = r.Start("t1")
	r.Wait("t1")
	ms, _ := r.Matches("t1")
	if ms[0].State != protocol.MatchCrashed {
		t.Fatalf("%+v", ms)
	}
	sc, _ := readSidecar(dir, "t1", 1)
	if !strings.Contains(sc.Reason, "rejected") || !strings.Contains(sc.Reason, "out of range") {
		t.Fatalf("reason %q", sc.Reason)
	}
	if r.Tables()[0].State != protocol.TableHalted {
		t.Fatal("table not halted")
	}
	if _, err := os.Stat(filepath.Join(dir, "crash", "t1-1.txt")); err != nil {
		t.Fatal("no crash report for a rejected intent")
	}
}

func TestANonTerminatingMatchIsACrash(t *testing.T) {
	o := testOptions(t)
	o.MaxIntents = 50
	r, _ := New(o)
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	_ = r.Start("t1")
	r.Wait("t1")
	ms, _ := r.Matches("t1")
	if ms[0].State != protocol.MatchCrashed || r.Tables()[0].State != protocol.TableHalted {
		t.Fatalf("%+v / %+v", ms, r.Tables())
	}
}

func TestACrashedMatchStillReplaysFromDisk(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(faultyOptions(t, dir, "panic"))
	_ = r.AddTable(fourSeatTable("t1", false))
	_ = r.Start("t1")
	r.Wait("t1")
	r.Close()
	r2, err := New(diskOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	ms, _ := r2.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchCrashed {
		t.Fatalf("%+v", ms)
	}
	if _, err := r2.ViewAt("t1", 1, uint64(ms[0].Events-1)); err != nil {
		t.Fatalf("crashed match does not replay: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./host/ -run 'Crash|Panick|Rejected|NonTerminating' -count=1`
Expected: FAIL — no crash report file / table not halted.

- [ ] **Step 3: Implement `host/crash.go` and wire it**

```go
package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// writeCrashReport is spec D15's evidence: what the match was doing when
// it died, enough to reproduce from the files beside it. Best effort — a
// failure to write the report must not mask the crash itself.
func (r *Registry) writeCrashReport(t *table, m *match, reason string) {
	if r.opts.Dir == "" {
		return
	}
	dir := filepath.Join(r.opts.Dir, "crash")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	m.mu.RLock()
	body := fmt.Sprintf("table: %s\nmatch: %d\nseed: %d\nhead: %s\nseq: %d\nintents: %d\nturn: %d\nreason:\n%s\n",
		t.cfg.ID, m.k, m.seed, m.e.L.Head(), len(m.e.L.Events)-1, m.intents, m.e.G.Turn, reason)
	m.mu.RUnlock()
	_ = os.WriteFile(filepath.Join(dir, string(t.cfg.ID)+"-"+strconv.Itoa(m.k)+".txt"), []byte(body), 0o644)
}
```

In `match.go`'s `crash`, after setting the state and before `onMatchEnd`: `r.writeCrashReport(t, m, err.Error())`. The panic reason already includes `debug.Stack()` (so "goroutine" appears in the report); for a rejected intent and non-termination it is the error text.

In `registry.go`'s `halt`: keep the state change, then `r.sendHalted(t, k, err.Error())` where `k` is read under `t.mu`. `run` already returns after `halt` without restarting — the perpetual test in Step 1 pins that.

- [ ] **Step 4: Run all host tests with `-race`**

Run: `go test -race ./host/ -count=1`
Expected: PASS.

- [ ] **Step 5: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./...`

```bash
git add host/
git commit -m "feat(host): a crashed match halts its table with a report, never restarts

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

## Phase 3 — HTTP and the binary

### Task 14: `host/httpapi` — handler skeleton, JSON GETs, POSTs, errors

**Files:**
- Create: `host/httpapi/handler.go`, `host/httpapi/rest.go`, `host/httpapi/errors.go`, `host/httpapi/rest_test.go`

**Interfaces:**
- Consumes: `host.Registry` (`Tables`, `Matches`, `ViewAt`, `Events`, `Session`, `Subscribe`, `Unsubscribe`), `host.ErrNotFound`, `host.ErrBeyondHead`, `protocol.*`.
- Produces:

```go
type Options struct {
	Authorize      func(*http.Request) error // nil allows everyone; an error is a 401
	WidgetInterval time.Duration             // SSE widget flush cadence; 0 = 250ms (Task 15)
	KeepAlive      time.Duration             // SSE comment cadence; 0 = 15s (Task 15)
	ResumeGrace    time.Duration             // how long a disconnected session survives; 0 = 30s (Task 15)
	Web            fs.FS                     // the built client (has index.html); nil = 503 for non-API paths (Task 15)
}
func NewHandler(r *host.Registry, o Options) http.Handler
```

Routes: `GET /api/tables`, `GET /api/tables/{t}/matches`, `GET /api/tables/{t}/matches/{k}/view?seq=N` (no `seq` = head), `GET /api/tables/{t}/matches/{k}/events?since=N` (default 0), `POST /api/subscribe`, `POST /api/unsubscribe`; Task 15 adds `GET /api/stream` and the static SPA. Every error reply is a `protocol.ErrorBody`: 400 `bad_request`, 401 `unauthorized`, 404 `not_found`, 405 `method_not_allowed`, 409 `beyond_head` (with `head`), 500 `internal`.

- [ ] **Step 1: Write the failing tests**

Create `host/httpapi/rest_test.go`:

```go
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

func loader(t *testing.T) func(string) (host.Deck, error) {
	t.Helper()
	names, decks := testutil.SampleDecks(t, 4)
	by := map[string][]*cards.Card{}
	for i, n := range names {
		by[n] = decks[i]
	}
	return func(n string) (host.Deck, error) {
		cs, ok := by[n]
		if !ok {
			return host.Deck{}, host.ErrNotFound
		}
		return host.Deck{Name: n, Cards: cs}, nil
	}
}

// finishedServer plays one 4-seat match to completion and serves it.
func finishedServer(t *testing.T, o Options) (*httptest.Server, *host.Registry) {
	t.Helper()
	r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: "t1", Name: "Table 1", Seats: 4, Decks: []string{"a", "b", "c", "d"}, Seed: 5, Spectator: view.Omniscient}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	srv := httptest.NewServer(NewHandler(r, o))
	t.Cleanup(srv.Close)
	return srv, r
}

func getJSON(t *testing.T, url string, into any) (int, protocol.ErrorBody) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("%s: content-type %q", url, resp.Header.Get("Content-Type"))
	}
	if resp.StatusCode >= 400 {
		var e protocol.ErrorBody
		if err := json.Unmarshal(body, &e); err != nil {
			t.Fatalf("%s: non-JSON error body %q", url, body)
		}
		return resp.StatusCode, e
	}
	if into != nil {
		if err := json.Unmarshal(body, into); err != nil {
			t.Fatalf("%s: %v\n%s", url, err, body)
		}
	}
	return resp.StatusCode, protocol.ErrorBody{}
}

func postJSON(t *testing.T, url string, body any) (int, protocol.ErrorBody) {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e protocol.ErrorBody
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return resp.StatusCode, e
	}
	return resp.StatusCode, protocol.ErrorBody{}
}

func TestTablesAndMatches(t *testing.T) {
	srv, _ := finishedServer(t, Options{})
	var tables []protocol.TableInfo
	if code, _ := getJSON(t, srv.URL+"/api/tables", &tables); code != 200 || len(tables) != 1 || tables[0].ID != "t1" {
		t.Fatalf("%d %+v", code, tables)
	}
	var ms []protocol.MatchInfo
	if code, _ := getJSON(t, srv.URL+"/api/tables/t1/matches", &ms); code != 200 || len(ms) != 1 || ms[0].State != protocol.MatchFinished {
		t.Fatalf("%d %+v", code, ms)
	}
	if code, e := getJSON(t, srv.URL+"/api/tables/t9/matches", nil); code != 404 || e.Code != "not_found" {
		t.Fatalf("%d %+v", code, e)
	}
}

func TestViewAndEvents(t *testing.T) {
	srv, r := finishedServer(t, Options{})
	ms, _ := r.Matches("t1")
	head := uint64(ms[0].Events - 1)
	var v view.View
	if code, _ := getJSON(t, srv.URL+"/api/tables/t1/matches/1/view", &v); code != 200 || !v.Over || v.Visibility != "omniscient" {
		t.Fatalf("view at head: %d %+v", code, v)
	}
	if code, _ := getJSON(t, fmt.Sprintf("%s/api/tables/t1/matches/1/view?seq=%d", srv.URL, head/2), &v); code != 200 || v.Over {
		t.Fatalf("view mid: %d over=%v", code, v.Over)
	}
	code, e := getJSON(t, fmt.Sprintf("%s/api/tables/t1/matches/1/view?seq=%d", srv.URL, head+1), nil)
	if code != 409 || e.Code != "beyond_head" || e.Head != head {
		t.Fatalf("beyond head: %d %+v", code, e)
	}
	if code, e := getJSON(t, srv.URL+"/api/tables/t1/matches/1/view?seq=abc", nil); code != 400 || e.Code != "bad_request" {
		t.Fatalf("bad seq: %d %+v", code, e)
	}
	if code, _ := getJSON(t, srv.URL+"/api/tables/t1/matches/x/view", nil); code != 400 {
		t.Fatalf("bad k: %d", code)
	}
	if code, _ := getJSON(t, srv.URL+"/api/tables/t1/matches/2/view", nil); code != 404 {
		t.Fatalf("unknown match: %d", code)
	}
	var evs []protocol.EventBody
	if code, _ := getJSON(t, fmt.Sprintf("%s/api/tables/t1/matches/1/events?since=%d", srv.URL, head-2), &evs); code != 200 || len(evs) != 3 || evs[0].Event.Seq != head-2 {
		t.Fatalf("events tail: %d %+v", code, evs)
	}
	if code, _ := getJSON(t, srv.URL+"/api/tables/t1/matches/1/events", &evs); code != 200 || len(evs) != int(head)+1 {
		t.Fatalf("all events: %d %d", code, len(evs))
	}
	if code, e := getJSON(t, fmt.Sprintf("%s/api/tables/t1/matches/1/events?since=%d", srv.URL, head+1), nil); code != 409 || e.Head != head {
		t.Fatalf("events beyond head: %d %+v", code, e)
	}
}

func TestSubscribeAndUnsubscribe(t *testing.T) {
	srv, r := finishedServer(t, Options{})
	s := r.OpenSession()
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: s.ID, Table: "*", Mode: protocol.ModeOverview}); code != 204 {
		t.Fatalf("subscribe *: %d", code)
	}
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: s.ID, Table: "t1", Mode: protocol.ModeFocus}); code != 204 {
		t.Fatalf("subscribe t1: %d", code)
	}
	if code, e := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: "s999", Table: "t1", Mode: protocol.ModeFocus}); code != 404 || e.Code != "not_found" {
		t.Fatalf("unknown session: %d %+v", code, e)
	}
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: s.ID, Table: "t1", Mode: "sideways"}); code != 400 {
		t.Fatalf("bad mode: %d", code)
	}
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: s.ID, Table: "t9", Mode: protocol.ModeFocus}); code != 404 {
		t.Fatalf("unknown table: %d", code)
	}
	if code, _ := postJSON(t, srv.URL+"/api/unsubscribe", protocol.Unsubscribe{Session: s.ID, Table: "t1"}); code != 204 {
		t.Fatalf("unsubscribe: %d", code)
	}
	if code, _ := postJSON(t, srv.URL+"/api/unsubscribe", protocol.Unsubscribe{Session: s.ID, Table: "t1"}); code != 404 {
		t.Fatalf("unsubscribe twice: %d", code)
	}
	resp, _ := http.Post(srv.URL+"/api/subscribe", "application/json", bytes.NewReader([]byte("{")))
	if resp.StatusCode != 400 {
		t.Fatalf("malformed JSON: %d", resp.StatusCode)
	}
	resp, _ = http.Get(srv.URL + "/api/subscribe")
	if resp.StatusCode != 405 {
		t.Fatalf("GET subscribe: %d", resp.StatusCode)
	}
}

func TestAuthorizeGatesEveryRoute(t *testing.T) {
	denied := errors.New("no")
	srv, _ := finishedServer(t, Options{Authorize: func(r *http.Request) error {
		if r.Header.Get("X-Ok") == "1" {
			return nil
		}
		return denied
	}})
	for _, path := range []string{"/api/tables", "/api/tables/t1/matches", "/api/tables/t1/matches/1/view", "/api/tables/t1/matches/1/events"} {
		if code, e := getJSON(t, srv.URL+path, nil); code != 401 || e.Code != "unauthorized" {
			t.Fatalf("%s: %d %+v", path, code, e)
		}
	}
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{}); code != 401 {
		t.Fatalf("subscribe: %d", code)
	}
	req, _ := http.NewRequest("GET", srv.URL+"/api/tables", nil)
	req.Header.Set("X-Ok", "1")
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		t.Fatalf("authorised request: %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./host/httpapi/ -count=1`
Expected: FAIL — no non-test files.

- [ ] **Step 3: Implement `errors.go`, `handler.go`, `rest.go`**

`host/httpapi/errors.go`:

```go
// Package httpapi serves a host.Registry over plain net/http: JSON GETs for
// tables, matches, views and events; POSTs for subscriptions; one SSE
// stream per client; and the embedded web client. cmd/gorged mounts it;
// mtgserve can too (spec D9, PL-4).
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
)

// writeJSON writes any reply; a marshal failure of our own types is a bug
// and surfaces as a 500 with the message.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers are gone; the best we can do is log-free silence — the
		// client sees a truncated body and its decoder fails.
		return
	}
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, protocol.ErrorBody{Code: code, Message: msg})
}

// writeHostError maps the host's errors onto statuses: not found 404, seq
// beyond head 409 with the head in the body, anything else 500.
func writeHostError(w http.ResponseWriter, err error) {
	var beyond host.ErrBeyondHead
	switch {
	case errors.Is(err, host.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.As(err, &beyond):
		writeJSON(w, http.StatusConflict, protocol.ErrorBody{Code: "beyond_head", Message: err.Error(), Head: beyond.Head})
	default:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
	}
}
```

`host/httpapi/handler.go`:

```go
package httpapi

import (
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/host"
)

// Options configures the handler. Every duration defaults when zero.
type Options struct {
	Authorize      func(*http.Request) error
	WidgetInterval time.Duration
	KeepAlive      time.Duration
	ResumeGrace    time.Duration
	Web            fs.FS
}

func (o Options) withDefaults() Options {
	if o.WidgetInterval == 0 {
		o.WidgetInterval = 250 * time.Millisecond
	}
	if o.KeepAlive == 0 {
		o.KeepAlive = 15 * time.Second
	}
	if o.ResumeGrace == 0 {
		o.ResumeGrace = 30 * time.Second
	}
	return o
}

type handler struct {
	reg  *host.Registry
	opts Options

	mu     sync.Mutex
	grace  map[string]*time.Timer // session id -> pending close after disconnect (Task 15)
}

// NewHandler routes the API and, when Options.Web is set, the client.
func NewHandler(r *host.Registry, o Options) http.Handler {
	h := &handler{reg: r, opts: o.withDefaults(), grace: map[string]*time.Timer{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tables", h.tables)
	mux.HandleFunc("GET /api/tables/{t}/matches", h.matches)
	mux.HandleFunc("GET /api/tables/{t}/matches/{k}/view", h.view)
	mux.HandleFunc("GET /api/tables/{t}/matches/{k}/events", h.events)
	mux.HandleFunc("POST /api/subscribe", h.subscribe)
	mux.HandleFunc("POST /api/unsubscribe", h.unsubscribe)
	// Method-less twins of every API pattern: the mux prefers the
	// method-specific pattern, so these only ever see the wrong method and
	// answer 405 in JSON rather than the mux's default text body.
	for _, p := range []string{"/api/tables", "/api/tables/{t}/matches", "/api/tables/{t}/matches/{k}/view",
		"/api/tables/{t}/matches/{k}/events", "/api/subscribe", "/api/unsubscribe", "/api/stream"} {
		mux.HandleFunc(p, methodNotAllowed)
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
	})
	h.mountStream(mux) // Task 15
	h.mountStatic(mux) // Task 15
	return h.authorized(mux)
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", r.Method+" is not allowed here")
}

// authorized runs the hook before every request.
func (h *handler) authorized(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.opts.Authorize != nil {
			if err := h.opts.Authorize(r); err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Stubs Task 15 replaces.
func (h *handler) mountStream(mux *http.ServeMux) {}
func (h *handler) mountStatic(mux *http.ServeMux) {}
```

`host/httpapi/rest.go`:

```go
package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
)

func (h *handler) tables(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.reg.Tables())
}

func (h *handler) matches(w http.ResponseWriter, r *http.Request) {
	ms, err := h.reg.Matches(host.TableID(r.PathValue("t")))
	if err != nil {
		writeHostError(w, err)
		return
	}
	if ms == nil {
		ms = []protocol.MatchInfo{}
	}
	writeJSON(w, http.StatusOK, ms)
}

// matchKey parses {t} and {k}; a non-numeric k is a 400.
func matchKey(w http.ResponseWriter, r *http.Request) (host.TableID, int, bool) {
	k, err := strconv.Atoi(r.PathValue("k"))
	if err != nil || k < 1 {
		writeError(w, http.StatusBadRequest, "bad_request", "match index must be a positive integer")
		return "", 0, false
	}
	return host.TableID(r.PathValue("t")), k, true
}

// uintQuery parses an optional unsigned query parameter.
func uintQuery(w http.ResponseWriter, r *http.Request, name string) (uint64, bool, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, false, true
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", name+" must be a non-negative integer")
		return 0, false, false
	}
	return n, true, true
}

func (h *handler) view(w http.ResponseWriter, r *http.Request) {
	t, k, ok := matchKey(w, r)
	if !ok {
		return
	}
	seq, given, ok := uintQuery(w, r, "seq")
	if !ok {
		return
	}
	if !given {
		ms, err := h.reg.Matches(t)
		if err != nil {
			writeHostError(w, err)
			return
		}
		found := false
		for _, m := range ms {
			if m.Match == k && m.Events > 0 {
				seq, found = uint64(m.Events-1), true
			}
		}
		if !found {
			writeHostError(w, host.ErrNotFound)
			return
		}
	}
	v, err := h.reg.ViewAt(t, k, seq)
	if err != nil {
		writeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *handler) events(w http.ResponseWriter, r *http.Request) {
	t, k, ok := matchKey(w, r)
	if !ok {
		return
	}
	since, _, ok := uintQuery(w, r, "since")
	if !ok {
		return
	}
	evs, err := h.reg.Events(t, k, since)
	if err != nil {
		writeHostError(w, err)
		return
	}
	if evs == nil {
		evs = []protocol.EventBody{}
	}
	writeJSON(w, http.StatusOK, evs)
}

// decodeBody reads a small JSON body; anything malformed is a 400.
func decodeBody(w http.ResponseWriter, r *http.Request, into any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "malformed JSON body: "+err.Error())
		return false
	}
	return true
}

// session resolves the body's session id; unknown is a 404 (it may have
// expired — the client must reconnect the stream to get a new one).
func (h *handler) session(w http.ResponseWriter, id string) (*host.Session, bool) {
	s, ok := h.reg.Session(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "unknown session "+id)
		return nil, false
	}
	return s, true
}

func (h *handler) subscribe(w http.ResponseWriter, r *http.Request) {
	var req protocol.Subscribe
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Mode != protocol.ModeOverview && req.Mode != protocol.ModeFocus {
		writeError(w, http.StatusBadRequest, "bad_request", "mode must be overview or focus")
		return
	}
	s, ok := h.session(w, req.Session)
	if !ok {
		return
	}
	if err := h.reg.Subscribe(s, host.TableID(req.Table), req.Mode); err != nil {
		if err == host.ErrNotFound {
			writeHostError(w, err)
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) unsubscribe(w http.ResponseWriter, r *http.Request) {
	var req protocol.Unsubscribe
	if !decodeBody(w, r, &req) {
		return
	}
	s, ok := h.session(w, req.Session)
	if !ok {
		return
	}
	if err := h.reg.Unsubscribe(s, host.TableID(req.Table)); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./host/httpapi/ -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./...`

```bash
git add host/httpapi/
git commit -m "feat(httpapi): JSON GETs, subscribe/unsubscribe, error mapping, authorizer hook

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 15: `host/httpapi` — the SSE stream, resume, widget ticker, static SPA

**Files:**
- Create: `host/httpapi/sse.go`, `host/httpapi/static.go`, `host/httpapi/sse_test.go`
- Modify: `host/httpapi/handler.go` (remove the two stubs)

**Interfaces:**
- Consumes: `host.Registry.OpenSession/Session/CloseSession/Hello`, `Session.Out/TakeWidgets/Since/Overflowed`.
- Produces: `GET /api/stream` (SSE; `Last-Event-ID: <session>:<frame>` honoured), static files from `Options.Web` with an SPA fallback to `index.html` for every non-`/api/` path, `503 web build missing` when `Web` is nil.

SSE wire format, one frame:

```
id: s3:4182
event: event
data: {"v":1,"t":"event", ...}

```

Widgets have no `id:` line (PL-5); keep-alives are `: ping` comments.

- [ ] **Step 1: Write the failing tests**

Create `host/httpapi/sse_test.go`:

```go
package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

type sseFrame struct {
	id    string
	event string
	frame protocol.Frame
}

// readSSE parses frames until n have arrived or the stream ends.
func readSSE(t *testing.T, body io.Reader, n int) []sseFrame {
	t.Helper()
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	var out []sseFrame
	cur := sseFrame{}
	var data bytes.Buffer
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if data.Len() > 0 {
				if err := json.Unmarshal(data.Bytes(), &cur.frame); err != nil {
					t.Fatalf("bad frame data %q: %v", data.String(), err)
				}
				out = append(out, cur)
				if len(out) == n {
					return out
				}
			}
			cur, data = sseFrame{}, bytes.Buffer{}
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "id: "):
			cur.id = line[4:]
		case strings.HasPrefix(line, "event: "):
			cur.event = line[7:]
		case strings.HasPrefix(line, "data: "):
			data.WriteString(line[6:])
		}
	}
	return out
}

// pausedServer builds a registry whose table waits on `gate` at every
// decision so the test controls pacing, and serves it.
func pausedServer(t *testing.T, o Options, gate chan struct{}, ring int) (*httptest.Server, *host.Registry) {
	t.Helper()
	r, err := host.New(host.Options{LoadDeck: loader(t), Ring: ring, Sleep: func(time.Duration) {
		if gate != nil {
			<-gate
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(host.TableConfig{ID: "t1", Name: "Table 1", Seats: 4, Decks: []string{"a", "b", "c", "d"}, Seed: 5, Spectator: view.Omniscient}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewHandler(r, o))
	t.Cleanup(srv.Close)
	return srv, r
}

func openStream(t *testing.T, ctx context.Context, url, lastID string) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, "GET", url+"/api/stream", nil)
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	return resp
}

func TestStreamOpensWithHelloThenSnapshotThenEvents(t *testing.T) {
	gate := make(chan struct{})
	srv, r := pausedServer(t, Options{}, gate, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := openStream(t, ctx, srv.URL, "")
	defer resp.Body.Close()
	first := readSSE(t, resp.Body, 1)
	if first[0].event != "hello" || first[0].id != "" {
		t.Fatalf("first frame %+v", first[0])
	}
	var hello protocol.Hello
	_ = first[0].frame.Decode(&hello)
	if hello.Session == "" || len(hello.Tables) != 1 {
		t.Fatalf("hello %+v", hello)
	}
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: hello.Session, Table: "t1", Mode: protocol.ModeFocus}); code != 204 {
		t.Fatalf("subscribe %d", code)
	}
	_ = r.Start("t1")
	frames := readSSE(t, resp.Body, 12)
	if frames[0].event != "match_start" || frames[1].event != "snapshot" {
		t.Fatalf("after start: %s, %s", frames[0].event, frames[1].event)
	}
	if frames[1].id != hello.Session+":"+"2" {
		t.Fatalf("snapshot id %q (PL-6 expects <session>:<frame>)", frames[1].id)
	}
	// Release the gate for good and check event seqs stay contiguous over
	// a long stretch of the match.
	close(gate)
	more := readSSE(t, resp.Body, 200)
	var last uint64
	for _, f := range more {
		if f.event == "event" {
			if last != 0 && f.frame.Seq != last+1 {
				t.Fatalf("event seq %d after %d", f.frame.Seq, last)
			}
			last = f.frame.Seq
		}
	}
	if last == 0 {
		t.Fatal("no event frames")
	}
}

func TestLastEventIDResumesExactlyTheMissedFrames(t *testing.T) {
	srv, r := pausedServer(t, Options{ResumeGrace: 5 * time.Second}, nil, 100000)
	ctx, cancel := context.WithCancel(context.Background())
	resp := openStream(t, ctx, srv.URL, "")
	hello := readSSE(t, resp.Body, 1)[0]
	var h protocol.Hello
	_ = hello.frame.Decode(&h)
	_, _ = postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: h.Session, Table: "t1", Mode: protocol.ModeFocus})
	_ = r.Start("t1")
	got := readSSE(t, resp.Body, 30)
	lastID := got[len(got)-1].id
	cancel() // client drops mid-stream
	resp.Body.Close()
	r.Wait("t1") // the match finishes while we are away; the ring keeps the tail

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	resp2 := openStream(t, ctx2, srv.URL, lastID)
	defer resp2.Body.Close()
	resumed := readSSE(t, resp2.Body, 5)
	if resumed[0].event == "hello" {
		t.Fatalf("resume within the ring started over with hello")
	}
	lastNum, err := strconv.ParseUint(strings.TrimPrefix(lastID, h.Session+":"), 10, 64)
	if err != nil {
		t.Fatalf("last id %q: %v", lastID, err)
	}
	if resumed[0].frame.ID != lastNum+1 {
		t.Fatalf("first resumed frame id %d, want %d", resumed[0].frame.ID, lastNum+1)
	}
	for i := 1; i < len(resumed); i++ {
		if resumed[i].frame.ID != resumed[i-1].frame.ID+1 {
			t.Fatalf("resumed ids not contiguous: %d after %d", resumed[i].frame.ID, resumed[i-1].frame.ID)
		}
	}
}

func TestAnIDOlderThanTheRingStartsOverWithHello(t *testing.T) {
	srv, r := pausedServer(t, Options{}, nil, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := openStream(t, ctx, srv.URL, "")
	var h protocol.Hello
	_ = readSSE(t, resp.Body, 1)[0].frame.Decode(&h)
	_, _ = postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: h.Session, Table: "t1", Mode: protocol.ModeFocus})
	_ = r.Start("t1")
	r.Wait("t1")
	cancel()
	resp.Body.Close()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	resp2 := openStream(t, ctx2, srv.URL, h.Session+":1")
	defer resp2.Body.Close()
	first := readSSE(t, resp2.Body, 1)[0]
	var h2 protocol.Hello
	if first.event != "hello" || first.frame.Decode(&h2) != nil || h2.Session == h.Session {
		t.Fatalf("expected a fresh hello with a new session, got %s %+v", first.event, h2)
	}
	resp3 := openStream(t, ctx2, srv.URL, "s999:5")
	defer resp3.Body.Close()
	if f := readSSE(t, resp3.Body, 1)[0]; f.event != "hello" {
		t.Fatalf("unknown session resume: %s", f.event)
	}
}

func TestWidgetsAreCoalescedToTheTicker(t *testing.T) {
	srv, r := pausedServer(t, Options{WidgetInterval: 40 * time.Millisecond}, nil, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp := openStream(t, ctx, srv.URL, "")
	var h protocol.Hello
	_ = readSSE(t, resp.Body, 1)[0].frame.Decode(&h)
	_, _ = postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: h.Session, Table: "*", Mode: protocol.ModeOverview})
	start := time.Now()
	_ = r.Start("t1")
	r.Wait("t1")
	elapsed := time.Since(start)
	// Read for 300ms after the match ends, then count what arrived.
	deadline := time.After(300 * time.Millisecond)
	widgets, withID := 0, 0
	done := make(chan []sseFrame)
	go func() { done <- readSSE(t, resp.Body, 1000) }()
	select {
	case fs := <-done:
		for _, f := range fs {
			if f.event == "widget" {
				widgets++
				if f.id != "" {
					withID++
				}
			}
		}
	case <-deadline:
		cancel()
		fs := <-done
		for _, f := range fs {
			if f.event == "widget" {
				widgets++
				if f.id != "" {
					withID++
				}
			}
		}
	}
	ms, _ := r.Matches("t1")
	if widgets == 0 || withID != 0 {
		t.Fatalf("%d widgets, %d with ids", widgets, withID)
	}
	maxTicks := int(elapsed/(40*time.Millisecond)) + 10
	if widgets > maxTicks {
		t.Fatalf("%d widgets for a %v match with a 40ms ticker (%d decisions)", widgets, elapsed, ms[0].Events)
	}
}

func TestStaticServesTheClientWithSPAFallbackOr503(t *testing.T) {
	web := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>gorge</title>")}, "assets/app.js": {Data: []byte("console.log(1)")}}
	srv, _ := pausedServer(t, Options{Web: web}, nil, 0)
	for _, p := range []string{"/", "/t/t1", "/t/t1/m/3"} {
		resp, _ := http.Get(srv.URL + p)
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 || !strings.Contains(string(body), "<title>gorge</title>") {
			t.Fatalf("%s: %d %q", p, resp.StatusCode, body)
		}
	}
	resp, _ := http.Get(srv.URL + "/assets/app.js")
	if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Type"), "javascript") {
		t.Fatalf("asset: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	resp, _ = http.Get(srv.URL + "/api/nope")
	if resp.StatusCode != 404 || resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("api 404: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	srv2, _ := pausedServer(t, Options{}, nil, 0)
	resp, _ = http.Get(srv2.URL + "/")
	if resp.StatusCode != 503 {
		t.Fatalf("no web build: %d", resp.StatusCode)
	}
}
```

`pausedServer`'s fourth argument is the registry's ring size (0 = default): the resume test must keep the whole tail of the match inside the ring, so it passes `100000`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./host/httpapi/ -run 'Stream|LastEventID|OlderThan|Widgets|Static' -count=1`
Expected: FAIL — `/api/stream` returns 404.

- [ ] **Step 3: Implement `sse.go`**

```go
package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
)

func (h *handler) mountStream(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/stream", h.stream)
}

// stream is one SSE connection. A Last-Event-ID of "<session>:<frame>"
// resumes that session from its ring; anything else — no header, an
// unknown session, an id older than the ring — opens a fresh session and
// starts with hello (the client then re-subscribes).
func (h *handler) stream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "streaming unsupported")
		return
	}
	s, backlog := h.resume(r.Header.Get("Last-Event-ID"))
	fresh := s == nil
	if fresh {
		s = h.reg.OpenSession()
	}
	h.cancelGrace(s.ID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	write := func(f protocol.Frame) error {
		return writeFrame(w, s.ID, f)
	}
	if fresh {
		if err := write(h.reg.Hello(s)); err != nil {
			return
		}
	}
	for _, f := range backlog {
		if err := write(f); err != nil {
			return
		}
	}
	fl.Flush()

	widgets := time.NewTicker(h.opts.WidgetInterval)
	defer widgets.Stop()
	keep := time.NewTicker(h.opts.KeepAlive)
	defer keep.Stop()
	out := s.Out()
	for {
		select {
		case <-r.Context().Done():
			h.scheduleGrace(s.ID)
			return
		case f, open := <-out:
			if !open {
				if dropped, of := s.Overflowed(); of {
					ov, _ := protocol.NewFrame(protocol.TOverflow, "", 0, 0, protocol.Overflow{Dropped: dropped})
					_ = write(ov)
					fl.Flush()
				}
				return // closed by overflow or CloseSession: the client reconnects
			}
			if err := write(f); err != nil {
				h.scheduleGrace(s.ID)
				return
			}
			// Drain whatever else is ready before flushing once.
			for drained := false; !drained; {
				select {
				case f, open := <-out:
					if !open {
						drained = true
						out = nil // let the closed case run on the next loop
						continue
					}
					if err := write(f); err != nil {
						h.scheduleGrace(s.ID)
						return
					}
				default:
					drained = true
				}
			}
			fl.Flush()
		case <-widgets.C:
			ws := s.TakeWidgets()
			for _, f := range ws {
				if err := writeFrame(w, "", f); err != nil {
					h.scheduleGrace(s.ID)
					return
				}
			}
			if len(ws) > 0 {
				fl.Flush()
			}
		case <-keep.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				h.scheduleGrace(s.ID)
				return
			}
			fl.Flush()
		}
	}
}

// writeFrame emits one SSE message. The id line is "<session>:<frame>" and
// is omitted for frames with no ID (widgets) or when session is "".
func writeFrame(w http.ResponseWriter, session string, f protocol.Frame) error {
	raw, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if f.ID != 0 && session != "" {
		if _, err := fmt.Fprintf(w, "id: %s:%d\n", session, f.ID); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", f.T, raw)
	return err
}

// resume parses Last-Event-ID. It returns the session and the frames to
// replay when the ring can serve them; nil otherwise (start over).
func (h *handler) resume(header string) (*host.Session, []protocol.Frame) {
	sid, rest, ok := strings.Cut(header, ":")
	if !ok {
		return nil, nil
	}
	id, err := strconv.ParseUint(rest, 10, 64)
	if err != nil {
		return nil, nil
	}
	s, ok := h.reg.Session(sid)
	if !ok {
		return nil, nil
	}
	frames, ok := s.Since(id)
	if !ok {
		h.reg.CloseSession(sid) // its client has moved on; free it
		return nil, nil
	}
	return s, frames
}

// scheduleGrace closes a disconnected session after ResumeGrace unless a
// resume arrives first (PL-6).
func (h *handler) scheduleGrace(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t, ok := h.grace[id]; ok {
		t.Stop()
	}
	h.grace[id] = time.AfterFunc(h.opts.ResumeGrace, func() {
		h.mu.Lock()
		delete(h.grace, id)
		h.mu.Unlock()
		h.reg.CloseSession(id)
	})
}

func (h *handler) cancelGrace(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t, ok := h.grace[id]; ok {
		t.Stop()
		delete(h.grace, id)
	}
}
```

Note the drain loop sets `out = nil` on close so the outer `select`'s receive on a nil channel blocks and the closed-channel branch is not re-entered; restructure to a labelled loop if that reads better — the observable behaviour to keep: after close, the overflow frame (if any) is written exactly once and the handler returns.

`static.go`:

```go
package httpapi

import (
	"io/fs"
	"net/http"
	"strings"
)

// mountStatic serves the embedded client for every non-API path, with the
// SPA fallback to index.html so /t/{table} deep links load. With no build
// embedded, every such path is a 503 that says how to build it.
func (h *handler) mountStatic(mux *http.ServeMux) {
	if h.opts.Web == nil {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
				return
			}
			http.Error(w, "web build missing — run make web", http.StatusServiceUnavailable)
		})
		return
	}
	files := http.FileServer(http.FS(h.opts.Web))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "not_found", "no such endpoint")
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if f, err := h.opts.Web.Open(p); err == nil {
			f.Close()
			files.ServeHTTP(w, r)
			return
		}
		// Unknown path: the SPA router owns it.
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	})
}

var _ fs.FS = (fs.FS)(nil)
```

(drop the trailing `var _` line if `fs` is otherwise unused — import only what is used). Remove the two stubs from `handler.go`. The generic `/api/` 404 registration in `NewHandler` now conflicts with `/`'s handler only by specificity — the mux picks the longer pattern, so keep both.

- [ ] **Step 4: Run the tests with `-race`**

Run: `go test -race ./host/httpapi/ -count=1 -v`
Expected: PASS.

- [ ] **Step 5: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./...`

```bash
git add host/httpapi/
git commit -m "feat(httpapi): SSE stream with Last-Event-ID resume, widget ticker, static SPA

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 16: `cmd/gorged`

**Files:**
- Create: `cmd/gorged/main.go`, `cmd/gorged/embed.go`, `cmd/gorged/webdist/.keep`, `cmd/gorged/main_test.go`
- Modify: `.gitignore` (`cmd/gorged/webdist/*` + `!cmd/gorged/webdist/.keep`), `Makefile` (`gorged` target; `build` includes it; `help`), `README.md` (a "Running gorged" section)

**Interfaces:**
- Consumes: `cards.OpenCorpus`, `deck.Load`, `host.New/AddTable/StartAll/Close`, `httpapi.NewHandler`.
- Produces: the binary. Flags: `-addr :8080`, `-cards .cards`, `-decks internal/testutil/decks`, `-tables 4`, `-seats 4`, `-pace 1.5s`, `-cooldown 5s`, `-dir gorged-data`, `-spectator omniscient`, `-seed 1`, `-perpetual` (default true). Table *i* (1-based) is `t<i>` / "Table <i>" with seed `seed + i - 1` and every deck in the directory (sorted by file name) as its rotation.

- [ ] **Step 1: Write the failing test**

Create `cmd/gorged/main_test.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
)

func TestServesTablesOverHTTP(t *testing.T) {
	testutil.CorpusRegistry(t) // Skips when .cards/ is absent
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config{cards: "../../.cards", decks: "../../internal/testutil/decks", tables: 1, seats: 2, pace: 0,
		cooldown: 0, dir: t.TempDir(), spectator: "omniscient", seed: 1, perpetual: false}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, ln) }()
	url := "http://" + ln.Addr().String()
	var tables []protocol.TableInfo
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get(url + "/api/tables")
		if err == nil {
			_ = json.NewDecoder(resp.Body).Decode(&tables)
			resp.Body.Close()
			if len(tables) == 1 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no tables served: %v %+v", err, tables)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if tables[0].ID != "t1" || tables[0].Seats != 2 || tables[0].Spectator != "omniscient" {
		t.Fatalf("%+v", tables[0])
	}
	resp, err := http.Get(url + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 && resp.StatusCode != 503 {
		t.Fatalf("/: %d", resp.StatusCode)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
}

func TestDeckDirectoryListingIsSortedAndComplete(t *testing.T) {
	names, err := deckFiles("../../internal/testutil/decks")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 12 || names[0] != "death-n-taxes" || names[len(names)-1] != "uw-tempo" {
		t.Fatalf("%v", names)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./cmd/gorged/ -count=1`
Expected: FAIL — no non-test files.

- [ ] **Step 3: Implement `embed.go` and `main.go`**

`cmd/gorged/embed.go`:

```go
package main

import (
	"embed"
	"io/fs"
)

// webdist holds the Svelte build (make web). It is gitignored except for
// .keep, so a clean clone builds with no Node; webFS reports nil until a
// real build is present and httpapi then serves a 503 for the client.
//
//go:embed all:webdist
var webdist embed.FS

func webFS() fs.FS {
	sub, err := fs.Sub(webdist, "webdist")
	if err != nil {
		return nil
	}
	if f, err := sub.Open("index.html"); err != nil {
		return nil
	} else {
		f.Close()
	}
	return sub
}
```

`cmd/gorged/main.go`:

```go
// Command gorged runs perpetual bot tables and serves them to browsers:
// the host library behind a net/http server with the Svelte client
// embedded. It is the M2a deliverable; mtgserve embeds the same packages.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/deck"
	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/host/httpapi"
	"github.com/adams-shaun/gorge/view"
)

type config struct {
	addr, cards, decks, dir, spectator string
	tables, seats                       int
	pace, cooldown                      time.Duration
	seed                                uint64
	perpetual                           bool
}

func main() {
	var c config
	flag.StringVar(&c.addr, "addr", ":8080", "listen address")
	flag.StringVar(&c.cards, "cards", ".cards", "corpus directory (ir.gob.gz / cardsfolder)")
	flag.StringVar(&c.decks, "decks", "internal/testutil/decks", "directory of deck JSON files")
	flag.IntVar(&c.tables, "tables", 4, "number of tables")
	flag.IntVar(&c.seats, "seats", 4, "seats per table")
	flag.DurationVar(&c.pace, "pace", 1500*time.Millisecond, "sleep after every decision; 0 = as fast as possible")
	flag.DurationVar(&c.cooldown, "cooldown", 5*time.Second, "pause between matches on a perpetual table")
	flag.StringVar(&c.dir, "dir", "gorged-data", "persistence directory")
	flag.StringVar(&c.spectator, "spectator", "omniscient", "spectator visibility: public or omniscient")
	flag.Uint64Var(&c.seed, "seed", 1, "seed of table 1; table i uses seed+i-1")
	flag.BoolVar(&c.perpetual, "perpetual", true, "start a new match when one ends")
	flag.Parse()

	ln, err := net.Listen("tcp", c.addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gorged:", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx, c, ln); err != nil {
		fmt.Fprintln(os.Stderr, "gorged:", err)
		os.Exit(1)
	}
}

// serve runs until ctx is cancelled, then aborts live matches and shuts
// the server down. Split from main so a test can drive it on a random
// port.
func serve(ctx context.Context, c config, ln net.Listener) error {
	vis, err := view.ParseVisibility(c.spectator)
	if err != nil {
		return err
	}
	if vis == view.Seat {
		return fmt.Errorf("-spectator must be public or omniscient")
	}
	reg, err := cards.OpenCorpus(c.cards)
	if err != nil {
		return fmt.Errorf("opening corpus at %s: %w (run make fetch-cards compile-cards)", c.cards, err)
	}
	names, err := deckFiles(c.decks)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("no deck files in %s", c.decks)
	}
	load := deckLoader(reg, c.decks)

	r, err := host.New(host.Options{Dir: c.dir, LoadDeck: load, Sleep: time.Sleep, Sync: true, Cooldown: c.cooldown})
	if err != nil {
		return err
	}
	if len(r.Tables()) == 0 {
		for i := 1; i <= c.tables; i++ {
			cfg := host.TableConfig{ID: host.TableID(fmt.Sprintf("t%d", i)), Name: fmt.Sprintf("Table %d", i), Seats: c.seats,
				Decks: names, Seed: c.seed + uint64(i-1), Pace: c.pace, Spectator: vis, Perpetual: c.perpetual}
			if err := r.AddTable(cfg); err != nil {
				return err
			}
		}
	}
	if err := r.StartAll(); err != nil {
		return err
	}
	srv := &http.Server{Handler: httpapi.NewHandler(r, httpapi.Options{Web: webFS()})}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	fmt.Fprintf(os.Stderr, "gorged: %d tables of %d on %s (dir %s)\n", len(r.Tables()), c.seats, ln.Addr(), c.dir)
	select {
	case err := <-errc:
		r.Close()
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	return r.Close()
}

// deckFiles lists the deck names (file stems) in dir, sorted, so seat
// assignment is the same on every machine.
func deckFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, deck.Stem(e.Name()))
		}
	}
	sort.Strings(names)
	return names, nil
}

// deckLoader resolves a name to dir/<name>.json once and caches it: the
// host asks for the same decks every match.
func deckLoader(reg *cards.Registry, dir string) func(string) (host.Deck, error) {
	var mu sync.Mutex
	cache := map[string]host.Deck{}
	return func(name string) (host.Deck, error) {
		mu.Lock()
		defer mu.Unlock()
		if d, ok := cache[name]; ok {
			return d, nil
		}
		// The seat is named after the file stem (PL-14), not the deck
		// file's own name field, so the parsed File is not needed here.
		_, cs, err := deck.Load(reg, filepath.Join(dir, name+".json"))
		if err != nil {
			return host.Deck{}, err
		}
		d := host.Deck{Name: name, Cards: cs}
		cache[name] = d
		return d, nil
	}
}
```

`.gitignore` additions:

```
# Svelte build output embedded into gorged; rebuilt by `make web`.
cmd/gorged/webdist/*
!cmd/gorged/webdist/.keep
web/node_modules/
web/dist/
web/test-results/
web/playwright-report/
```

Create the empty file `cmd/gorged/webdist/.keep`.

`Makefile`: add `$(BIN_DIR)/gorged` (same recipe as mtgsim, `./cmd/gorged`), include it in `build`, add:

```make
.PHONY: gorged
gorged: $(BIN_DIR)/gorged
	$(BIN_DIR)/gorged -decks internal/testutil/decks -tables 4 -seats 4 -pace 1.5s
```

and a `help` line. `README.md`: a short "Running gorged" section — the one command, what you see at `http://localhost:8080`, that `make web` builds the client first, and that `gorged-data/` holds the match files.

- [ ] **Step 4: Run tests and the binary**

Run: `go test ./cmd/gorged/ -count=1 -v` — PASS (or SKIP without a corpus; on this machine the corpus exists so it must PASS).
Run: `make build && ./bin/gorged -tables 1 -seats 4 -pace 0 -perpetual=false -dir $(mktemp -d) &` then `curl -s localhost:8080/api/tables | head -c 300; curl -s localhost:8080/ -o /dev/null -w '%{http_code}\n'; kill %1`
Expected: a JSON table list; `503` (no web build yet).

- [ ] **Step 5: Gates and commit**

Run: `make lint && go build ./... && go test -count=1 ./... && git ls-files | grep -c '\.txt$'`
Expected: clean; `0`.

```bash
git add cmd/gorged/ .gitignore Makefile README.md
git commit -m "feat(gorged): the table server binary with the client embed point

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

## Phase 4 — the client

The client is a Svelte 5 + Vite + TypeScript SPA under `web/`. It renders views, options and server-supplied lines; it never decides what is legal. Node is needed for Tasks 17–24 only; `make lint` runs `svelte-check` and eslint (blocking, per the spec), so from Task 17 on a Go-only machine fails lint — that is intended.

Conventions for every web task: run `npm run check && npm run lint && npm run test` in `web/` before committing; component logic that can be a pure function (reducers, formatters, stores' transitions) lives in `web/src/lib/*.ts` with Vitest tests; Svelte components stay thin. Use Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`) — no legacy stores except where a module-level store is the simplest shared state. No `any`.

### Task 17: `web/` scaffold — build into `webdist`, stream, API, router, lint

**Files:**
- Create: `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json`, `web/svelte.config.js`, `web/eslint.config.js`, `web/index.html`, `web/src/main.ts`, `web/src/app.css`, `web/src/App.svelte`, `web/src/lib/api.ts`, `web/src/lib/stream.ts`, `web/src/lib/router.ts`, `web/src/lib/colours.ts`, `web/src/lib/api.test.ts`, `web/src/lib/stream.test.ts`, `web/src/lib/router.test.ts`
- Modify: `Makefile` (`web`, `web-dev`, `test-web`; `lint` gains the web checks), `.gitignore` (already has the entries from Task 16)

**Interfaces:**
- Consumes: `web/src/protocol.ts` (Task 8).
- Produces (used by Tasks 18–24):

```ts
// api.ts
export async function getJSON<T>(path: string): Promise<T>            // throws ApiError{status, code, message, head?}
export function tablesURL(): string; matchesURL(t): string; viewURL(t, k, seq?): string; eventsURL(t, k, since): string
export async function subscribe(session, table, mode): Promise<void>; unsubscribe(session, table)
// stream.ts
export type FrameHandler = (f: Frame) => void
export interface Stream { readonly session: string | null; onFrame(h: FrameHandler): () => void; close(): void }
export function openStream(url: string, es: EventSourceLike = window.EventSource): Stream
export function parseFrame(data: string): Frame | null
// router.ts
export type Route = {kind:'overview'} | {kind:'table', table: string} | {kind:'match', table: string, match: number} | {kind:'notfound'}
export function parseRoute(pathname: string): Route
export function href(r: Route): string
// colours.ts
export const SEAT_COLOURS: readonly string[]   // same eight as protocol.SeatColours
export function seatColour(i: number, seats?: SeatInfo[]): string
```

`openStream` reconnects through `EventSource` itself; on every `hello` it records the new session id and notifies handlers, so the stores re-subscribe (spec: "the client re-subscribes and receives fresh snapshots").

- [ ] **Step 1: Scaffold**

Run in the repo root:

```bash
npm create vite@latest web -- --template svelte-ts
cd web && npm install && npm install -D vitest @vitest/coverage-v8 svelte-check eslint @eslint/js typescript-eslint eslint-plugin-svelte globals @playwright/test
```

Pin what `npm create` produced (Svelte 5.x, Vite 6 or 7); record the exact versions in the commit body. Replace `web/vite.config.ts` with:

```ts
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: { outDir: '../cmd/gorged/webdist', emptyOutDir: true },
  server: { proxy: { '/api': { target: 'http://localhost:8080', changeOrigin: true } } },
  test: { environment: 'node', include: ['src/**/*.test.ts'] },
});
```

`emptyOutDir` would delete `.keep`; add to the build script: `"build": "vite build && touch ../cmd/gorged/webdist/.keep"`. Scripts in `package.json`:

```json
"scripts": {
  "dev": "vite",
  "build": "vite build && touch ../cmd/gorged/webdist/.keep",
  "preview": "vite preview",
  "check": "svelte-check --tsconfig ./tsconfig.json",
  "lint": "eslint .",
  "test": "vitest run",
  "e2e": "playwright test"
}
```

`web/eslint.config.js` (flat config):

```js
import js from '@eslint/js';
import ts from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';

export default ts.config(
  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs['flat/recommended'],
  { languageOptions: { globals: { ...globals.browser } } },
  { files: ['**/*.svelte'], languageOptions: { parserOptions: { parser: ts.parser } } },
  { rules: { '@typescript-eslint/no-explicit-any': 'error' } },
  { ignores: ['dist/', 'node_modules/', '../cmd/gorged/webdist/', 'src/protocol.ts'] },
);
```

Delete the template's `Counter.svelte`, `lib/` assets and `app.css` content you do not use; keep `index.html` with `<title>gorge</title>` and a `<div id="app">`.

- [ ] **Step 2: Write the failing unit tests**

`web/src/lib/router.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { href, parseRoute } from './router';

describe('router', () => {
  it('parses the three routes and rejects the rest', () => {
    expect(parseRoute('/')).toEqual({ kind: 'overview' });
    expect(parseRoute('/t/t1')).toEqual({ kind: 'table', table: 't1' });
    expect(parseRoute('/t/t1/m/7')).toEqual({ kind: 'match', table: 't1', match: 7 });
    expect(parseRoute('/t/t1/m/x')).toEqual({ kind: 'notfound' });
    expect(parseRoute('/nope')).toEqual({ kind: 'notfound' });
  });
  it('round-trips through href', () => {
    for (const p of ['/', '/t/t1', '/t/t1/m/7']) expect(href(parseRoute(p))).toBe(p);
  });
});
```

`web/src/lib/api.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { eventsURL, matchesURL, tablesURL, viewURL } from './api';

describe('api urls', () => {
  it('builds the documented paths', () => {
    expect(tablesURL()).toBe('/api/tables');
    expect(matchesURL('t1')).toBe('/api/tables/t1/matches');
    expect(viewURL('t1', 3)).toBe('/api/tables/t1/matches/3/view');
    expect(viewURL('t1', 3, 0)).toBe('/api/tables/t1/matches/3/view?seq=0');
    expect(eventsURL('t 1', 3, 42)).toBe('/api/tables/t%201/matches/3/events?since=42');
  });
});
```

`web/src/lib/stream.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { openStream, parseFrame, type EventSourceLike } from './stream';

class FakeES implements EventSourceLike {
  static last: FakeES | null = null;
  listeners: Record<string, ((e: MessageEvent) => void)[]> = {};
  closed = false;
  constructor(public url: string) { FakeES.last = this; }
  addEventListener(type: string, fn: (e: MessageEvent) => void) { (this.listeners[type] ??= []).push(fn); }
  close() { this.closed = true; }
  emit(type: string, data: string, lastEventId = '') {
    for (const fn of this.listeners[type] ?? []) fn(new MessageEvent(type, { data, lastEventId }));
  }
}

const hello = (session: string) => JSON.stringify({ v: 1, t: 'hello', seq: 0, body: { session, tables: [] } });

describe('stream', () => {
  it('parses frames and rejects garbage', () => {
    expect(parseFrame(hello('s1'))?.t).toBe('hello');
    expect(parseFrame('{')).toBeNull();
    expect(parseFrame(JSON.stringify({ v: 2, t: 'hello', seq: 0, body: {} }))).toBeNull(); // wrong version
  });
  it('tracks the session across hellos and dispatches every frame type', () => {
    const s = openStream('/api/stream', FakeES as unknown as typeof EventSource);
    const seen: string[] = [];
    s.onFrame((f) => seen.push(f.t));
    const es = FakeES.last!;
    expect(es.url).toBe('/api/stream');
    es.emit('hello', hello('s1'));
    expect(s.session).toBe('s1');
    es.emit('widget', JSON.stringify({ v: 1, t: 'widget', seq: 5, table: 't1', match: 1, body: {} }));
    es.emit('hello', hello('s2'));
    expect(s.session).toBe('s2');
    expect(seen).toEqual(['hello', 'widget', 'hello']);
    s.close();
    expect(es.closed).toBe(true);
  });
});
```

Run: `cd web && npm run test` — FAIL (modules missing).

- [ ] **Step 3: Implement the libs**

`web/src/lib/router.ts`:

```ts
export type Route =
  | { kind: 'overview' }
  | { kind: 'table'; table: string }
  | { kind: 'match'; table: string; match: number }
  | { kind: 'notfound' };

export function parseRoute(pathname: string): Route {
  if (pathname === '/') return { kind: 'overview' };
  const m = pathname.match(/^\/t\/([^/]+)(?:\/m\/(\d+))?$/);
  if (!m) return { kind: 'notfound' };
  const table = decodeURIComponent(m[1]);
  if (m[2] === undefined) return { kind: 'table', table };
  return { kind: 'match', table, match: Number(m[2]) };
}

export function href(r: Route): string {
  switch (r.kind) {
    case 'overview': return '/';
    case 'table': return `/t/${encodeURIComponent(r.table)}`;
    case 'match': return `/t/${encodeURIComponent(r.table)}/m/${r.match}`;
    case 'notfound': return '/';
  }
}

/** navigate pushes a route and notifies App via popstate. */
export function navigate(r: Route): void {
  history.pushState(null, '', href(r));
  dispatchEvent(new PopStateEvent('popstate'));
}
```

`web/src/lib/api.ts`:

```ts
import type { ErrorBody, EventBody, MatchInfo, TableInfo, View } from '../protocol';

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public head?: number) {
    super(message);
  }
}

const enc = encodeURIComponent;
export const tablesURL = () => '/api/tables';
export const matchesURL = (t: string) => `/api/tables/${enc(t)}/matches`;
export const viewURL = (t: string, k: number, seq?: number) =>
  `/api/tables/${enc(t)}/matches/${k}/view${seq === undefined ? '' : `?seq=${seq}`}`;
export const eventsURL = (t: string, k: number, since: number) =>
  `/api/tables/${enc(t)}/matches/${k}/events?since=${since}`;

export async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: 'application/json' } });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as Partial<ErrorBody>;
    throw new ApiError(res.status, body.code ?? 'http', body.message ?? res.statusText, body.head);
  }
  return (await res.json()) as T;
}

async function postJSON(path: string, body: unknown): Promise<void> {
  const res = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  if (!res.ok) {
    const e = (await res.json().catch(() => ({}))) as Partial<ErrorBody>;
    throw new ApiError(res.status, e.code ?? 'http', e.message ?? res.statusText);
  }
}

export const fetchTables = () => getJSON<TableInfo[]>(tablesURL());
export const fetchMatches = (t: string) => getJSON<MatchInfo[]>(matchesURL(t));
export const fetchView = (t: string, k: number, seq?: number) => getJSON<View>(viewURL(t, k, seq));
export const fetchEvents = (t: string, k: number, since: number) => getJSON<EventBody[]>(eventsURL(t, k, since));
export const subscribe = (session: string, table: string, mode: 'overview' | 'focus') =>
  postJSON('/api/subscribe', { session, table, mode });
export const unsubscribe = (session: string, table: string) => postJSON('/api/unsubscribe', { session, table });
```

`web/src/lib/stream.ts`:

```ts
import type { Frame, FrameType, Hello } from '../protocol';

/** The subset of EventSource the stream uses; tests pass a fake. */
export interface EventSourceLike {
  addEventListener(type: string, fn: (e: MessageEvent) => void): void;
  close(): void;
}
type ESCtor = new (url: string) => EventSourceLike;

export type FrameHandler = (f: Frame) => void;

export interface Stream {
  readonly session: string | null;
  onFrame(h: FrameHandler): () => void;
  close(): void;
}

const FRAME_TYPES: FrameType[] = ['hello', 'widget', 'match_start', 'snapshot', 'event', 'decision', 'match_end', 'table_halted', 'overflow', 'error'];

/** parseFrame decodes one SSE data line; anything malformed or from another protocol version is dropped. */
export function parseFrame(data: string): Frame | null {
  try {
    const f = JSON.parse(data) as Frame;
    if (f.v !== 1 || !FRAME_TYPES.includes(f.t)) return null;
    return f;
  } catch {
    return null;
  }
}

/**
 * openStream owns one EventSource. The browser reconnects and resends
 * Last-Event-ID by itself; every hello (first connect, or a resume the
 * server could not serve from its ring) carries a new session id, which
 * handlers see as a hello frame and answer by re-subscribing.
 */
export function openStream(url: string, es: ESCtor = EventSource as unknown as ESCtor): Stream {
  const source = new es(url);
  const handlers = new Set<FrameHandler>();
  let session: string | null = null;
  for (const t of FRAME_TYPES) {
    source.addEventListener(t, (e: MessageEvent) => {
      const f = parseFrame(String(e.data));
      if (!f) return;
      if (f.t === 'hello') session = (f.body as Hello).session;
      for (const h of handlers) h(f);
    });
  }
  return {
    get session() { return session; },
    onFrame(h) { handlers.add(h); return () => handlers.delete(h); },
    close() { source.close(); },
  };
}
```

`web/src/lib/colours.ts`:

```ts
import type { SeatInfo } from '../protocol';

/** Mirrors protocol.SeatColours; the server's SeatInfo.colour wins when known. */
export const SEAT_COLOURS: readonly string[] = ['#e5484d', '#3b82f6', '#22c55e', '#eab308', '#a855f7', '#f97316', '#14b8a6', '#ec4899'];

export function seatColour(i: number, seats?: SeatInfo[]): string {
  return seats?.[i]?.colour ?? SEAT_COLOURS[i % SEAT_COLOURS.length];
}
```

`web/src/App.svelte` — the router shell (routes render placeholders until Tasks 19–21 fill them):

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { parseRoute, type Route } from './lib/router';
  import Overview from './routes/Overview.svelte';
  import Table from './routes/Table.svelte';

  let route: Route = $state(parseRoute(location.pathname));
  onMount(() => {
    const onPop = () => (route = parseRoute(location.pathname));
    addEventListener('popstate', onPop);
    return () => removeEventListener('popstate', onPop);
  });
</script>

{#if route.kind === 'overview'}
  <Overview />
{:else if route.kind === 'table'}
  <Table table={route.table} />
{:else if route.kind === 'match'}
  <Table table={route.table} match={route.match} />
{:else}
  <main class="notfound"><h1>Not found</h1><a href="/">Overview</a></main>
{/if}
```

Create `web/src/routes/Overview.svelte` and `web/src/routes/Table.svelte` as placeholders that render their props in an `<h1>` (Tasks 19 and 20 replace them). `web/src/main.ts` mounts `App` with `mount(App, { target: document.getElementById('app')! })` (Svelte 5 API).

- [ ] **Step 4: Makefile and lint**

Add to `Makefile`:

```make
.PHONY: web web-dev test-web lint-web
web:
	cd web && npm ci && npm run build

web-dev:
	cd web && npm run dev

test-web:
	cd web && npm run test

lint-web:
	cd web && npm run check && npm run lint
```

and make `lint` depend on `lint-web` (`lint: lint-web` before its recipe). Update `help`.

Run: `cd web && npm run test && npm run check && npm run lint && npm run build && ls ../cmd/gorged/webdist/` — PASS; `index.html`, `assets/`, `.keep` present.
Run: `make lint` (from the root) — clean. `go build ./cmd/gorged && ./gorged -tables 1 -pace 0 -perpetual=false -dir $(mktemp -d) & sleep 2; curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/; kill %1` — `200`.

- [ ] **Step 5: Commit**

```bash
git add web/ Makefile
git commit -m "feat(web): Svelte scaffold, stream, api, router; lint gains svelte-check and eslint

Svelte <x.y.z>, Vite <x.y.z>, Vitest <x.y.z>, Playwright <x.y.z>.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

(fill in the four versions from `web/package.json`). Confirm `git status` shows nothing under `cmd/gorged/webdist/` but `.keep` and no `node_modules`.

---

### Task 18: the DVR cursor reducer

**Files:**
- Create: `web/src/lib/dvr.ts`, `web/src/lib/dvr.test.ts`

**Interfaces:**
- Consumes: `EventBody`, `Snapshot` from `protocol.ts`.
- Produces:

```ts
export interface DvrState {
  match: string | null;      // "<table>/<k>" this state belongs to
  head: number;              // seq of the latest event known
  cursor: number;            // seq being rendered
  live: boolean;             // cursor pinned to head
  events: EventBody[];       // contiguous, ascending; events[0].event.seq === firstSeq
  turnStarts: number[];      // ascending
  gap: boolean;              // an event arrived out of order: caller must re-snapshot
}
export type DvrAction =
  | { type: 'snapshot'; match: string; head: number; turnStarts: number[] }
  | { type: 'event'; body: EventBody }
  | { type: 'backfill'; events: EventBody[] }     // older events fetched from /events
  | { type: 'pause' } | { type: 'live' }
  | { type: 'step'; by: number }                   // ±1
  | { type: 'scrub'; seq: number }
  | { type: 'reset' };
export const initialDvr: DvrState
export function dvrReducer(s: DvrState, a: DvrAction): DvrState
export function behindLive(s: DvrState): number
export function eventAt(s: DvrState, seq: number): EventBody | undefined
export function turnOf(s: DvrState, seq: number): number   // index into turnStarts, -1 before the first
```

- [ ] **Step 1: Write the failing tests**

`web/src/lib/dvr.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { behindLive, dvrReducer, eventAt, initialDvr, turnOf, type DvrState } from './dvr';
import type { EventBody } from '../protocol';

const ev = (seq: number, kind = 'tap'): EventBody => ({ event: { seq, kind, player: 0 }, line: `${kind} ${seq}` });

function live(seqs: number[], head0 = 100): DvrState {
  let s = dvrReducer(initialDvr, { type: 'snapshot', match: 't1/1', head: head0, turnStarts: [0, 40, 90] });
  for (const q of seqs) s = dvrReducer(s, { type: 'event', body: ev(q) });
  return s;
}

describe('dvr reducer', () => {
  it('starts live at the snapshot head', () => {
    const s = live([]);
    expect(s).toMatchObject({ match: 't1/1', head: 100, cursor: 100, live: true, events: [], gap: false });
  });
  it('follows contiguous events while live', () => {
    const s = live([101, 102, 103]);
    expect(s.head).toBe(103);
    expect(s.cursor).toBe(103);
    expect(behindLive(s)).toBe(0);
    expect(eventAt(s, 102)?.line).toBe('tap 102');
  });
  it('flags a gap on an out-of-order event and ignores duplicates', () => {
    let s = live([101]);
    s = dvrReducer(s, { type: 'event', body: ev(101) });
    expect(s.gap).toBe(false);
    expect(s.events.length).toBe(1);
    s = dvrReducer(s, { type: 'event', body: ev(105) });
    expect(s.gap).toBe(true);
  });
  it('pauses, counts behind-live, steps, and returns to live', () => {
    let s = live([101, 102]);
    s = dvrReducer(s, { type: 'pause' });
    s = dvrReducer(s, { type: 'event', body: ev(103) });
    s = dvrReducer(s, { type: 'event', body: ev(104) });
    expect(s.cursor).toBe(102);
    expect(behindLive(s)).toBe(2);
    s = dvrReducer(s, { type: 'step', by: -1 });
    expect(s.cursor).toBe(101);
    s = dvrReducer(s, { type: 'step', by: 1 });
    s = dvrReducer(s, { type: 'step', by: 1 });
    s = dvrReducer(s, { type: 'step', by: 1 });
    s = dvrReducer(s, { type: 'step', by: 1 });
    expect(s.cursor).toBe(104); // clamped at head, still paused
    expect(s.live).toBe(false);
    s = dvrReducer(s, { type: 'live' });
    expect(s).toMatchObject({ cursor: 104, live: true });
  });
  it('stepping back from live pauses; cursor never goes below 0', () => {
    let s = live([101]);
    s = dvrReducer(s, { type: 'step', by: -1 });
    expect(s.live).toBe(false);
    expect(s.cursor).toBe(100);
    s = dvrReducer(s, { type: 'scrub', seq: -5 });
    expect(s.cursor).toBe(0);
    s = dvrReducer(s, { type: 'scrub', seq: 999 });
    expect(s.cursor).toBe(101);
  });
  it('scrubs to turn starts and reports the current turn', () => {
    let s = live([101]);
    s = dvrReducer(s, { type: 'scrub', seq: 40 });
    expect(s.cursor).toBe(40);
    expect(turnOf(s, 40)).toBe(1);
    expect(turnOf(s, 39)).toBe(0);
    expect(turnOf(s, 95)).toBe(2);
    expect(turnOf(s, -1)).toBe(-1);
  });
  it('backfills older events in front of the known ones', () => {
    let s = live([101, 102]);
    s = dvrReducer(s, { type: 'backfill', events: [ev(98), ev(99), ev(100), ev(101)] });
    expect(s.events.map((e) => e.event.seq)).toEqual([98, 99, 100, 101, 102]);
  });
  it('a snapshot for another match resets everything', () => {
    let s = live([101]);
    s = dvrReducer(s, { type: 'snapshot', match: 't1/2', head: 7, turnStarts: [0] });
    expect(s).toMatchObject({ match: 't1/2', head: 7, cursor: 7, live: true, events: [], gap: false });
    expect(dvrReducer(s, { type: 'reset' })).toEqual(initialDvr);
  });
  it('a snapshot for the same match while paused keeps the cursor', () => {
    let s = live([101, 102]);
    s = dvrReducer(s, { type: 'pause' });
    s = dvrReducer(s, { type: 'snapshot', match: 't1/1', head: 150, turnStarts: [0, 40, 90, 130] });
    expect(s.cursor).toBe(102);
    expect(s.live).toBe(false);
    expect(s.head).toBe(150);
    expect(s.events).toEqual([]); // the client re-fetches the range it needs
    expect(s.gap).toBe(false);
  });
});
```

Run: `cd web && npm run test -- dvr` — FAIL.

- [ ] **Step 2: Implement `dvr.ts`**

```ts
import type { EventBody } from '../protocol';

/**
 * The DVR is a cursor over a match's event sequence. The match never
 * pauses; the client does. `live` pins the cursor to `head`; pause/step/
 * scrub move it; `live` snaps back. Events are kept contiguous: an event
 * that is not head+1 is a gap, and the owner re-snapshots.
 */
export interface DvrState {
  match: string | null;
  head: number;
  cursor: number;
  live: boolean;
  events: EventBody[];
  turnStarts: number[];
  gap: boolean;
}

export type DvrAction =
  | { type: 'snapshot'; match: string; head: number; turnStarts: number[] }
  | { type: 'event'; body: EventBody }
  | { type: 'backfill'; events: EventBody[] }
  | { type: 'pause' }
  | { type: 'live' }
  | { type: 'step'; by: number }
  | { type: 'scrub'; seq: number }
  | { type: 'reset' };

export const initialDvr: DvrState = { match: null, head: 0, cursor: 0, live: true, events: [], turnStarts: [], gap: false };

const clamp = (n: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, n));

export function dvrReducer(s: DvrState, a: DvrAction): DvrState {
  switch (a.type) {
    case 'reset':
      return initialDvr;
    case 'snapshot': {
      const same = s.match === a.match;
      const live = same ? s.live : true;
      return {
        match: a.match, head: a.head, live,
        cursor: live ? a.head : clamp(s.cursor, 0, a.head),
        events: [], turnStarts: [...a.turnStarts], gap: false,
      };
    }
    case 'event': {
      const seq = a.body.event.seq;
      if (seq <= s.head && s.events.some((e) => e.event.seq === seq)) return s; // duplicate
      if (seq !== s.head + 1) return { ...s, gap: true };
      return { ...s, head: seq, cursor: s.live ? seq : s.cursor, events: [...s.events, a.body] };
    }
    case 'backfill': {
      const known = new Set(s.events.map((e) => e.event.seq));
      const older = a.events.filter((e) => !known.has(e.event.seq) && e.event.seq <= s.head);
      const events = [...older, ...s.events].sort((x, y) => x.event.seq - y.event.seq);
      return { ...s, events };
    }
    case 'pause':
      return { ...s, live: false };
    case 'live':
      return { ...s, live: true, cursor: s.head };
    case 'step':
      return { ...s, live: false, cursor: clamp(s.cursor + a.by, 0, s.head) };
    case 'scrub':
      return { ...s, live: false, cursor: clamp(a.seq, 0, s.head) };
  }
}

export const behindLive = (s: DvrState): number => s.head - s.cursor;

export function eventAt(s: DvrState, seq: number): EventBody | undefined {
  return s.events.find((e) => e.event.seq === seq);
}

/** turnOf is the index of the turn containing seq: the last turn start <= seq, or -1. */
export function turnOf(s: DvrState, seq: number): number {
  let i = -1;
  for (let k = 0; k < s.turnStarts.length && s.turnStarts[k] <= seq; k++) i = k;
  return i;
}
```

Run: `cd web && npm run test && npm run check && npm run lint` — PASS.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/dvr.ts web/src/lib/dvr.test.ts
git commit -m "feat(web): DVR cursor reducer

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 19: the overview — table widgets and the shared feed

**Files:**
- Create: `web/src/lib/tables.svelte.ts` (the tables store), `web/src/lib/feed.ts`, `web/src/lib/feed.test.ts`, `web/src/lib/session.svelte.ts` (one stream for the app), `web/src/routes/Overview.svelte` (replace the placeholder), `web/src/components/TableCell.svelte`, `web/src/components/Feed.svelte`, `web/src/components/LifeGrid.svelte`

**Interfaces:**
- Consumes: `openStream`, `subscribe`, `fetchTables`, `Widget`, `TableInfo`, `MatchStart`, `SeatInfo`, `seatColour`, `navigate`.
- Produces:

```ts
// session.svelte.ts — module singleton
export const session: { readonly id: string | null; readonly stream: Stream; ensureOverview(): void; focus(table: string): Promise<void>; unfocus(table: string): Promise<void> }
// tables.svelte.ts
export interface TableState { info: TableInfo; widget: Widget | null; seats: SeatInfo[]; match: number }
export const tables: { readonly list: TableState[]; apply(f: Frame): void; load(): Promise<void> }
// feed.ts (pure)
export interface FeedLine { table: string; match: number; seq: number; line: string }
export function pushFeed(lines: FeedLine[], l: FeedLine, cap?: number): FeedLine[]   // dedupes (table,match,seq); newest last; cap default 200
```

Survey recommendations 29–33 fix the look: a grid of state widgets (never shrunken boards) — each cell a 2×2 life grid in seat colours, a centre turn marker showing turn number and phase, the table name, a stack-depth badge, a state badge (live / cooldown / halted in red); a right rail with the merged, table-tagged feed of `widget.last` lines; clicking a cell navigates to the focused table.

- [ ] **Step 1: Write the failing feed test**

`web/src/lib/feed.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { pushFeed, type FeedLine } from './feed';

const l = (table: string, seq: number, line = 'x'): FeedLine => ({ table, match: 1, seq, line });

describe('feed', () => {
  it('appends newest last, dedupes by table/match/seq, and caps', () => {
    let f: FeedLine[] = [];
    f = pushFeed(f, l('t1', 1, 'a'));
    f = pushFeed(f, l('t2', 1, 'b'));
    f = pushFeed(f, l('t1', 1, 'a again'));
    expect(f.map((x) => x.line)).toEqual(['a', 'b']);
    for (let i = 2; i < 500; i++) f = pushFeed(f, l('t1', i), 100);
    expect(f.length).toBe(100);
    expect(f[f.length - 1].seq).toBe(499);
  });
  it('drops empty lines', () => {
    expect(pushFeed([], l('t1', 1, ''))).toEqual([]);
  });
});
```

Run: `cd web && npm run test -- feed` — FAIL.

- [ ] **Step 2: Implement the stores and components**

`web/src/lib/feed.ts`:

```ts
export interface FeedLine { table: string; match: number; seq: number; line: string }

export function pushFeed(lines: FeedLine[], l: FeedLine, cap = 200): FeedLine[] {
  if (!l.line) return lines;
  if (lines.some((x) => x.table === l.table && x.match === l.match && x.seq === l.seq)) return lines;
  const out = [...lines, l];
  return out.length > cap ? out.slice(out.length - cap) : out;
}
```

`web/src/lib/session.svelte.ts`:

```ts
import { openStream, type Stream } from './stream';
import { subscribe, unsubscribe } from './api';
import type { Frame } from '../protocol';

/**
 * One stream per page. Every hello (first connect or a server-side restart
 * of the session) re-issues the subscriptions this page holds, so a
 * reconnect is invisible to the routes.
 */
class Session {
  readonly stream: Stream;
  private wantOverview = false;
  private focused = new Set<string>();
  id = $state<string | null>(null);

  constructor() {
    this.stream = openStream('/api/stream');
    this.stream.onFrame((f: Frame) => {
      if (f.t !== 'hello') return;
      this.id = this.stream.session;
      void this.resubscribe();
    });
  }
  private async resubscribe() {
    if (!this.id) return;
    if (this.wantOverview) await subscribe(this.id, '*', 'overview').catch(() => {});
    for (const t of this.focused) await subscribe(this.id, t, 'focus').catch(() => {});
  }
  ensureOverview() {
    if (this.wantOverview) return;
    this.wantOverview = true;
    if (this.id) void subscribe(this.id, '*', 'overview').catch(() => {});
  }
  async focus(table: string) {
    this.focused.add(table);
    if (this.id) await subscribe(this.id, table, 'focus');
  }
  async unfocus(table: string) {
    this.focused.delete(table);
    if (this.id) await unsubscribe(this.id, table).catch(() => {});
  }
}

export const session = new Session();
```

`web/src/lib/tables.svelte.ts`:

```ts
import type { Frame, Hello, MatchStart, SeatInfo, TableInfo, Widget } from '../protocol';
import { fetchTables } from './api';
import { session } from './session.svelte';

export interface TableState { info: TableInfo; widget: Widget | null; seats: SeatInfo[]; match: number }

class Tables {
  list = $state<TableState[]>([]);

  constructor() {
    session.stream.onFrame((f) => this.apply(f));
  }
  private find(id: string) { return this.list.find((t) => t.info.id === id); }

  apply(f: Frame) {
    switch (f.t) {
      case 'hello': {
        const h = f.body as Hello;
        this.list = h.tables.map((info) => ({ info, widget: this.find(info.id)?.widget ?? null, seats: this.find(info.id)?.seats ?? [], match: info.match }));
        break;
      }
      case 'widget': {
        const t = this.find(f.table ?? '');
        if (t) { t.widget = f.body as Widget; t.match = f.match ?? t.match; }
        break;
      }
      case 'match_start': {
        const t = this.find(f.table ?? '');
        if (t) { t.seats = (f.body as MatchStart).seats; t.match = f.match ?? t.match; t.info = { ...t.info, state: 'live', match: t.match }; }
        break;
      }
      case 'match_end': {
        const t = this.find(f.table ?? '');
        if (t) t.info = { ...t.info, state: t.info.perpetual ? 'cooldown' : 'idle' };
        break;
      }
      case 'table_halted': {
        const t = this.find(f.table ?? '');
        if (t) t.info = { ...t.info, state: 'halted' };
        break;
      }
    }
  }
  async load() {
    const infos = await fetchTables();
    this.list = infos.map((info) => ({ info, widget: this.find(info.id)?.widget ?? null, seats: this.find(info.id)?.seats ?? [], match: info.match }));
  }
}

export const tables = new Tables();
```

`web/src/components/LifeGrid.svelte` — a 2×2 grid (4 seats; for 2 seats two cells, for more a wrapping grid) of `life` numbers on `seatColour(i, seats)` backgrounds, struck through when `lost[i]`:

```svelte
<script lang="ts">
  import type { SeatInfo } from '../protocol';
  import { seatColour } from '../lib/colours';
  let { life, lost, seats = [], active = -1 }: { life: number[]; lost: boolean[]; seats?: SeatInfo[]; active?: number } = $props();
</script>

<div class="grid" style:--n={life.length}>
  {#each life as l, i}
    <div class="seat" class:lost={lost[i]} class:active={i === active} style:background={seatColour(i, seats)} title={seats[i]?.name ?? `Seat ${i}`}>
      {l}
    </div>
  {/each}
</div>

<style>
  .grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 2px; }
  .seat { color: white; font: 700 1.6rem/1 system-ui, sans-serif; padding: .6rem .4rem; text-align: center; border-radius: 4px; opacity: .95; }
  .seat.lost { opacity: .35; text-decoration: line-through; }
  .seat.active { outline: 3px solid white; outline-offset: -3px; }
</style>
```

`web/src/components/TableCell.svelte` — name, state badge, `LifeGrid`, centre marker "T{turn} · {phase}", stack badge "{stack_depth} on stack"; `onclick` → `navigate({kind:'table', table})`. Halted cells get a red border and the word HALTED. `web/src/components/Feed.svelte` — a list of `FeedLine`s newest at the bottom, auto-scrolled, each prefixed by a coloured table tag. `web/src/routes/Overview.svelte`:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { session } from '../lib/session.svelte';
  import { tables } from '../lib/tables.svelte';
  import { pushFeed, type FeedLine } from '../lib/feed';
  import TableCell from '../components/TableCell.svelte';
  import Feed from '../components/Feed.svelte';
  import type { Widget } from '../protocol';

  let feed = $state<FeedLine[]>([]);
  onMount(() => {
    session.ensureOverview();
    void tables.load();
    return session.stream.onFrame((f) => {
      if (f.t === 'widget' && f.table) feed = pushFeed(feed, { table: f.table, match: f.match ?? 0, seq: f.seq, line: (f.body as Widget).last });
    });
  });
</script>

<main class="overview">
  <section class="grid">
    {#each tables.list as t (t.info.id)}
      <TableCell table={t} />
    {/each}
  </section>
  <aside class="rail"><Feed lines={feed} /></aside>
</main>

<style>
  .overview { display: grid; grid-template-columns: 1fr 22rem; height: 100vh; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(18rem, 1fr)); gap: 1rem; padding: 1rem; align-content: start; }
  .rail { border-left: 1px solid #333; overflow: hidden; }
</style>
```

- [ ] **Step 3: Check it against a real `gorged`**

Run: `cd web && npm run test && npm run check && npm run lint`. Then in one terminal `make build && ./bin/gorged -tables 2 -seats 4 -pace 300ms -dir $(mktemp -d)`; in another `cd web && npm run dev`; open `http://localhost:5173/`: two cells with four life numbers each ticking, turn marker advancing, feed lines scrolling with table tags, click a cell → URL becomes `/t/t1` (placeholder page). Reload: the cells return within a second (fresh hello → re-subscribe).

- [ ] **Step 4: Commit**

```bash
git add web/src
git commit -m "feat(web): overview grid of table widgets with a shared feed

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 20: the focused table — board, rail, identity bars, recent strip, transcript

**Files:**
- Create: `web/src/lib/board.ts`, `web/src/lib/board.test.ts`, `web/src/lib/mana.ts`, `web/src/lib/mana.test.ts`, `web/src/lib/match.svelte.ts`, `web/src/routes/Table.svelte` (replace the placeholder), `web/src/components/Board.svelte`, `web/src/components/Quadrant.svelte`, `web/src/components/CardTile.svelte`, `web/src/components/Rail.svelte`, `web/src/components/HandList.svelte`, `web/src/components/ManaSymbols.svelte`, `web/src/components/StackTile.svelte`, `web/src/components/PendingTray.svelte`, `web/src/components/IdentityBar.svelte`, `web/src/components/RecentStrip.svelte`, `web/src/components/Transcript.svelte`

**Interfaces:**
- Consumes: `View`, `PlayerView`, `CardView`, `StackView`, `PendingView`, `EventBody`, `Snapshot`, `MatchStart`, `DecisionBody`; `session.focus/unfocus`; `dvrReducer`; `fetchView`, `fetchEvents`, `fetchMatches`.
- Produces:

```ts
// board.ts (pure)
export type Group = 'lands' | 'creatures' | 'others'
export function groupBattlefield(cards: CardView[]): Record<Group, CardView[]>   // stable: by id within a group
export function quadrantFor(seat: number, seats: number): 'tl' | 'tr' | 'bl' | 'br' | 'l' | 'r'
export function recentlyMattered(events: EventBody[]): number | null  // obj id of the last stack_resolve, else null
// mana.ts (pure)
export function manaSymbols(cost: string): string[]   // "1 W" -> ["1","W"]; "X G G" -> ["X","G","G"]; "" -> []
// match.svelte.ts — per focused match state
export class MatchState {
  readonly table: string; match: number | null
  view: View | null; seats: SeatInfo[]; dvr: DvrState; decision: DecisionBody | null; halted: string | null
  apply(f: Frame): void            // frames addressed to this table
  dispatch(a: DvrAction): void
}
```

Layout follows survey recommendations 23–28: board ≈ 70 % of the width with four quadrants in seat colours (two seats: left/right halves); a right rail ≈ 18 % with every revealed hand as a text list with mana symbols, then the stack as type-banded tiles with labelled targets, then the pending-trigger tray; identity bars ≈ 11 % anchored to each seat's outer corner with life centred; a "recently mattered" strip showing the last resolved object large; the rules transcript along the bottom. Colours: seat colours from `MatchStart.seats`; active player's identity bar outlined; priority holder marked with a dot.

The live board is PL-16: after every `decision` or `match_end` frame the component fetches `view?seq=<head>` (Task 21 generalises this into the view cache; here a single `fetchView(table, match)` with one in-flight request and a "refetch after" flag is enough).

- [ ] **Step 1: Write the failing pure tests**

`web/src/lib/board.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { groupBattlefield, quadrantFor, recentlyMattered } from './board';
import type { CardView, EventBody } from '../protocol';

const card = (id: number, types: string): CardView => ({ id, name: `c${id}`, types, tapped: false, power: 0, toughness: 0, damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false, printing: { name: `c${id}` }, token: `#${id}` });

describe('board', () => {
  it('groups lands, creatures and the rest, ordered by id', () => {
    const g = groupBattlefield([card(9, 'Creature Goblin'), card(3, 'Basic Land Mountain'), card(5, 'Artifact'), card(2, 'Creature Human'), card(7, 'Artifact Creature Golem')]);
    expect(g.lands.map((c) => c.id)).toEqual([3]);
    expect(g.creatures.map((c) => c.id)).toEqual([2, 7, 9]);
    expect(g.others.map((c) => c.id)).toEqual([5]);
  });
  it('places seats in quadrants', () => {
    expect([0, 1, 2, 3].map((s) => quadrantFor(s, 4))).toEqual(['bl', 'tl', 'tr', 'br']);
    expect([0, 1].map((s) => quadrantFor(s, 2))).toEqual(['l', 'r']);
  });
  it('finds the last resolved object', () => {
    const ev = (seq: number, kind: string, obj?: number): EventBody => ({ event: { seq, kind, player: 0, obj }, line: '' });
    expect(recentlyMattered([ev(1, 'stack_push', 4), ev(2, 'stack_resolve', 4), ev(3, 'tap', 9)])).toBe(4);
    expect(recentlyMattered([ev(1, 'tap', 9)])).toBeNull();
  });
});
```

`web/src/lib/mana.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { manaSymbols } from './mana';

describe('manaSymbols', () => {
  it('splits Forge cost notation into symbols', () => {
    expect(manaSymbols('1 W')).toEqual(['1', 'W']);
    expect(manaSymbols('X G G')).toEqual(['X', 'G', 'G']);
    expect(manaSymbols('W/U W/U')).toEqual(['W/U', 'W/U']);
    expect(manaSymbols('')).toEqual([]);
    expect(manaSymbols('no cost')).toEqual([]);
  });
});
```

Run: `cd web && npm run test -- board mana` — FAIL.

- [ ] **Step 2: Implement the pure libs**

`web/src/lib/board.ts`:

```ts
import type { CardView, EventBody } from '../protocol';

export type Group = 'lands' | 'creatures' | 'others';

/** groupBattlefield sorts a seat's permanents into the three rows a quadrant shows. Type words come from the view; nothing here decides what a card does. */
export function groupBattlefield(cards: CardView[]): Record<Group, CardView[]> {
  const out: Record<Group, CardView[]> = { lands: [], creatures: [], others: [] };
  for (const c of [...cards].sort((a, b) => a.id - b.id)) {
    const words = c.types.split(' ');
    if (words.includes('Creature')) out.creatures.push(c);
    else if (words.includes('Land')) out.lands.push(c);
    else out.others.push(c);
  }
  return out;
}

/** quadrantFor places seat 0 bottom-left and proceeds clockwise, so turn order reads around the table. */
export function quadrantFor(seat: number, seats: number): 'tl' | 'tr' | 'bl' | 'br' | 'l' | 'r' {
  if (seats <= 2) return seat === 0 ? 'l' : 'r';
  return (['bl', 'tl', 'tr', 'br'] as const)[seat % 4];
}

/** recentlyMattered is the object id of the most recent stack_resolve, for the strip. */
export function recentlyMattered(events: EventBody[]): number | null {
  for (let i = events.length - 1; i >= 0; i--) {
    const e = events[i].event;
    if (e.kind === 'stack_resolve' && e.obj) return e.obj;
  }
  return null;
}
```

`web/src/lib/mana.ts`:

```ts
/** manaSymbols splits Forge's cost notation ("1 W", "X G G", "W/U") into renderable symbols; anything unrecognised yields []. */
export function manaSymbols(cost: string): string[] {
  if (!cost) return [];
  const parts = cost.trim().split(/\s+/);
  const ok = parts.every((p) => /^(\d+|X|Y|Z|[WUBRGC]|[WUBRGC]\/[WUBRGCP]|S|2\/[WUBRG])$/.test(p));
  return ok ? parts : [];
}
```

- [ ] **Step 3: The match state and the components**

`web/src/lib/match.svelte.ts`:

```ts
import type { DecisionBody, Frame, MatchStart, SeatInfo, Snapshot, View, EventBody } from '../protocol';
import { dvrReducer, initialDvr, type DvrAction, type DvrState } from './dvr';
import { fetchView } from './api';

/** MatchState is everything the focused view renders for one table. */
export class MatchState {
  match = $state<number | null>(null);
  view = $state<View | null>(null);
  seats = $state<SeatInfo[]>([]);
  dvr = $state<DvrState>(initialDvr);
  decision = $state<DecisionBody | null>(null);
  halted = $state<string | null>(null);
  private inflight = false;
  private again = false;

  constructor(readonly table: string) {}

  apply(f: Frame) {
    if (f.table !== this.table) return;
    switch (f.t) {
      case 'match_start':
        this.match = f.match ?? null;
        this.seats = (f.body as MatchStart).seats;
        this.decision = null;
        this.halted = null;
        break;
      case 'snapshot': {
        const s = f.body as Snapshot;
        this.match = f.match ?? this.match;
        this.dispatch({ type: 'snapshot', match: `${this.table}/${this.match}`, head: s.head, turnStarts: s.turn_starts });
        if (this.dvr.live) this.view = s.view;
        break;
      }
      case 'event':
        this.dispatch({ type: 'event', body: f.body as EventBody });
        break;
      case 'decision':
        this.decision = f.body as DecisionBody;
        if (this.dvr.live) void this.refreshLive();
        break;
      case 'match_end':
        this.decision = null;
        if (this.dvr.live) void this.refreshLive();
        break;
      case 'table_halted':
        this.halted = (f.body as { reason: string }).reason;
        break;
    }
  }

  dispatch(a: DvrAction) {
    this.dvr = dvrReducer(this.dvr, a);
  }

  /** refreshLive is PL-16: one GET per burst, coalesced. */
  async refreshLive() {
    if (this.match === null) return;
    if (this.inflight) { this.again = true; return; }
    this.inflight = true;
    try {
      const v = await fetchView(this.table, this.match, this.dvr.head);
      if (this.dvr.live) this.view = v;
    } catch { /* a 409 while the head moved: the next burst refetches */ }
    finally {
      this.inflight = false;
      if (this.again) { this.again = false; void this.refreshLive(); }
    }
  }
}
```

`web/src/routes/Table.svelte` (live mode; Task 21 adds the finished-match mode and the DVR bar):

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { session } from '../lib/session.svelte';
  import { tables } from '../lib/tables.svelte';
  import { MatchState } from '../lib/match.svelte';
  import Board from '../components/Board.svelte';
  import Rail from '../components/Rail.svelte';
  import IdentityBar from '../components/IdentityBar.svelte';
  import RecentStrip from '../components/RecentStrip.svelte';
  import Transcript from '../components/Transcript.svelte';
  import { quadrantFor } from '../lib/board';
  import { seatColour } from '../lib/colours';

  let { table, match = null }: { table: string; match?: number | null } = $props();
  const m = new MatchState(table);

  onMount(() => {
    const off = session.stream.onFrame((f) => m.apply(f));
    void session.focus(table);
    const t = tables.list.find((x) => x.info.id === table);
    if (t) m.seats = t.seats;
    return () => { off(); void session.unfocus(table); };
  });
</script>

<main class="table">
  {#if m.halted}<div class="halted">Table halted: {m.halted}</div>{/if}
  {#if m.view}
    <section class="board">
      <Board view={m.view} seats={m.seats} />
      {#each m.view.players as p (p.seat)}
        <IdentityBar player={p} seat={m.seats[p.seat]} colour={seatColour(p.seat, m.seats)} active={m.view.active === p.seat} priority={m.view.priority === p.seat} corner={quadrantFor(p.seat, m.view.players.length)} />
      {/each}
      <RecentStrip view={m.view} events={m.dvr.events} />
    </section>
    <aside class="rail"><Rail view={m.view} seats={m.seats} decision={m.decision} /></aside>
    <footer class="transcript"><Transcript dvr={m.dvr} onSeek={(seq) => m.dispatch({ type: 'scrub', seq })} /></footer>
  {:else}
    <p class="waiting">Waiting for {table}…</p>
  {/if}
</main>

<style>
  .table { display: grid; grid-template-columns: 1fr 18%; grid-template-rows: 1fr 9rem; height: 100vh; }
  .board { position: relative; overflow: hidden; }
  .rail { border-left: 1px solid #333; overflow-y: auto; padding: .5rem; }
  .transcript { grid-column: 1 / -1; border-top: 1px solid #333; overflow-y: auto; font-family: ui-monospace, monospace; font-size: .85rem; }
  .halted { position: absolute; inset: 0 auto auto 0; background: #b00; color: white; padding: .5rem 1rem; z-index: 10; }
</style>
```

Components, each thin:

- `Board.svelte` — `position: relative` box; one `Quadrant` per player at `quadrantFor(seat, n)` (absolute, 50 %×50 %, or halves for two seats) tinted with the seat colour at low alpha.
- `Quadrant.svelte` — three rows from `groupBattlefield(player.battlefield)`: lands (small), creatures (large), others; each a `CardTile`.
- `CardTile.svelte` — `data-obj={card.id}` (Task 22's arrows anchor on it); the text card: name, mana symbols, types, P/T for creatures; `tapped` rotates 15°; damage badge when `damage > 0`; counters as small chips; `summon_sick` dims. Task 23 replaces the face with the image when one is known.
- `IdentityBar.svelte` — absolutely positioned at the seat's outer corner; name (deck), big centred life, `library_size`/`hand_size`/`graveyard_size`, an outline when `active`, a dot when `priority`, struck through when `lost`.
- `Rail.svelte` — for each player with `hand !== null`: `HandList` (name + `ManaSymbols`, one line per card); then "Stack" with a `StackTile` per `view.stack` entry **top first** (the view lists bottom→top; reverse for display and say so in a comment); then "Pending" with the `PendingTray` (`view.pending`, label + "optional · decider N" when optional); then the current `decision` line ("Seat 2 · priority · You have priority.").
- `StackTile.svelte` — a band coloured by `kind` (spell / trigger / ability), name, text, and `targets` rendered as "→ {label}: {name or Seat N}" using `view` to resolve `obj` ids to names (a lookup over every visible card list — a rendering concern).
- `RecentStrip.svelte` — the card for `recentlyMattered(events)` rendered large in the board's bottom centre, or nothing.
- `Transcript.svelte` — one line per `dvr.events` entry: seq, line; the line whose seq equals `dvr.cursor` is highlighted and scrolled into view; clicking a line calls `onSeek(seq)`; lines with an empty `line` are skipped.

- [ ] **Step 4: Check against a real `gorged`**

Run: `cd web && npm run test && npm run check && npm run lint`. With `gorged -tables 1 -seats 4 -pace 700ms` and `npm run dev`, open `/t/t1`: four quadrants with cards appearing as lands are played; four identity bars with life; the rail lists four hands (omniscient) with mana symbols, the stack tile appears when a spell is cast and vanishes when it resolves, the pending tray fills when a trigger waits; the transcript scrolls with the match; the last resolved spell shows in the strip. Reload: the board is back within a second (fresh hello → focus → snapshot).

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat(web): focused table view — board, rail, identity bars, recent strip, transcript

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 21: the DVR bar, the view cache, and finished matches

**Files:**
- Create: `web/src/lib/viewcache.ts`, `web/src/lib/viewcache.test.ts`, `web/src/lib/turns.ts`, `web/src/lib/turns.test.ts`, `web/src/components/DvrBar.svelte`, `web/src/components/MatchList.svelte`
- Modify: `web/src/lib/match.svelte.ts` (cursor-driven view fetching, backfill, finished-match loading), `web/src/routes/Table.svelte` (DVR bar; finished mode; match list when idle)

**Interfaces:**
- Consumes: `fetchView`, `fetchEvents`, `fetchMatches`, `MatchInfo`, `dvr`.
- Produces:

```ts
// viewcache.ts
export class ViewCache { constructor(private load: (seq: number) => Promise<View>, private cap = 64); get(seq: number): Promise<View>; has(seq: number): boolean; clear(): void }
// turns.ts
export function turnStartsFrom(events: EventBody[]): number[]   // seqs of kind === 'turn'
// MatchState additions
loadFinished(k: number): Promise<void>     // no stream: events + head from /matches, turn starts derived
```

DVR behaviours (spec "DVR cursor"): **pause** stops the cursor; frames keep arriving; the badge shows how many events behind live. **Step** ±1 fetches `view?seq=N` (cached). **Scrub** ticks are the turn starts; dragging fetches the view at the chosen turn start. **Return to live** snaps to the head. A finished match is the same machinery with no stream.

- [ ] **Step 1: Write the failing tests**

`web/src/lib/viewcache.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { ViewCache } from './viewcache';
import type { View } from '../protocol';

const v = (seq: number) => ({ turn: seq } as unknown as View);

describe('ViewCache', () => {
  it('dedupes in-flight loads and caches results', async () => {
    let calls = 0;
    const c = new ViewCache(async (seq) => { calls++; return v(seq); });
    const [a, b] = await Promise.all([c.get(5), c.get(5)]);
    expect(a).toBe(b);
    expect(calls).toBe(1);
    await c.get(5);
    expect(calls).toBe(1);
    expect(c.has(5)).toBe(true);
  });
  it('evicts the least recently used beyond cap', async () => {
    const c = new ViewCache(async (seq) => v(seq), 3);
    for (const s of [1, 2, 3]) await c.get(s);
    await c.get(1); // touch 1
    await c.get(4); // evicts 2
    expect(c.has(1)).toBe(true);
    expect(c.has(2)).toBe(false);
    expect(c.has(3)).toBe(true);
    expect(c.has(4)).toBe(true);
  });
  it('does not cache failures', async () => {
    let n = 0;
    const c = new ViewCache(async () => { if (n++ === 0) throw new Error('x'); return v(1); });
    await expect(c.get(1)).rejects.toThrow();
    await expect(c.get(1)).resolves.toBeTruthy();
  });
});
```

`web/src/lib/turns.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { turnStartsFrom } from './turns';

describe('turnStartsFrom', () => {
  it('lists the seq of every turn event', () => {
    const ev = (seq: number, kind: string) => ({ event: { seq, kind, player: 0 }, line: '' });
    expect(turnStartsFrom([ev(0, 'game_start'), ev(4, 'turn'), ev(9, 'tap'), ev(30, 'turn')])).toEqual([4, 30]);
  });
});
```

Run: `cd web && npm run test -- viewcache turns` — FAIL.

- [ ] **Step 2: Implement**

`web/src/lib/viewcache.ts`:

```ts
import type { View } from '../protocol';

/** ViewCache memoises view-at-seq fetches with a small LRU, so stepping back and forth never refetches. */
export class ViewCache {
  private done = new Map<number, View>();
  private pending = new Map<number, Promise<View>>();
  constructor(private load: (seq: number) => Promise<View>, private cap = 64) {}

  has(seq: number) { return this.done.has(seq); }
  clear() { this.done.clear(); this.pending.clear(); }

  get(seq: number): Promise<View> {
    const hit = this.done.get(seq);
    if (hit) { this.done.delete(seq); this.done.set(seq, hit); return Promise.resolve(hit); }
    const inflight = this.pending.get(seq);
    if (inflight) return inflight;
    const p = this.load(seq).then((v) => {
      this.done.set(seq, v);
      if (this.done.size > this.cap) this.done.delete(this.done.keys().next().value as number);
      return v;
    }).finally(() => this.pending.delete(seq));
    this.pending.set(seq, p);
    return p;
  }
}
```

`web/src/lib/turns.ts`:

```ts
import type { EventBody } from '../protocol';

/** turnStartsFrom derives the scrub ticks from a finished match's events (a live match gets them in its snapshot). */
export const turnStartsFrom = (events: EventBody[]): number[] =>
  events.filter((e) => e.event.kind === 'turn').map((e) => e.event.seq);
```

`match.svelte.ts` additions:

```ts
  private cache = new ViewCache((seq) => fetchView(this.table, this.match!, seq));
  private seeking = 0;

  /** showCursor renders the view at the cursor (paused) and backfills the transcript when the cursor precedes the known events. */
  async showCursor() {
    if (this.match === null || this.dvr.live) return;
    const seq = this.dvr.cursor;
    const token = ++this.seeking;
    const first = this.dvr.events[0]?.event.seq ?? this.dvr.head + 1;
    if (seq < first) {
      const since = Math.max(0, seq - 200);
      const older = await fetchEvents(this.table, this.match, since).catch(() => []);
      this.dispatch({ type: 'backfill', events: older.filter((e) => e.event.seq < first) });
    }
    const v = await this.cache.get(seq).catch(() => null);
    if (v && token === this.seeking) this.view = v;
  }

  dispatch(a: DvrAction) {
    this.dvr = dvrReducer(this.dvr, a);
    if (a.type === 'snapshot' || a.type === 'reset') this.cache.clear();
    if (!this.dvr.live && a.type !== 'event') void this.showCursor();
    if (a.type === 'live') void this.refreshLive();
  }

  /** loadFinished renders a match that is not live: no subscription, everything from the JSON GETs. */
  async loadFinished(k: number) {
    const infos = await fetchMatches(this.table);
    const info = infos.find((m) => m.match === k);
    if (!info || info.events === 0) throw new Error(`no match ${k}`);
    this.match = k;
    this.seats = info.seats;
    const all = await fetchEvents(this.table, k, 0);
    this.dispatch({ type: 'snapshot', match: `${this.table}/${k}`, head: info.events - 1, turnStarts: turnStartsFrom(all) });
    this.dispatch({ type: 'backfill', events: all });
    this.dispatch({ type: 'pause' });
    this.dispatch({ type: 'scrub', seq: info.events - 1 });
  }
```

(Fetching every event of a finished match is fine for M2a: a 40-turn game is ~8 000 small objects.)

`DvrBar.svelte`:

```svelte
<script lang="ts">
  import type { DvrState } from '../lib/dvr';
  import { behindLive, turnOf } from '../lib/dvr';
  let { dvr, onAction, finished = false }: { dvr: DvrState; onAction: (a: import('../lib/dvr').DvrAction) => void; finished?: boolean } = $props();
  const turn = $derived(turnOf(dvr, dvr.cursor));
</script>

<div class="dvr" data-cursor={dvr.cursor} data-live={dvr.live}>
  <button onclick={() => onAction({ type: 'step', by: -1 })} aria-label="step back">⏮</button>
  {#if dvr.live && !finished}
    <button onclick={() => onAction({ type: 'pause' })} aria-label="pause">⏸</button>
  {:else if !finished}
    <button onclick={() => onAction({ type: 'live' })} aria-label="return to live">▶ live</button>
  {/if}
  <button onclick={() => onAction({ type: 'step', by: 1 })} aria-label="step forward">⏭</button>
  <input type="range" min="0" max={dvr.head} value={dvr.cursor} list="turn-ticks"
    oninput={(e) => onAction({ type: 'scrub', seq: Number((e.target as HTMLInputElement).value) })} aria-label="scrub" />
  <datalist id="turn-ticks">{#each dvr.turnStarts as t}<option value={t}></option>{/each}</datalist>
  <span class="badge" class:live={dvr.live}>
    {#if dvr.live}LIVE{:else}PAUSED · {behindLive(dvr)} behind{/if}
  </span>
  <span class="seq">seq {dvr.cursor} / {dvr.head} · turn {turn + 1}</span>
</div>

<style>
  .dvr { display: flex; gap: .5rem; align-items: center; padding: .25rem .5rem; background: #111; color: #ddd; }
  input[type=range] { flex: 1; }
  .badge { font-weight: 700; color: #f66; }
  .badge.live { color: #6f6; }
</style>
```

`MatchList.svelte` — a table of `MatchInfo` rows (k, state, result/winner name, turns, events) linking to `/t/{t}/m/{k}`; shown by `Table.svelte` under the board when the table has no live match, and as the whole page when the table is idle/halted. `Table.svelte` gains: the `DvrBar` above the transcript; `finished` mode when the `match` prop is set (`m.loadFinished(match)` instead of `session.focus`); and a scrub-by-turn also available from the transcript (clicking a "Turn N" line).

- [ ] **Step 3: Check against a real `gorged`**

Run: `cd web && npm run test && npm run check && npm run lint`. With `gorged -tables 1 -seats 4 -pace 500ms -perpetual=true -cooldown 3s`: on `/t/t1` press pause — the badge shows PAUSED and the behind count climbs; step back through a combat — the board changes at every step (tapped attackers, damage badges, a creature leaving); drag the scrubber to a turn tick — the board jumps; return to live — LIVE. Let a match finish: the match list appears; open `/t/t1/m/1` — the finished board at its last seq, scrub to the first turn, step forward. Restart `gorged` (same `-dir`) and reload `/t/t1/m/1`: the same match, served from disk.

- [ ] **Step 4: Commit**

```bash
git add web/src
git commit -m "feat(web): DVR bar, view cache, finished-match playback

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 22: the arrow overlay

**Files:**
- Create: `web/src/lib/arrows.ts`, `web/src/lib/arrows.test.ts`, `web/src/components/Arrows.svelte`
- Modify: `web/src/components/Board.svelte` (render `Arrows` last, above the quadrants), `web/src/components/IdentityBar.svelte` (`data-seat={seat}` anchor)

**Interfaces:**
- Consumes: `View` (`stack[].targets`, `players[].battlefield[].attacking_player`, `blocked_by`), DOM anchors `[data-obj]` and `[data-seat]`.
- Produces:

```ts
export type End = { obj: number } | { seat: number }
export interface Arrow { from: End; to: End; kind: 'target' | 'attack' | 'block' }
export function arrowsFor(view: View): Arrow[]     // pure, deterministic order: stack bottom→top, then seats, then cards by id
```

- [ ] **Step 1: Write the failing test**

`web/src/lib/arrows.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { arrowsFor } from './arrows';
import type { CardView, View } from '../protocol';

const card = (id: number, extra: Partial<CardView> = {}): CardView => ({ id, name: `c${id}`, types: 'Creature', tapped: false, power: 1, toughness: 1, damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false, printing: { name: `c${id}` }, token: `#${id}`, ...extra });

describe('arrowsFor', () => {
  it('draws target, attack and block arrows in a fixed order', () => {
    const view = {
      players: [
        { seat: 0, battlefield: [card(1, { attacking: true, attacking_player: 1, blocked_by: [2] })], hand: null, graveyard: [], exile: [], pool: null },
        { seat: 1, battlefield: [card(2)], hand: null, graveyard: [], exile: [], pool: null },
      ],
      stack: [{ id: 9, kind: 'spell', name: 'Bolt', text: '', controller: 1, targets: [{ player: 0, is_player: true, label: 'any' }, { obj: 1, player: 0, is_player: false }] }],
      pending: [],
    } as unknown as View;
    expect(arrowsFor(view)).toEqual([
      { from: { obj: 9 }, to: { seat: 0 }, kind: 'target' },
      { from: { obj: 9 }, to: { obj: 1 }, kind: 'target' },
      { from: { obj: 1 }, to: { seat: 1 }, kind: 'attack' },
      { from: { obj: 2 }, to: { obj: 1 }, kind: 'block' },
    ]);
  });
});
```

Run: `cd web && npm run test -- arrows` — FAIL.

- [ ] **Step 2: Implement**

`web/src/lib/arrows.ts`:

```ts
import type { View } from '../protocol';

export type End = { obj: number } | { seat: number };
export interface Arrow { from: End; to: End; kind: 'target' | 'attack' | 'block' }

/** arrowsFor reads relationships the server already resolved; it decides nothing about legality. */
export function arrowsFor(view: View): Arrow[] {
  const out: Arrow[] = [];
  for (const s of view.stack) {
    for (const t of s.targets) out.push({ from: { obj: s.id }, to: t.is_player ? { seat: t.player } : { obj: t.obj ?? 0 }, kind: 'target' });
  }
  for (const p of view.players) {
    for (const c of [...p.battlefield].sort((a, b) => a.id - b.id)) {
      if (c.attacking && c.attacking_player !== undefined && c.attacking_player !== null) out.push({ from: { obj: c.id }, to: { seat: c.attacking_player }, kind: 'attack' });
    }
  }
  for (const p of view.players) {
    for (const c of [...p.battlefield].sort((a, b) => a.id - b.id)) {
      for (const b of c.blocked_by ?? []) out.push({ from: { obj: b }, to: { obj: c.id }, kind: 'block' });
    }
  }
  return out;
}
```

`Arrows.svelte` — an absolutely positioned `<svg>` covering the board; on every `view` change and on `resize`, measure each end's anchor (`[data-obj="<id>"]` or `[data-seat="<n>"]`) with `getBoundingClientRect` relative to the board and draw a `<line>` with a marker-end arrowhead; colours: target amber `#f5a524`, attack red `#e5484d`, block blue `#3b82f6`; a missing anchor skips the arrow. Use `$effect` with a `requestAnimationFrame` so the DOM has laid out.

- [ ] **Step 3: Check and commit**

Run: `cd web && npm run test && npm run check && npm run lint`; with a paced `gorged`, step through a combat in the DVR: red arrows from attackers to the defender's identity bar, blue from blockers to attackers, amber from a spell on the stack to its target.

```bash
git add web/src
git commit -m "feat(web): SVG arrow overlay for targets, attacks and blocks

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 23: card images via Scryfall

**Files:**
- Create: `web/src/lib/images.ts`, `web/src/lib/images.test.ts`, `web/src/components/CardImage.svelte`
- Modify: `web/src/components/CardTile.svelte`, `web/src/components/RecentStrip.svelte`, `web/src/components/StackTile.svelte` (use `CardImage`)

**Interfaces:**
- Consumes: `printing.name` on `CardView`; `fetch`; `localStorage`.
- Produces:

```ts
export interface ImageSource { fetch: typeof fetch; now: () => number; setTimeout: (fn: () => void, ms: number) => void; storage: Storage | null }
export function createImages(src?: Partial<ImageSource>): { url(name: string): Promise<string | null>; offline: () => boolean }
export const images: ReturnType<typeof createImages>   // app singleton
```

Rules (spec "Images"): exact-name lookup through `https://api.scryfall.com/cards/named?exact=<name>`; the `normal` image (front face for a double-faced card); memoised in memory and `localStorage` (`gorge.img.<name>`; a 404 stores `""` so unknown names are not retried); requests spaced ≥ 100 ms apart (Scryfall asks for ≤ 10/s); any network failure marks the source offline for 60 s and the text card is shown with a muted badge.

- [ ] **Step 1: Write the failing tests**

`web/src/lib/images.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { createImages } from './images';

function fakeEnv(responses: Record<string, unknown | Error>) {
  const calls: string[] = [];
  let clock = 0;
  const timers: { at: number; fn: () => void }[] = [];
  const store = new Map<string, string>();
  const storage = { getItem: (k: string) => store.get(k) ?? null, setItem: (k: string, v: string) => void store.set(k, v) } as unknown as Storage;
  const env = {
    fetch: (async (url: string) => {
      calls.push(url);
      const name = decodeURIComponent(new URL(url).searchParams.get('exact')!);
      const r = responses[name];
      if (r instanceof Error) throw r;
      if (r === undefined) return new Response('{}', { status: 404 });
      return new Response(JSON.stringify(r), { status: 200 });
    }) as unknown as typeof fetch,
    now: () => clock,
    setTimeout: (fn: () => void, ms: number) => void timers.push({ at: clock + ms, fn }),
    storage,
  };
  const tick = (ms: number) => { clock += ms; for (const t of timers.splice(0)) if (t.at <= clock) t.fn(); else timers.push(t); };
  return { env, calls, tick, store };
}

describe('images', () => {
  it('resolves the normal image, caches in memory and storage, and treats 404 as a known miss', async () => {
    const { env, calls, store } = fakeEnv({ 'Goblin Guide': { image_uris: { normal: 'https://img/gg.jpg' } } });
    const im = createImages(env);
    expect(await im.url('Goblin Guide')).toBe('https://img/gg.jpg');
    expect(await im.url('Goblin Guide')).toBe('https://img/gg.jpg');
    expect(calls.length).toBe(1);
    expect(store.get('gorge.img.Goblin Guide')).toBe('https://img/gg.jpg');
    expect(await im.url('Nonexistent')).toBeNull();
    expect(await im.url('Nonexistent')).toBeNull();
    expect(calls.length).toBe(2);
  });
  it('uses the front face of a double-faced card', async () => {
    const { env } = fakeEnv({ 'Delver of Secrets': { card_faces: [{ image_uris: { normal: 'https://img/front.jpg' } }, { image_uris: { normal: 'https://img/back.jpg' } }] } });
    expect(await createImages(env).url('Delver of Secrets')).toBe('https://img/front.jpg');
  });
  it('spaces requests at least 100ms apart', async () => {
    const { env, calls, tick } = fakeEnv({ A: { image_uris: { normal: 'a' } }, B: { image_uris: { normal: 'b' } }, C: { image_uris: { normal: 'c' } } });
    const im = createImages(env);
    const all = Promise.all([im.url('A'), im.url('B'), im.url('C')]);
    await Promise.resolve();
    expect(calls.length).toBe(1);
    tick(100); await Promise.resolve(); await Promise.resolve();
    expect(calls.length).toBe(2);
    tick(100); await Promise.resolve(); await Promise.resolve();
    expect(calls.length).toBe(3);
    expect(await all).toEqual(['a', 'b', 'c']);
  });
  it('goes offline on a network error and recovers after 60s', async () => {
    const { env, calls, tick } = fakeEnv({ A: new Error('net down') });
    const im = createImages(env);
    expect(await im.url('A')).toBeNull();
    expect(im.offline()).toBe(true);
    expect(await im.url('B')).toBeNull();
    expect(calls.length).toBe(1);
    tick(60_000);
    expect(im.offline()).toBe(false);
  });
  it('works without storage', async () => {
    const { env } = fakeEnv({ A: { image_uris: { normal: 'a' } } });
    expect(await createImages({ ...env, storage: null }).url('A')).toBe('a');
  });
});
```

Run: `cd web && npm run test -- images` — FAIL.

- [ ] **Step 2: Implement `images.ts`**

```ts
export interface ImageSource {
  fetch: typeof fetch;
  now: () => number;
  setTimeout: (fn: () => void, ms: number) => void;
  storage: Storage | null;
}

const SPACING = 100;
const OFFLINE_FOR = 60_000;
const KEY = 'gorge.img.';

type Scryfall = { image_uris?: { normal?: string }; card_faces?: { image_uris?: { normal?: string } }[] };

/** createImages resolves exact card names to Scryfall image URLs with memory + localStorage caches, request spacing and an offline backoff. */
export function createImages(src: Partial<ImageSource> = {}) {
  const env: ImageSource = {
    fetch: src.fetch ?? ((...a) => fetch(...a)),
    now: src.now ?? (() => Date.now()),
    setTimeout: src.setTimeout ?? ((fn, ms) => void setTimeout(fn, ms)),
    storage: src.storage === undefined ? safeStorage() : src.storage,
  };
  const memo = new Map<string, string | null>();
  const pending = new Map<string, Promise<string | null>>();
  const queue: (() => void)[] = [];
  let lastAt = -Infinity;
  let offlineUntil = 0;

  const offline = () => env.now() < offlineUntil;

  function fromStorage(name: string): string | null | undefined {
    try { const v = env.storage?.getItem(KEY + name); return v === null || v === undefined ? undefined : v || null; } catch { return undefined; }
  }
  function toStorage(name: string, url: string | null) {
    try { env.storage?.setItem(KEY + name, url ?? ''); } catch { /* quota or private mode */ }
  }

  function schedule(run: () => void) {
    const wait = Math.max(0, lastAt + SPACING - env.now());
    if (wait === 0 && queue.length === 0) { lastAt = env.now(); run(); return; }
    queue.push(run);
    env.setTimeout(drain, wait || SPACING);
  }
  function drain() {
    const next = queue.shift();
    if (!next) return;
    lastAt = env.now();
    next();
    if (queue.length) env.setTimeout(drain, SPACING);
  }

  async function lookup(name: string): Promise<string | null> {
    const res = await env.fetch(`https://api.scryfall.com/cards/named?exact=${encodeURIComponent(name)}`, { headers: { Accept: 'application/json' } });
    if (res.status === 404) return null;
    if (!res.ok) throw new Error(`scryfall ${res.status}`);
    const j = (await res.json()) as Scryfall;
    return j.image_uris?.normal ?? j.card_faces?.[0]?.image_uris?.normal ?? null;
  }

  function url(name: string): Promise<string | null> {
    if (memo.has(name)) return Promise.resolve(memo.get(name)!);
    const stored = fromStorage(name);
    if (stored !== undefined) { memo.set(name, stored); return Promise.resolve(stored); }
    if (offline()) return Promise.resolve(null);
    const inflight = pending.get(name);
    if (inflight) return inflight;
    const p = new Promise<string | null>((resolve) => {
      schedule(() => {
        lookup(name).then((u) => { memo.set(name, u); toStorage(name, u); resolve(u); })
          .catch(() => { offlineUntil = env.now() + OFFLINE_FOR; resolve(null); });
      });
    }).finally(() => pending.delete(name));
    pending.set(name, p);
    return p;
  }

  return { url, offline };
}

function safeStorage(): Storage | null {
  try { return typeof localStorage === 'undefined' ? null : localStorage; } catch { return null; }
}

export const images = createImages();
```

`CardImage.svelte` — props `card: CardView`, `size: 'tile' | 'large'`; `$effect` resolves `images.url(card.printing.name)`; renders `<img>` when a URL exists, else the text card (name, `ManaSymbols`, types, P/T) with a muted "offline" badge when `images.offline()`. `CardTile`, `RecentStrip` and `StackTile` use it.

- [ ] **Step 3: Check and commit**

Run: `cd web && npm run test && npm run check && npm run lint`; with a paced `gorged` and network, images appear on the board within a few seconds and the console shows requests spaced ≥ 100 ms; reload — no new requests for known names (localStorage); with the network off (DevTools), text cards and the muted badge.

```bash
git add web/src
git commit -m "feat(web): card images by exact Scryfall name with caching, spacing and text fallback

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

### Task 24: Playwright against a real `gorged`, and the no-rules-knowledge test

**Files:**
- Create: `web/playwright.config.ts`, `web/e2e/overview.spec.ts`, `web/e2e/focus.spec.ts`, `web/e2e/dvr.spec.ts`
- Modify: `internal/archtest/arch_test.go` (client vocabulary), `Makefile` (`test-e2e-web`)

**Interfaces:**
- Consumes: the built `gorged` with the embedded client; a corpus at `.cards/`.
- Produces: `make test-e2e-web` (blocking where Node and the corpus exist); `TestClientHoldsNoRulesKnowledge`.

- [ ] **Step 1: The vocabulary test (Go)**

Append to `internal/archtest/arch_test.go`:

```go
// TestClientHoldsNoRulesKnowledge is the engine spec's M2 gate: the client
// renders views and lines the server computed; it never decides legality,
// timing, costs or targets. The vocabulary below is what such code would
// need to say. protocol.ts is generated and tests are exempt.
func TestClientHoldsNoRulesKnowledge(t *testing.T) {
	root := repoRoot(t)
	forbidden := regexp.MustCompile(`\b(legal|illegal|canCast|canAttack|canBlock|canPay|canTarget|isLegal|hasPriority|payMana|manaAvailable|resolveStack|applyEvent|stateBased|summoningSick(ness)?Check|CR\s?\d{3}\.\d)\b`)
	var checked int
	err := filepath.WalkDir(filepath.Join(root, "web", "src"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || (!strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".svelte")) {
			return nil
		}
		if strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, "protocol.ts") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		checked++
		for i, line := range strings.Split(string(src), "\n") {
			if m := forbidden.FindString(line); m != "" {
				t.Errorf("%s:%d: %q — the client must not reason about rules", path, i+1, m)
			}
		}
		return nil
	})
	if os.IsNotExist(err) {
		t.Skip("no web/src yet")
	}
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Skip("no client sources")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}
```

(imports: `io/fs`, `os`, `path/filepath`, `regexp`.) Run: `go test ./internal/archtest/ -count=1 -v` — PASS; then add `// legal` to a `.ts` file, run again — FAIL naming the file:line; revert.

- [ ] **Step 2: Playwright config and specs**

`web/playwright.config.ts`:

```ts
import { defineConfig } from '@playwright/test';

const port = 8099;
export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  retries: 0,
  use: { baseURL: `http://localhost:${port}`, trace: 'retain-on-failure' },
  webServer: {
    command: `cd .. && make web >/dev/null && CGO_ENABLED=0 go build -o bin/gorged ./cmd/gorged && exec ./bin/gorged -addr :${port} -tables 2 -seats 4 -pace 150ms -cooldown 2s -dir "$(mktemp -d)"`,
    url: `http://localhost:${port}/api/tables`,
    timeout: 180_000,
    reuseExistingServer: false,
  },
});
```

`web/e2e/overview.spec.ts`:

```ts
import { expect, test } from '@playwright/test';

test('overview shows two live widgets and a feed', async ({ page }) => {
  await page.goto('/');
  const cells = page.locator('[data-testid=table-cell]');
  await expect(cells).toHaveCount(2);
  await expect(cells.first().locator('[data-testid=life]')).toHaveCount(4);
  await expect(cells.first().locator('[data-testid=turn]')).not.toHaveText('', { timeout: 15_000 });
  await expect(page.locator('[data-testid=feed-line]').first()).toBeVisible({ timeout: 15_000 });
  await cells.first().click();
  await expect(page).toHaveURL(/\/t\/t1$/);
});
```

`web/e2e/focus.spec.ts`:

```ts
import { expect, test } from '@playwright/test';

test('focused table shows board, hands, transcript and survives a reload', async ({ page }) => {
  await page.goto('/t/t1');
  await expect(page.locator('[data-testid=quadrant]')).toHaveCount(4, { timeout: 15_000 });
  await expect(page.locator('[data-testid=identity]')).toHaveCount(4);
  await expect(page.locator('[data-testid=hand-list]')).toHaveCount(4);           // omniscient
  await expect(page.locator('[data-testid=transcript-line]').first()).toBeVisible();
  await expect(page.locator('[data-testid=card-tile]').first()).toBeVisible({ timeout: 30_000 }); // a land gets played
  const before = await page.locator('[data-testid=transcript-line]').count();
  await page.reload();
  await expect(page.locator('[data-testid=quadrant]')).toHaveCount(4, { timeout: 15_000 });
  await expect.poll(async () => page.locator('[data-testid=transcript-line]').count(), { timeout: 15_000 }).toBeGreaterThan(0);
  expect(before).toBeGreaterThan(0);
});
```

`web/e2e/dvr.spec.ts`:

```ts
import { expect, test } from '@playwright/test';

test('pause, step back, scrub, return to live', async ({ page }) => {
  await page.goto('/t/t1');
  const bar = page.locator('.dvr');
  await expect(bar).toHaveAttribute('data-live', 'true', { timeout: 15_000 });
  await page.waitForTimeout(3000); // let some events accrue
  await page.getByLabel('pause').click();
  await expect(bar).toHaveAttribute('data-live', 'false');
  const at = Number(await bar.getAttribute('data-cursor'));
  await page.getByLabel('step back').click();
  await page.getByLabel('step back').click();
  await expect(bar).toHaveAttribute('data-cursor', String(at - 2));
  await expect(bar.locator('.badge')).toContainText('behind');
  await page.getByLabel('scrub').fill('0');
  await expect(bar).toHaveAttribute('data-cursor', '0');
  await page.getByLabel('return to live').click();
  await expect(bar).toHaveAttribute('data-live', 'true');
});
```

Add `data-testid` attributes to the components named above (`table-cell`, `life`, `turn`, `feed-line`, `quadrant`, `identity`, `hand-list`, `transcript-line`, `card-tile`). `Makefile`:

```make
.PHONY: test-e2e-web
test-e2e-web:
	cd web && npx playwright install chromium && npm run e2e
```

- [ ] **Step 3: Run**

Run: `make test-e2e-web` — three specs PASS (needs `.cards/`; a machine without a corpus cannot run this target — say so in `help`). Then `go test ./internal/archtest/ -count=1`.

- [ ] **Step 4: Commit**

```bash
git add web/playwright.config.ts web/e2e internal/archtest/arch_test.go Makefile web/src
git commit -m "test(web): Playwright against a real gorged; client vocabulary gate

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
```

---

## Phase 5 — soak, docs, done-when

### Task 25: soak test, documentation, and the done-when walk

**Files:**
- Create: `host/soak_test.go`
- Modify: `Makefile` (`soak`), `README.md`, `AGENTS.md`, `docs/superpowers/specs/2026-09-04-gorge-m2a-tables-spectators-design.md` (an "Implementation notes" section recording PL-1..PL-17 by number with one line each)

- [ ] **Step 1: The soak test**

`host/soak_test.go`:

```go
package host

import (
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/rules"
)

// TestSoak runs four perpetual tables at zero pace for GORGE_SOAK (default
// 10m) and checks that goroutines and heap stay bounded and that every
// finished match replays from its files. Off unless GORGE_SOAK is set:
// `make soak`.
func TestSoak(t *testing.T) {
	dur := os.Getenv("GORGE_SOAK")
	if dur == "" {
		t.Skip("set GORGE_SOAK=10m to run")
	}
	d, err := time.ParseDuration(dur)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	o := testOptions(t)
	o.Dir = dir
	o.Cooldown = 0
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 4; i++ {
		id := TableID("t" + strconv.Itoa(i))
		cfg := fourSeatTable(id, true)
		cfg.Seed = uint64(i)
		if err := r.AddTable(cfg); err != nil {
			t.Fatal(err)
		}
	}
	// A subscriber that keeps up, so fan-out is exercised too.
	s := r.OpenSession()
	_ = r.Subscribe(s, TableAll, protocol.ModeOverview)
	go func() {
		for range s.Out() {
		}
	}()
	if err := r.StartAll(); err != nil {
		t.Fatal(err)
	}
	baseG := runtime.NumGoroutine()
	var ms runtime.MemStats
	deadline := time.Now().Add(d)
	var peak uint64
	var first uint64
	for time.Now().Before(deadline) {
		time.Sleep(30 * time.Second)
		runtime.GC()
		runtime.ReadMemStats(&ms)
		if first == 0 {
			first = ms.HeapInuse
		}
		if ms.HeapInuse > peak {
			peak = ms.HeapInuse
		}
		s.TakeWidgets()
		if g := runtime.NumGoroutine(); g > baseG+10 {
			t.Fatalf("goroutines grew from %d to %d", baseG, g)
		}
		t.Logf("heap %d MiB, goroutines %d", ms.HeapInuse>>20, runtime.NumGoroutine())
	}
	r.Close()
	if peak > 3*first {
		t.Fatalf("heap grew from %d to %d MiB", first>>20, peak>>20)
	}
	// Every finished match replays byte for byte.
	replayed := 0
	for i := 1; i <= 4; i++ {
		id := TableID("t" + strconv.Itoa(i))
		scs, err := readSidecars(dir, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, sc := range scs {
			if sc.State != protocol.MatchFinished {
				continue
			}
			l, err := readLog(dir, id, sc.Match)
			if err != nil {
				t.Fatal(err)
			}
			decks := make([][]*cards.Card, len(sc.Decks))
			for j, dn := range sc.Decks {
				dk, _ := o.LoadDeck(dn)
				decks[j] = dk.Cards
			}
			e, err := replay.Replay(l, rules.Config{Seed: sc.Seed, Names: sc.Names, Decks: decks})
			if err != nil || e.L.Head() != sc.Head {
				t.Fatalf("%s/%d does not replay: %v (head %s, sidecar %s)", id, sc.Match, err, e.L.Head(), sc.Head)
			}
			replayed++
		}
	}
	if replayed < 8 {
		t.Fatalf("only %d matches finished during the soak", replayed)
	}
}
```

`Makefile`:

```make
.PHONY: soak
soak:
	GORGE_SOAK=10m go test ./host/ -run TestSoak -count=1 -v -timeout 20m
```

Run: `GORGE_SOAK=2m go test ./host/ -run TestSoak -count=1 -v -timeout 10m` — PASS with the heap log lines flat; then the full `make soak` once and paste its last three log lines into the commit body.

- [ ] **Step 2: Documentation**

`README.md`: extend the package layout with `view`, `seat`, `replay`, `deck`, `protocol`, `host`, `host/httpapi`, `cmd/gorged`, `web/`; a "Running gorged" section (`make web && make gorged`, the URL, `gorged-data/`); a "Client development" section (`make web-dev` against a running `gorged`); the constraint that `time` is imported only by `host`, `host/httpapi` and `cmd/gorged`. `AGENTS.md`: add the new packages to the dependency order; the three `time` packages; `make lint` needs Node for the web checks and `gentypes`; `make test-e2e-web` needs `.cards/`; the M2a status paragraph (what runs, what is booked for M2b). Specs: an "Implementation notes" section in the M2a spec listing PL-1…PL-17 in one line each with the task that landed them.

- [ ] **Step 3: The done-when walk**

Every line below is checked by hand and its result recorded in the commit message of Step 4.

```sh
make lint && go build ./... && go test -count=1 ./... && go test -race -count=1 ./rules/ ./view/ ./replay/ ./host/ ./host/httpapi/
make sim | grep -c 'replay OK'                # 20
make report | grep '^cards:'                  # cards: 33667  playable: 15265 (45.3%)
go test ./rules/ -run TestRepoDeckGamesReplayExactly -count=1 -v | grep -o '[0-9a-f]\{16\}' | sort -u   # the four chain heads
git ls-files | grep -c '\.txt$'               # 0
make web && make build
rm -rf /tmp/gorged-walk && ./bin/gorged -decks internal/testutil/decks -tables 4 -seats 4 -pace 1.5s -dir /tmp/gorged-walk
```

In a browser at `http://localhost:8080/`: four live widgets; focus one → omniscient board, four hands, stack, pending tray, transcript, card images; pause, step back through a combat, scrub to the previous turn, return to live; reload mid-match → backfilled within a second; wait for a match to end (or run a second `gorged` at `-pace 0` into another dir) → `Ctrl-C`, restart with the same `-dir`, open the finished match from the list and replay it. `make test-e2e-web` green. `make soak` green.

- [ ] **Step 4: Commit and push**

```bash
git add host/soak_test.go Makefile README.md AGENTS.md docs/superpowers/specs/2026-09-04-gorge-m2a-tables-spectators-design.md
git commit -m "docs+test: M2a soak test, documentation, done-when walk

<paste: soak's last three heap lines; the four chain heads; sim 20/20; report 45.3%; e2e 3/3>

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>"
git ls-files | grep -c '\.txt$'   # must print 0 before pushing
git push origin main
```

---

## Self-review checklist (run by the plan author before execution)

1. **Spec coverage.** Goals → Tasks 9–13 (`gorged` runs N tables, restarts each), 19–23 (overview, focus, DVR, images), 10/15 (late join, reconnect), 12 (finished matches after restart), 9/14 (`host` importable without `gorged`), 6 (stdlib only, `time` fenced), 24 (no rules knowledge). D9→9,14,16; D10→15; D11→1,11; D12→3; D13→8,17; D14→9; D15→13; D16→6,9; view additions→3,4,5 (+PL-17); protocol frames/requests→7,14,15; data flow→10,15,20,21; error handling→10,13,14,15,23; testing rows→3,11,7/8,9/12/13,15,17–24; done-when→25.
2. **Placeholders.** No "TBD"; every code step shows code. Stubs in Tasks 9 and 14 are named and replaced by a later task in the same plan.
3. **Type consistency.** `host.TableID`/`Deck`/`Options`/`Registry` names match across 9–16; `protocol` names match 7/8/10/14/15/17; `view.ProjectFor`/`RedactEventsFor`/`NoSeat`/`Visibility`/`Describe`/`PhaseOf` match 3/5/10/11; `DvrState`/`DvrAction` match 18/20/21; `data-testid`s match 20/24.
