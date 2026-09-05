# gorge M2b + M2c — the library-first follow-up plan

Date: 2026-09-05. Status: planning output (this document makes the library-first
half of the post-M1 roadmap implementable). Supersedes nothing yet — it *specifies*
the M2b/M2c rows of `2026-09-04-gorge-post-m1-roadmap.md` so they can be worked
task-by-task. Every other row of the roadmap, and every section of the engine spec
`2026-09-03-mtgcore-go-engine-design.md` not amended here, still binds.

> **For agentic workers:** implement this plan task-by-task. Each numbered
> task ends in a failing-then-passing test and a commit. Steps use `- [ ]`.
> The implementer is bound by the module invariants in `AGENTS.md` and the
> verification gates in the M2a plan's "Global Constraints" (this plan adds no
> event kind, no `events.Event` field, no new package to the right of `host`).

---

## 1. Context

The user has **deprioritised the standalone binary** (`cmd/gorged`) and
refocused the project on **gorge as an embeddable library**: the in-process
`host` registry is the product, and the real first consumer is mtgserve
(`mtgbld`). That changes the roadmap's intent for three rows:

| Roadmap item | Status under this plan | Why |
|---|---|---|
| **M2c** (mtgserve integration) | **Pulled earlier and planned now, engine-side only.** The plan below is what lets mtgserve embed `host` in-process. The mtgserve-side migration is described in "Outside gorge". | The library face has a load-bearing gap (no per-seat view, no human seat), so M2c cannot start until the engine/library half of M2b lands. |
| **M2b** (player seat) | **Split.** This plan implements the **engine + `host` + `seat` half only**. The browser client (Svelte seat screen, the "human beats a scripted bot" user-facing gate) is explicitly **out of scope** and deferred to mtgserve's own UI. | `cmd/gorged` is deprioritised, so M2b's *client* delivery now belongs to mtgserve's web, not to `web/`. The *library* mechanism — a seat that can hold a decision pending and be answered — must exist here and be testable without a browser. |
| **`cmd/gorged`**, the **Playwright / e2e task** (M2a Task 24), the **soak task** | **Deferred** (this plan still keeps `cmd/gorged` compiling; it just stops being the milestone's delivery). | These exist to watchdog a *standalone server*. As a library, the soak/e2e story moves to mtgserve's deployment gates. Nothing in this plan deletes them; they are simply not the acceptance path. |

The load-bearing requirement the roadmap already states for M2c stands and is
made **true by construction** here (§3, D6): *"the mtgbld snapshot god-view
leak is impossible by construction because every payload is a `view.Project`
for a seat."*

---

## 2. Current surface

This section is the evidence base for the tasks. Every claim is file:line in
this worktree.

### What `host` exports today

Package `host` (`host/doc.go`) is the library surface. All of it flags `Options`
or `*Registry` methods:

- **`host.Options`** — `host/registry.go:17-44`. Already carries: `Dir`,
  required `LoadDeck func(name) (Deck, error)`, `Tokens`, injected `Sleep`,
  `Seats func(names, seed) []seat.Seat`, `Sync`, `Ring`, `Cooldown`,
  `MaxIntents`. This is the whole embedder knob surface; there is **no**
  observer/lifecycle/callback field and no human-seat or think-timeout knob.
- **`host.New(Options) (*Registry, error)`** — `registry.go:70`.
- Table lifecycle: `AddTable(TableConfig) error` (`registry.go:93`),
  `Start(id)` / `StartAll()` (`registry.go:110` / `133`), `Wait(id)`
  (`:145`), `Close()` (`:286`), `Done() <-chan struct{}` (`:306`).
- Reading: `Tables() []protocol.TableInfo` (`:239`), `Matches(id)
  ([]protocol.MatchInfo, error)` (`:254`), `ViewAt(id,k,seq) (view.View, error)`
  (`host/viewat.go:31`), `Events(id,k,since) ([]protocol.EventBody, error)`
  (`host/viewat.go:61`).
- Sessions/streams: `OpenSession() *Session` (`session.go:36`), `Session(id)`
  (`:47`), `CloseSession(id)` (`:55`), `Subscribe`/`Unsubscribe`
  (`session.go:213` / `261`), `Hello` (`:202`).
- Types: `TableID`, `TableConfig` (`host/table.go:30-40` — `ID Name Seats Decks
  Seed Pace Spectator Perpetual`), `Deck{Name,Cards}` (`table.go:21-26`),
  `MatchSeed` (`host/seed.go:7`).
- `host/httpapi` is **one consumer**, not the library: `httpapi.NewHandler(*Registry, Options) http.Handler` (`host/httpapi/handler.go:47`) over SSE + REST. `host` itself never imports `net/http` (archtest pins the dependency order).

### What `seat`, `view`, `decision`, `replay`, `protocol` give an embedder

- **`seat.Seat`** — `seat/seat.go:18`:
  `Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error)`. `host/play` blocks on it at `host/match.go:187-205`: it calls `seats[d.Player].Decide(ctx, v, dc)` (with `v = view.Project(...)`, `match.go:214`) and only then `m.e.Submit(in)`. **The engine therefore already computes the full seat view and the full decision (with every `Option`) for the human seat and hands them to `Decide`** — the only missing piece is a `Decide` that can *block* and be *answered from outside*.
- **`decision.Decision / Option / Intent`** — `decision/decision.go`. A decision carries `Seq Player Kind Prompt Min Max Options`; `Option` carries `Index Kind Label Obj Player` (server-only `Attacker/Mode/AltCostIndex/Amount` are `json:"-"`); an answer is `Intent{Seq, Player, Choices []int}` (indices only — no rules knowledge is required to answer). `Decision.Validate(in)` (`decision.go:~135`) is the single enforcement gate.
- **`view.View`** — `view/view.go:47`: `Viewer Visibility Turn Step Phase Active Priority Over Draw Winner Players Stack Pending Decision`. `PlayerView` (`view/view.go:88`) carries `LibrarySize/HandSize/GraveyardSize` counts and, only for the viewer's own seat, `Hand []CardView` / `Pool`. **Structurally there is no field that can carry a hidden zone or a `*state.Game`**.
- **`view.Project` / `ProjectFor` / `RedactEventsFor`** — `view/view.go:202`, `view/visibility.go:75`/`111`. `ProjectFor(g, ch, viewer, vis, d)` is the **only** function that turns a `*state.Game` into a payload, under `vis ∈ {Seat, Public, Omniscient}` and viewer `NoSeat = 255` (`visibility.go:67`). Omniscient projection still hides library order (visibility_test pins it).
- **`protocol`** — `protocol/protocol.go`: envelope + frame types + `DecisionBody{Player,Kind,Prompt}` (`protocol.go:153`, whose doc comment already says *"options come with the player seat (M2b), not here"*). `host/fanout.go:140` broadcasts a `TDecision` frame with that body.
- **`replay.Replay / ReplayTo`** — `replay/replay.go:105`/`179`: deterministic rebuild from `(Log, rules.Config)`; what a DVR/finished-match path uses (`host/viewat.go:163`).

### Honest assessment of what an embedder (mtgserve) is missing

1. **No per-seat (player) view.**
   `ViewAt(id,k,seq)` projects at the *table's spectator visibility* (`t.cfg.Spectator`) with viewer `NoSeat` — `host/viewat.go:46` reads `vis := t.cfg.Spectator`, and the projection is `view.ProjectFor(e.G, e, view.NoSeat, vis, nil)` (`viewat.go:249`). There is **no way to get the `Seat`-visibility view a *player* sees**, and no way to ask "is the current seq a decision for player p and, if so, what are its options?" as a *library call*.
2. **No human seat.**
   `Decide` is synchronous; `defaultSeats` (`match.go:163`) is all bots. A seat that blocks until a player answers, with a timeout/disconnect policy, does not exist, and `play()` has no path to wake one.
3. **`Options.Seats` builds seats up-front and discards them.**
   `play()` creates `seats := r.opts.Seats(...)` as a local (`match.go:187`) and it is not stored on the match, so no registry API can reach a seat to query/poke it after the match starts.
4. **Persistence is all-or-nothing, file-shaped, and restart aborts live matches.**
   `host` persists to a `Dir` of flat files (`host/persist.go`, `tables.json` + `<k>.events`/`<k>.intents`/sidecar) and `restart.go:16` rewrites in-progress matches to `aborted`. An in-memory run is already supported (`Dir=""` → `New` skips `load`, and `persistBurst` is a no-op when `m.files == nil`, `host/snapshot.go:72-89`), but there is **no observer hook** to copy events/intents into mtgserve's own SQLite, and no lifecycle callback to flip mtgserve's match row at start/finish.
5. **No structural god-view guard.**
   Nothing today *prevents* a future exported method from returning `*rules.Engine` or `*state.Game`. The current surface happens to be leak-free (everything returns `view.View`/`protocol.*`), but the guarantee is convention, not construction (§3, D6 makes it construction).
6. **`host/httpapi` is summit-shaped, not mtgserve-shaped.**
   Its SSE `id:` session-ring resume (`httpapi/sse.go`) and `protocol.Frame` bodies are tuned for `web/`, not for mtgserve's own WS/session/auth model. mtgserve should embed `*host.Registry`, not `host/httpapi` (§3, D4).

---

## 3. Design decisions

Each is `Decision: <what> — <why> — <cost if wrong>`.

- **D1 — Pending decision is a single blocking slot on a `*host.HumanSeat`.**
  A new `*host.HumanSeat` implements `seat.Seat`. It holds at most one
  `pendingSlot{dec decision.Decision, recv chan decision.Intent}`. `Decide(ctx,
  v, d)` stores `d` in the slot and blocks reading `recv`, selecting on
  `ctx.Done()`. The engine keeps its shape unchanged: `play()` already hands the
  seat its full `view.View` + `decision.Decision` (`match.go:214`), so the human
  seat receives everything it needs to render/answer without any legality
  logic. — *Why:* `seat.Seat` is already the sanctioned exit; a human is just
  another implementer. No new event kind, no change to the engine, `host`
  already owns blocking (it blocks on `Decide` for every bot today). — *Cost if
  wrong:* the match goroutine parks on a channel; that is the existing design
  (`play` blocks on `Decide` regardless), so the cost is bounded by the
  timeout/cancel story in D3.

- **D2 — `host.SubmitIntent` validates before it wakes.**
  `(*Registry).SubmitIntent(id, k, player, intent) error` resolves to the
  matching match's `HumanSeat`, checks `intent.Seq == pending.Seq` (a stale
  answer must not clobber a decision that has since turned), runs
  `pending.Validate(intent)` (rejects wrong player / wrong option index / wrong
  min-max / duplicates), and only then pushes to the slot's `recv`, unblocking
  `Decide`. — *Why:* `decision.Decision.Validate` is the single enforcement
  gate and already rejects everything a client can get wrong; duplicating it
  here keeps a stale/out-of-order HTTP answer from poisoning a decision the
  engine has already moved past. — *Cost if wrong:* validation runs at two
  sites (seat + engine's own `e.Submit`) — cheap, and both are the same
  predicate.

- **D3 — Timeout/disconnect substitutes a deterministic caretaker bot.**
  On `Options.ThinkTimeout` (a host clock read — `host` is the package allowed
  `time`) or on `ctx` cancellation, the `HumanSeat` must not wedge the match
  goroutine. It falls back to the deterministic seated bot (the one
  `Options.Seats`/`defaultSeats` would have built for that seat, seeded per
  PL-14 from `MatchSeed`), and that bot's intent is logged and submitted like
  any other intent — so a human timeout produces **exactly the log a bot game
  would**, and `replay.Replay` is byte-identical. The player may reconnect and
  resume via `SubmitIntent`. — *Why:* no wall clock or ambient randomness may
  reach the log/view/replay; only committed *intents* replay, so a caretaker
  intent is deterministic. And a dead/disconnected human must never hang the
  loop — `registry.go:run`/`match.go:play`'s own comments single out exactly
  this as the "disconnected human" wedge (FL-17). — *Cost if wrong:* human
  churn becomes bot turns the player must live with; the alternative (a true
  in-place pause/resume of a live game) is a much larger M2c+ feature that the
  roadmap defers to M5, so the caretaker is the honest MVP.

- **D4 — mtgserve embeds `*host.Registry`, not `host/httpapi`.**
  mtgserve consumes the library through its own thin internal adapter
  (mtgserve-side, "Outside gorge") that exposes only seat-scoped methods. The
  `host/httpapi` web build, SSE `<session>:<frame>` ring and `protocol.Frame`
  bodies are one consumer tuned for `web/`; mtgserve already owns its client
  transport (it is how mtgplay's WS seat attaches today), its session manager
  (`scs`) and its auth. — *Why:* forcing mtgserve's WS onto `protocol.Frame` /
  session-ring resume is more work and more coupling than writing mtgserve's
  own ~100-line adapter over the new host methods, and it keeps gorge's wire
  types optional. — *Cost if wrong:* mtgserve writes a thin transport adapter
  instead of reusing `host/httpapi`; the cost is small and the separation keeps
  the god-view guarantee inspectable (§3, D6).

- **D5 — Persistence stays in mtgserve's SQLite; `host` is run in-memory; the
  observer copies the log.**
  mtgserve runs `host.New(Options{Dir: ""})` (in-memory; `restart.go`/`persist
  .go` become no-ops) and persists to its own `matches` / `match_seats` /
  `match_events` tables by observing the engine through a new `host.Options`
  callback (Task 6). `host`'s own `Dir` flat-file persistence (`.events /
  .intents / tables.json`) stays for `gorged` and for tests, and stays the
  load-bearing *replay* source (`replay.Replay` over `host.ViewAt`). — *Why:*
  accounts, deck pairing and match history are mtgbld's rows; asking gorge to
  also own a filesystem is a second source of truth mtgserve would have to
  reconcile. The observer keeps the feed identical to today's mtgplay bridge
  (which already POSTs each event/seq to the DB), and keeps `match_events`
  replayable later by `replay.Replay`. — *Cost if wrong:* mtgserve must keep
  `match_events` in lockstep with the observer (same invariant as today's
  bridge), and if mtgserve later wants midpoint scrubbing it does it through
  `host.ViewAt` with the stored log rather than mtgbld's god-view snapshots.

- **D6 — The god-view guarantee is structural: "the projection is the only
  exit".**
  Three independently-enforced facts make the mtgbld god-view leak impossible
  *by construction*:
  1. `view.ProjectFor` (`view/visibility.go:75`) is the **only** function that
     turns a `*state.Game` into a payload, and `view.View` (`view/view.go:47`)
     is the only payload type — which structurally cannot carry a hidden zone
     or a raw `state.Game` (a `PlayerView` has counts + derived `CardView`s;
     Omniscient mode still hides library order; `visibility_test.go` pins it).
  2. `host` exposes **no** exported symbol whose signature can return or expose
     a `*state.Game` or `*rules.Engine`. `ViewAt` → `view.View`, `Events` →
     `[]protocol.EventBody` redacted via `RedactEventsFor`. The new methods in
     this plan (`Pending` → `*decision.Decision`, `ViewAtSeat` → `view.View`,
     `SubmitIntent` → `error`) all keep that property.
  3. An **archtest** (extends `internal/archtest/arch_test.go`) greps every
     exported `host`/`host/httpapi` signature for `*state.Game`/`*rules.Engine`
     in the result type or reached by a `.(type)` assertion, **and** a runtime
     test walks every exported `*Registry` method and asserts each result is
     `view.View`/`protocol.*`. If anyone ever adds an engine accessor, the arch
     test fails and blocks merge.
  Because gorge hands an embedder *no* unprojected state, mtgbld cannot serve a
  god view even if its own handlers regress — there is nothing to serve.
  (turning a god-view payload into a per-seat `view.View` is described in
  "Outside gorge".) — *Cost if wrong:* if someone needs raw engine access the
  archtest blocks them; that is the point, and the tested escape hatch is
  `replay.Replay` (`replay/replay.go:105`) which is explicit and library-only.

- **D7 — One goroutine per match is preserved; the human seat adds no driver.**
  `SubmitIntent` (and the caretaker on timeout) only **wake** a blocked seat;
  the actual `m.e.Submit` still runs inside `play()` on the match goroutine
  under `m.mu`. Readers (`Pending`, `ViewAtSeat`, `Events`) take `RLock` and
  never call `Submit`. — *Why:* the hash-chained log, view redaction and D6 all
  assume a single writer; letting an HTTP goroutine drive `Submit` would break
  it. — *Cost if wrong:* a wake-then-submit indirection (bounded per-seat
  channel, no new concurrency).

- **D8 — `TableConfig` gains an explicit human-seat plan; bot autoplay becomes
  opt-out per match.**
  A human match is single-shot and paced by `ThinkTimeout`, not by bot
  autoplay. `TableConfig` (extend `host/table.go:30-40`, re-validate at
  `table.go:42`) gets a `Humans []int` (the slots that are `*HumanSeat`) so
  `newMatch`/`play` build the right `Seats` and the run loop can be told a match
  is human-driven. Non-perpetual human tables end at game over (today's `run`
  already does single-shot for non-perpetual). — *Why:* `Options.Seats` already
  lets an embedder choose bot-vs-human at build time, but nothing in
  `TableConfig` records *which* seats are human, so a persisted table or a
  restart couldn't reproduce them, and `run`/`restart` currently assume bot
  autoplay. — *Cost if wrong:* a persisted-table field; mtgserve mostly passes
  2-seat single-shot configs, so the blast radius is small.

---

## 4. Open questions

Things this plan could not resolve from the code; an implementer must not guess.

1. **Per-match seed source in mtgserve.** `host.MatchSeed(tableSeed, k)`
   (`host/seed.go:7`) makes a table's whole history a pure function of a single
   `table.Seed`. mtgserve's `Create`/`Join` (`matches/store.go:104`/`225`) have
   no seed column today. mtgserve must either persist a random seed at match
   creation (host can't read wall clock for it, but mtgserve's creator can draw
   one) or derive a determinism-preserving seed from stable ids. Unresolved:
   which, and where it's stored. This is mtgserve work, but it gates whether a
   human match is replayable from mtgserve's DB.
2. **Legacy `match_snapshots` and the replay UI.** mtgserve persists god-view
   snapshots (`snapshots.go:45`) and serves them to authenticated seat-holders
   (`handlers.go:695`). After M2c, does mtgserve (a) migrate these legacy
   snapshots as read-only history and stop writing new ones, or (b) keep them
   for already-finished matches and serve *new* matches purely from `host.ViewAt`
   + gorge's log? The plan targets (b); the migration of historical rows is
   unspecified.
3. **Think-timeout default and per-decision scaling.** A priority decision
   deserves a long timer; an `KChoose`/`KTriggerOptional` maybe shorter. This
   plan adds a single `Options.ThinkTimeout`; whether mtgserve wants
   per-`decision.Kind` budgets is its call and is deliberately not planned here.
   The caretaker must be **legal** for the pending decision kind (yield for
   `KPriority`; the seated bot already is), which the caretaker inherits by
   being the real bot.
4. **Whether a departed human must be allowed to rejoin the *same* decision.**
   `SubmitIntent` rejects a stale `Seq`. With a caretaker active past the
   timeout, the pending slot has been consumed (the bot's intent was submitted).
   A reconnecting player must wait for the *next* decision — or mtgserve holds
   reconnect differently. The client retry/`Seq` semantics are mtgserve's.
5. **Multi-human tables.** mtgplay today is effectively 1 human + AI/other.
   This plan's `Humans []int` permits several, but the timeout/disconnect policy
   for *two or more* humans (does one timeout pause all?) is not settled; M2c
   targets single-human-first and treats the rest as untested surface.
6. **Commander.** mtgserve's `deckPayload` is commander-aware
   (`matches/handlers.go:27-80`); gorge's commander support is M2d. M2c must
   gate to non-Commander formats. Unresolved: whether the adapter rejects
   commander decks or silently maps commanders into the mainboard (rejected
   candidate).

---

## 5. Tasks

Sized for one implementer in one worktree; each ends in a test that fails before
the change and passes after. Tasks M2b-1..M2b-5 are the engine/library half of
M2b (client out of scope). Tasks M2c-1..M2c-4 are the gorge side of M2c. Work on
the mtgbld consumer is "Outside gorge" below.

### M2b — a human at a seat (engine/library half)

#### Task M2b-1: `host.HumanSeat` — a blocking, externally-answerable seat

**Files:**
- Create: `host/humanseat.go`, `host/humanseat_test.go`

**Interfaces:**
- Consumes: `seat.Seat` (`seat/seat.go:18`), `view.View`, `decision.Decision`/`Intent`, `context`.
- Produces: `type HumanSeat` (field of `host`), `func NewHumanSeat() *HumanSeat`, `func (*HumanSeat) Decide(ctx, v, d) (Intent, error)` (satisfies `seat.Seat`), plus the package-internal slot accessors `(*HumanSeat) pending() (*decision.Decision, bool)` and `(*HumanSeat) submit(in decision.Intent) error` that Task M2b-2 uses.

**Steps** (tests first):

- [ ] **Step 1 — failing test.** Create `host/humanseat_test.go`:
  ```go
  func TestHumanSeatBlocksUntilSubmittable(t *testing.T) {
      s := NewHumanSeat()
      d := decision.Decision{Seq: 3, Player: 1, Kind: decision.KPriority, Min: 1, Max: 1,
          Options: []decision.Option{{Index: 0, Label: "pass"}, {Index: 1, Label: "act"}}}
      done := make(chan decision.Intent, 1)
      errc := make(chan error, 1)
      go func() {
          in, err := s.Decide(context.Background(), view.View{}, d)
          if err == nil {
              done <- in
          } else {
              errc <- err
          }
      }()
      // No answer yet: Decide must be blocked.
      select {
      case <-done:
          t.Fatal("Decide returned before an intent was submitted")
      case <-time.After(20 * time.Millisecond):
      }
      if ok, got := s.pending(); !ok || got.Seq != 3 {
          t.Fatalf("pending() = %v, %+v", ok, got)
      }
      if err := s.submit(decision.Intent{Seq: 3, Player: 1, Choices: []int{0}}); err != nil {
          t.Fatalf("submit: %v", err)
      }
      select {
      case in := <-done:
          if len(in.Choices) != 1 || in.Choices[0] != 0 {
              t.Fatalf("intent %+v", in)
          }
      case <-time.After(1 * time.Second):
          t.Fatal("Decide did not return after submit")
      }
  }

  func TestHumanSeatSubmitRejectsWrongSeqAndUnblocksOnCancel(t *testing.T) {
      s := NewHumanSeat()
      d := decision.Decision{Seq: 9, Player: 0, Min: 1, Max: 1, Options: []decision.Option{{Index: 0}}}
      ctx, cancel := context.WithCancel(context.Background())
      errc := make(chan error, 1)
      go func() { _, err := s.Decide(ctx, view.View{}, d); errc <- err }()
      if err := s.submit(decision.Intent{Seq: 8, Player: 0, Choices: []int{0}}); err == nil {
          t.Error("submit with a stale Seq was accepted")
      }
      cancel()
      select {
      case err := <-errc:
          if err == nil {
              t.Fatal("cancel did not error Decide")
          }
      case <-time.After(1 * time.Second):
          t.Fatal("cancel did not unblock Decide")
      }
  }
  ```
  (`time` in a `host` test file is allowed — `host` is the package that may import `time`; the archtest walks non-test files.)
- [ ] **Step 2 — run to verify it fails.** `go test ./host/ -run TestHumanSeat -count=1` → FAIL (`NewHumanSeat undefined`).
- [ ] **Step 3 — implement** `host/humanseat.go`:
  ```go
  type HumanSeat struct {
      mu   sync.Mutex
      p    *decision.Decision
      recv chan decision.Intent
  }
  func NewHumanSeat() *HumanSeat { return &HumanSeat{recv: make(chan decision.Intent, 1)} }
  func (s *HumanSeat) Decide(ctx context.Context, _ view.View, d decision.Decision) (decision.Intent, error) {
      c := decision.Intent{}
      s.mu.Lock(); s.p = &d; s.mu.Unlock()
      select {  // Task M2b-3 wires ctx to the caretaker/think-timeout.
      case <-ctx.Done():
          return c, ctx.Err()
      case in := <-s.recv:
          return in, nil
      }
  }
  ```
  plus the `pending()` / `submit()` accessors (submit only accepts a matching `Seq`; it returns `decision.Validate`-style errors).
- [ ] **Step 4 — run to pass.** `go test ./host/ -run TestHumanSeat -count=1` → PASS.
- [ ] **Step 5 — gates & commit.** `gofmt -l . && go vet ./host/ && go build ./... && go test -count=1 ./host/`. Commit:
  `feat(host): HumanSeat — a seat that can hold a decision pending and be answered`.

#### Task M2b-2: `host.Pending`, `host.SubmitIntent`, and seats reachable

**Files:**
- Modify: `host/match.go` (store the built `seats` slice on `match`; add the two exported methods or put them in a new `host/action.go`)
- Create: `host/action.go`, `host/action_test.go`

**Interfaces:**
- Consumes: `Registry.lookup`/`TableConfig`/`match`, `seat.Seat`, `decision`.
- Produces: `func (r *Registry) Pending(id TableID, k int, player state.PlayerID) (*decision.Decision, error)`; `func (r *Registry) SubmitIntent(id TableID, k int, player state.PlayerID, in decision.Intent) error`.

**Steps:**

- [ ] **Step 1 — failing test** (`host/action_test.go`): build a 2-seat table through `New(Options{...})` + `AddTable` + `Start`, with `Options.Seats` returning one `NewHumanSeat()` at slot 0 and a bot at slot 1; wait until `Pending(t, 1, 0)` returns a non-nil decision; submit a legal intent via `SubmitIntent`; assert it was accepted and the game advanced. A companion test submits a stale/illegal intent and asserts a non-nil error and that the game did not move.
- [ ] **Step 2 — run to verify it fails.** `go test ./host/ -run 'TestPending|TestSubmitIntent' -count=1` → FAIL (no such methods).
- [ ] **Step 3 — implement.** In `host/match.go`, `play()` already builds `seats := r.opts.Seats(m.cfg.Names, m.seed)` (`match.go:187`); store it on `match` under `m.mu` (`m.seats = seats`) so registry methods can reach it. In `host/action.go`: `Pending` locks `m.mu`, finds `m.seats[player].(*HumanSeat)`, reads `pending()` under the seat mutex, and returns a copy of `*decision.Decision`. `SubmitIntent` validates `in.Seq`/`in.Player` against the pending decision (D2) and calls `seat.submit` — which *wakes* the blocked `Decide`; the actual `m.e.Submit` still happens inside `play()` (D7).
- [ ] **Step 4 — run to pass.** `go test ./host/ -run 'TestPending|TestSubmitIntent' -count=1` and the whole `./host/`.
- [ ] **Step 5 — gates & commit.** `gofmt -l . && go vet ./host/ && go build ./... && go test -count=1 ./host/`. Commit:
  `feat(host): Pending/SubmitIntent — answer a human seat from outside the loop`.

#### Task M2b-3: think-timeout caretaker and `Options.ThinkTimeout`

**Files:**
- Modify: `host/registry.go` (add `ThinkTimeout time.Duration` to `Options`), `host/match.go` (`play` honours it), `host/humanseat.go` (the caretaker path)
- Create: `host/caretaker_test.go`

**Interfaces:**
- Produces: `Options.ThinkTimeout time.Duration`; a deterministic caretaker that is exactly the already-seeded bot (`Options.Seats`/`defaultSeats`, `match.go:163`) for the same slot.

**Steps:**

- [ ] **Step 1 — failing test** (`host/caretaker_test.go`): a table with a `HumanSeat` at slot 0, `Options.ThinkTimeout` ~30 ms, and a *legal* bot fallback; start the match, *do not* answer; assert the pending decision is eventually turned by the caretaker bot (the log head advances) and that **replaying the resulting `Log` via `replay.Replay` produces the identical chain head** (determinism — D3). Against a very small `ThinkTimeout`, the per-match `MaxIntents` default and `Pace` make this fast.
- [ ] **Step 2 — run to verify it fails.** `go test ./host/ -run Caretaker -count=1` → FAIL (fields/methods missing).
- [ ] **Step 3 — implement.** `registry.go` default `ThinkTimeout` to `0` and treat `0` as "caretaker only on ctx cancel" to preserve current `gorged` behaviour; `play()` selects on the seat's own result versus `time.After(ThinkTimeout)`; on timeout or ctx cancel it submits the caretaker bot's intent for that player (the bot is deterministic given `MatchSeed`, so the log matches a pure-bot game; D3). Human reconnect later still works via `SubmitIntent` (D2, the slot has moved to the next decision).
- [ ] **Step 4 — run to pass**, plus the replay-parity assertion in Step 1.
- [ ] **Step 5 — gates & commit.** Into `./host/` and `./replay/`, plus the deterministic-replay link. Commit:
  `feat(host): ThinkTimeout caretaker — a human seat can never wedge the loop`.

#### Task M2b-4: per-seat view (`ViewAtSeat`) and per-seat events

**Files:**
- Modify: `host/viewat.go`, `host/fanout.go` (share a viewer param)
- Create: `host/seatscope_test.go`

**Interfaces:**
- Produces: `func (r *Registry) ViewAtSeat(id TableID, k int, seq uint64, player state.PlayerID) (view.View, error)`; `func (r *Registry) EventsSeat(id TableID, k int, since uint64, player state.PlayerID) ([]protocol.EventBody, error)`.

**Steps:**

- [ ] **Step 1 — failing test** (`host/seatscope_test.go`): run a real multi-seat game; assert (a) `ViewAtSeat(t,k,head,p)` has `PlayerView[p].Hand != nil` and every *other* seat's `Hand == nil`, `Decision == nil` for a non-asked seat; (b) `ViewAtSeat(...other seat...)` shows the *other* seat's hand and `nil` for `p`; (c) `EventsSeat(t,k,0,p)` redacts away the *other* seats' secret draws (a card in another player's hand never surfaces), and the seat's own draws pass.
- [ ] **Step 2 — run to verify it fails.**
- [ ] **Step 3 — implement.** `viewAt` already projects at line `viewat.go:249`; thread a `(viewer, vis, decision)` triple through a shared internal helper so `ViewAt` (spectator) and `ViewAtSeat` (seat) differ only in how they call `view.ProjectFor`. `Events` mirrors via `RedactEventsFor(g, evs, viewer, Seat)` instead of the table-spectator mode. No engine change.
- [ ] **Step 4 — run to pass** (`./host/`, `./view/`).
- [ ] **Step 5 — gates & commit.** Commit:
  `feat(host): ViewAtSeat/EventsSeat — per-player projection, never unprojected`.

#### Task M2b-5: library-level end-to-end human match acceptance

**Files:**
- Create: `host/human_match_test.go`

**Interfaces:**
- Consumes: everything from M2b-1..4 via `host.Options`.

**Steps:**

- [ ] **Step 1 — failing acceptance test.** Using the 12 repo decks via `internal/testutil` through `host.Options.LoadDeck`, AddTable a non-perpetual **4-seat** table: slot 0 `NewHumanSeat`, slots 1–3 bots (`defaultSeats`). Drive the human seat through a feature — say until ~20 decisions or a `KChoose` is pending — by a loop `for { d, _ := r.Pending(...); if d == nil break; r.SubmitIntent(... legal answer ...) }`, asserting the match reaches `MatchFinished` without the caretaker ever firing (the human answered everything). Then assert the whole logged game replays byte-identically (`replay.Replay` → same head), i.e. a human-driven game is indistinguishable from a recorded game.
- [ ] **Step 2 — run to verify it fails** (e.g. `Pending` naming or `AddTable` validation).
- [ ] **Step 3 — implement / fix** whatever M2b-1..4 left rough (this task is deliberately the integration seam; small fixes land here).
- [ ] **Step 4 — run to pass** and `go test ./internal/... ./host/ ./seat/ ./decision/ ./replay/ -count=1`.
- [ ] **Step 5 — gates & commit.** Commit:
  `test(host): a human seat plays a full bot-table match, replay-identical`.

### M2c — gorge's side of the mtgserve integration

#### Task M2c-1: embedder observer hooks

**Files:**
- Modify: `host/registry.go` (`Options.OnBurst`, `Options.OnMatchEnd`), `host/match.go`, `host/snapshot.go` (invoke), `host/restart.go` (invoke for archived/aborted)
- Create: `host/observe_test.go`

**Interfaces:**
- Produces:
  `type OnBurstFunc func(t TableID, k int, evs []events.Event, in *decision.Intent) error`
  `type OnMatchEndFunc func(t TableID, k int, m protocol.MatchInfo) error`
  as fields on `Options` (both nil by default → current behaviour **unchanged**).

**Steps:**

- [ ] **Step 1 — failing test** (`host/observe_test.go`): run an in-memory (`Dir=""`) bot match with an `OnBurst` sink that appends `evs`+`in`; assert every event/intent from genesis through the final `GameOver` reaches the sink in chain order, and `OnMatchEnd` fires once with the finished `MatchInfo`. A companion test with `Dir=""` still replays: feed the sink back to `replay.Replay` and assert the **same chain head** (the observer path preserves determinism).
- [ ] **Step 2 — run to verify it fails.**
- [ ] **Step 3 — implement.** `afterBurst` (`host/snapshot.go:18`) already runs after every successful burst holding `m.mu`; invoke `OnBurst` there (after the normal persist, error → `play` crashes the match like a persist failure, D15). `finish`/`crash`/`abort` invoke `OnMatchEnd`.
- [ ] **Step 4 — run to pass** (`./host/`, `./replay/`).
- [ ] **Step 5 — gates & commit.** Commit:
  `feat(host): OnBurst/OnMatchEnd embedder hooks for mtgserve SQLite persistence`.

#### Task M2c-2: `TableConfig` human seats + non-perpetual single-shot

**Files:**
- Modify: `host/table.go`, `host/match.go`, `host/restart.go`, `host/registry.go` (validation)
- Create: `host/table_plan_test.go`

**Interfaces:**
- Produces: `TableConfig.Humans []int` (`[]int` of slot indices; default nil = all bots = today), validated at `table.go:42` (in-range, no duplicates, disjoint from `bots`); the `run` loop treats a table with `Humans != nil` as single-shot (ends at game over, no `Perpetual` autoplay) and builds `Seats` from `Options.Seats` honoring the plan.

**Steps:**

- [ ] **Step 1 — failing test:** a table with `Humans: []int{0}` and one bot, started and unsettled, honours `ThinkTimeout` and ends single-shot rather than autoplaying the next match; validation rejects `Humans: []int{9}` or a duplicate.
- [ ] **Step 2 — run to verify it fails.**
- [ ] **Step 3 — implement** `TableConfig.Humans`; `newMatch`/`play` construct the seat slice accordingly (bot on non-listed slots).
- [ ] **Step 4 — run to pass** (`./host/`).
- [ ] **Step 5 — gates & commit.** Commit:
  `feat(host): TableConfig.Humans — reproducible human-driven single-shot tables`.

#### Task M2c-3: seat-scoped observer feed (o `match_events`)

This is a *gorge* test that pins the contract mtgserve's adapter will rely on; it does not write mtgserve code.

**Files:**
- Create: `host/persist_observe_test.go`

**Interfaces:**
- Consumes: `OnBurst` + `ViewAtSeat` + `EventsSeat` + `replay.Replay`.

**Steps:**

- [ ] **Step 1 — failing test:** run a human match through `OnBurst` into an in-memory sink; assert (a) the finished game's chain head derived by feeding the sink (not host files) to `replay.Replay` equals the live head; (b) for a finished archived match, `EventsSeat(t,k,0,p)` and `ViewAtSeat(t,k,head,p)` served *from the sink* (`readLog`-equivalent) equal the same seat's live projection. This is the exact guarantee mtgserve's `match_events` persistence must honour.
- [ ] **Step 2 — run to verify it fails** (no `EventsSeat` from a non-file source / no lore for it).
- [ ] **Step 3 — implement/fix** the read-side: extend `EventsSeat`/`ViewAtSeat` (or a readLog helper) so a persisted observer log is served identically to a live one (reuse `viewAt`'s replay-from-log path, `viewat.go:163-227`).
- [ ] **Step 4 — run to pass** (`./host/`).
- [ ] **Step 5 — gates & commit.** Commit:
  `test(host): observer-persisted matches serve and replay identically to live ones`.

#### Task M2c-4: the god-view structural guarantee test

**Files:**
- Modify: `internal/archtest/arch_test.go` (add `TestNoExportLeaksAnEngineGame`)
- Create: `host/leak_test.go`, `view/projection_closure_test.go`

**Interfaces:**
- Produces: the enforcement behind D6.

**Steps:**

- [ ] **Step 1 — failing test** (`internal/archtest/arch_test.go`):
  ```go
  // TestNoExportLeaksAnEngineGame is D6's compile-time half: no exported
  // symbol of host or host/httpapi may expose a *state.Game or a
  // *rules.Engine — through its return type or by a result that a caller
  // could type-assert into one. Every engine-facing value must already be a
  // view.View or a protocol.* payload.
  func TestNoExportLeaksAnEngineGame(t *testing.T) {
      // go doc -all <pkg> → for each exported func/method, fail if the
      // signature's result (or any documented return) contains
      // "*rules.Engine" or "*state.Game".
      for _, pkg := range []string{module + "/host", module + "/host/httpapi"} {
          out, err := exec.Command("go", "doc", "-all", pkg).CombinedOutput()
          if err != nil { t.Fatalf("go doc: %v", err) }
          for _, m := range []string{"*rules.Engine", "*state.Game", "state.Game", "rules.Engine"} {
              if strings.Contains(string(out), m) {
                  t.Errorf("%s documents a return/parameter that can leak an engine game: %q", pkg, m)
              }
          }
      }
  }
  ```
  (This is a *grepping* archtest as the package already is for imports; a `go doc`
  scan is fine for a planning gate.) The same test's runtime half lives in
  `host/leak_test.go`: call every exported `*Registry` method that returns data
  and assert each is a `view.View`, `[]protocol.EventBody`, or `protocol.*`.
- [ ] **Step 2 — run to verify it (currently) fails or passes as a baseline**, then in `view/projection_closure_test.go` add:
  ```go
  // TestViewMarshalsClosed: the Omniscient-view JSON carries hands and pools
  // but never a raw library order field. Asserts the View type's public JSON
  // keys are exactly {viewer, visibility, turn, step, phase, active, priority,
  // over, draw, winner, players, stack, pending, decision}; a new key that
  // could carry hidden zones fails review.
  ```
- [ ] **Step 3 — make it pass** (no production change should be needed; if the archtest trips on a new method *you* added in M2b/M2c, that is the point — fix the shape, don't weaken the test).
- [ ] **Step 4 — run the whole suite** `go test -count=1 ./host/ ./view/ ./internal/archtest/` plus the M2a gates (`make sim` unchanged, `make report` unchanged).
- [ ] **Step 5 — gates & commit.** Commit:
  `test: god-view leak is impossible by construction — no export leaks an engine game`.

---

## 6. Dependency order and parallelism

All tasks are **sequential** (one implementer at a time within this worktree)
because M2b-2/3/4 and M2c-2/3 touch overlapping `host/match.go`/`host/registry
.go`/`host/table.go` surface. Concurrency is safe only at the tear line between
the **library-feature tasks** and the **pure-test tasks**:

| Can run in parallel | Why / caveat |
|---|---|
| M2b-1 (`host/humanseat.go`) — alone in its own files | New file, only touches `host/humanseat.go`; nothing else edits `seat` or `host`'s files. |
| M2c-4 (`internal/archtest/arch_test.go`, `host/leak_test.go`, `view/projection_closure_test.go`) — in parallel with M2b-1 | Pure test + one archtest file; touches no shared source. But it *must* land **after** M2b-3/M2b-4 to be meaningful (it guards those new methods). **Recommendation:** write it after M2b-4. |
| M2c-1 (`host/observe.go`/`snapshot.go` OnBurst) vs M2b-2 (`host/match.go` seats-on-the-match) | Both touch `host/match.go` and `host/snapshot.go` — **collide**, do not parallelize. |
| M2b-3 vs M2b-2 | Both touch `host/match.go` `play()` — **collide**, sequential. |
| M2c-2 vs M2b-3 | Both touch `host/registry.go`/`host/match.go` — **collide**, sequential. |
| M2b-4 vs any M2b task | All touch `host/viewat.go`/`host/fanout.go` — **collide**, sequential. |

**Concrete collision map** (the orchestrator's dispatch source):

- `host/humanseat.go` — only M2b-1.
- `host/humanseat_test.go` — only M2b-1.
- `host/match.go` — M2b-2, M2b-3, M2c-1, M2c-2 (four tasks; strictly serialize).
- `host/registry.go` (Options) — M2b-3, M2c-1, M2c-2.
- `host/viewat.go` / `host/fanout.go` — M2b-4 (and M2c-3 reads it; do M2c-3 after M2b-4).
- `host/table.go` — M2c-2.
- `host/snapshot.go` — M2c-1 (invokes OnBurst).
- `internal/archtest/arch_test.go`, `host/leak_test.go`, `view/projection_closure_test.go` — M2c-4 (after M2b-4).
- `host/action.go`, `host/caretaker_test.go`, `host/human_match_test.go`, `host/seatscope_test.go`, `host/persist_observe_test.go`, `host/observe_test.go` — one task each.

**Recommended dispatch:** M2b-1 → M2b-2 → M2b-3 → M2b-4 → M2b-5 → M2c-1 → M2c-2 → M2c-3 → M2c-4. A second worktree may start **only** M2c-4's archtest scaffolding against the merge base after M2b-4 is merged, and must not touch the shared feature files.

---

## 7. Outside gorge — mtgserve requirements (NOT gorge tasks)

The work below is on `mtgbld/mtgserve` (a different repo, mounted read-only to
this planner). It is specified here as requirements, not numbered gorge tasks,
and the invariants above do not bind it.

Permitted **only**: rewrite mtgserve's engine binding and its god-view paths so
every outgoing payload is a `view.Project` for the authenticated seat.

1. **Replace the mtgplay bridge with an embedded `*host.Registry`.**
   Add a thin adapter (e.g. `internal/matches/adapter.go`) that maps a
   `matches.Match` row + its `match_seats` (`matches/store.go:43-60`) onto
   `host.TableConfig` (seats from the pairing, decks via `deckPayloadFor`,
   `handlers.go:35`, seeds per open question 1) and mirrors the observer feed
   (`OnBurst`) into `matches.AppendEvent` (`events.go:45`). Wire the stock
   lifecycle (`Create`/`Join` + a new `SetState(active)` on first host start) to
   `host.AddTable`/`Start`/`Done`, and `SetState(completed)` from `OnMatchEnd`.
2. **Retire the god-view snapshot path.** Stop calling `AppendSnapshot` /
   `ListSnapshots` (`snapshots.go:45/70`) and delete or gate off
   `ListSnapshotsPublic` (`handlers.go:695-742`), which today serves
   `revealHand=true` god views to any seat-holder. Replay/history must come from
   per-seat projections (`host.ViewAtSeat`/`EventsSeat` are the only exits).
   Keep legacy `match_snapshots` rows as read-only history or migrate them (open
   question 2).
3. **Serve the seat transport (mtgplay's WS role).** mtgserve must serve a
   WebSocket that (a) pushes the currently-pending decision for the authed seat
   (full `Options`, so the client needs no legality — `decision.Option` already
   carries `Index/Kind/Label/Obj/Player` on the wire), (b) receives an
   `Intent{Seq, Player, Choices}` and calls `host.SubmitIntent`, and (c) applies
   its session/auth so a request can only drive the seat it authenticated as
   (`auth.CurrentUserID`, `internal/auth/session.go:39`). The client retry / Seq
   semantics are mtgserve's (open question 4).
4. **Gate non-Commander formats.** mtgserve must not launch gorge matches with
   Commander decks until gorge M2d (open question 6).
5. **Persist a per-match seed** so mtgserve's stored log is replayable
   (`replay.Replay` / `host.ViewAt`) — see open question 1.
6. **A mtgserve-side leak test.** The adapter may expose only seat-scoped
   methods and must never reach into the `*rules.Engine` a gorge test would
   defeat-only-for-instrumentation via `replay.Replay`. This is mtgserve's
   analogue of Task M2c-4: no HTTP handler may marshal an unprojected game.

---

## 8. Done-when (this plan)

M2b (engine): `go test -count=1 ./host/ ./seat/ ./view/ ./decision/ ./replay/`
passes including M2b-5's human-vs-bot acceptance and M2c-3's observer-replay
parity; `make sim` stays 20/20 and `make report` unchanged; `internal/archtest`
passes including the new god-view guard. The browser client is out of scope, so
there is **no** Playwright gate in this plan.

M2c (gorge side): the embedder surface above is complete and leak-free; the
mtgserve migration is handed off via §7 and the open questions below.

---

## 9. Open questions (repeated, unabbreviated)

1. Per-match seed source/persistence in mtgserve (`store.go` has no seed column).
2. Legacy `match_snapshots` migration vs. freeze-and-read-only.
3. Per-`decision.Kind` think-timeout budgets (a single `Options.ThinkTimeout` is planned).
4. Reconnect/`Seq` semantics for a rejoining player past the caretaker turn.
5. Multi-human table policy (planned single-human-first).
6. Commander gating at the adapter (plan gates to non-Commander; deck mapping for M2d unresolved).
