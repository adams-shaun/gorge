# gorge M2d — Decision-Kind Completeness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the three decision-kind gaps the M2d design (§3) books for the engine — the London mulligan (`KMulligan`), modes / modal spells and Charm (`KModes`, which also delivers the **mid-resolution ask** that closes M2r's R-8 on `UnlessCost$`), and concede — so that a client that reaches the wire can actually mulligan, cast a modal spell, and concede. The investigation of all three is `decision/decision.go`'s two declared-but-never-constructed kinds plus audit item 12.

**Architecture:** `rules.New` keeps dealing seven, but between the opening deal and the first turn it runs a **London mulligan round** whenever `Config.Mulligans > 0`, asking each seat a `KMulligan` keep/mulligan decision and then, for anyone who mulliganed, a `Min==Max==taken` bottoming decision — the distinct-index shape `Validate` already enforces (Ruling U2). `KModes` turns `effects.Charm`'s today-always-first-mode approximation and modal spells into a real mode-pick, on top of a small **mid-resolution ask** machinery: an effect can suspend the top-of-stack resolution, ask a decision, and resume from the recorded answer. That machinery is what M2r's R-8 ruling calls "one `if` once mid-resolution asks exist": the `if` goes in `effects/copy.go`'s `effCopySpellAbility`, where `UnlessCost$` is currently declined deterministically. Concede is a `"concede"` option on the priority decision that emits the *existing* `PlayerLost` event (Text "conceded") — a concession is a way to lose (CR 104.3a), `PlayerLost` already marks the seat `Lost` and reaches the event log, and `checkGameOver` already ends the game with the last remaining seat as winner.

**Tech Stack:** Go 1.26 stdlib only. `CGO_ENABLED=0`. No new dependencies, no new wire format: `Decision`/`Option`/`Intent` are unchanged; this plan adds one field to `rules.Config` (`Mulligans int`), one new event kind (`events.ModeChosen`), two engine value fields (`pregame`, `mulligan`), one engine pointer field (`resume`), and one appended-tagged field to `state.Player`/nothing — the mulligan and bottoms need no new object state (the kept hand holds N cards, the bottomed N are a library bottom move).

**Spec:** `docs/superpowers/specs/2026-09-05-gorge-m2d-fixture-design.md` §3 (the three sub-tasks), §5 (sequencing), §7 (testing), §8 (the four open questions). The engine spec `docs/superpowers/specs/2026-09-03-mtgcore-go-engine-design.md` still binds everything not amended. The four §8 questions that were left open are settled by the controller in the Rulings below and are **not reopened**.

**Sequencing note (from design §5, non-negotiable):** M2d touches `rules/` core — `cast.go`, `engine.go`, `turn.go`, `stack.go`. It **cannot overlap M2r's in-flight rules work** (M2r Task 15, protection, which touches `rules/stack.go`, `rules/combat.go`, `rules/engine.go`). M2d starts only after R15 merges. And **M2r Task 21 (the M2r end-state docs and final numbers) lands after M2d** — R21's final numbers are stale the moment M2d regenerates the goldens. Within M2d the three sub-tasks **collide file-for-file and form one slot, not three parallel agents** — see the collision map below.

---

## Global Constraints

Every task's requirements implicitly include this section.

- **Stdlib only; no cgo.** `go.mod` gains no `require`.
- **Every mutation goes through `events.Apply`.** `events.Kind` is append-only; **never add, remove or reorder a field of `events.Event`** (its binary encoding is hash-chained and replayed). This plan appends exactly one kind (`ModeChosen`); nothing else. `effects` never imports `rules`; a mid-resolution ask is reached from `rules` through the `effects.Host` interface (type-asserted to `*rules.Engine` inside `rules`), never by exporting engine internals downward.
- **Dependency order** `cards → state → decision → events → effects → rules → view → seat → replay → …`. Every pregame shuffle, re-draw and bottom is an `e.emit`/`events.Emit`, so the whole mulligan round replays. `replay.Replay(l, cfg)` re-runs `rules.New(cfg)` then re-submits the recorded intents; `Config.Mulligans` therefore **must** travel in the same `Config` value replay is handed (R-8.4 below) — the moment it lives in a table setting instead, a replay cannot reproduce the mulligan round and the determinism invariant breaks.
- **Determinism.** No `time`, no global `math/rand`, no map-range order reaching an event, option order, view, or file. The mulligan round iterates `AliveFrom(0)` (a slice, deterministic order) and never a map. Every option list walks a zone slice or the log. The bots convert each payment/mode decision with their own seeded `rand.Rand` (like `KTriggerOptional`), so the resulting games are seed-deterministic. All `Engine` resume fields are value/pointer data cloned like `cast`/`choosing`/`drainAwaitsTarget`, never closures.
- **One goroutine per match; totality.** Any reachable panic or stall is a bug. Every new decision kind and every new mid-resolution path is covered by `rules/fuzz_test.go`'s invariant run (it must keep terminating within its 400 000-intent budget) and by `internal/testutil.CheckInvariants`. A mulligan round on a game already `Over` (decked out during the deal) never runs — `New` has a `if e.G.Over { return e }` guard before `beginTurn`; M2d-1 threads the pregame branch through that same guard.
- **Licensing.** The Forge corpus (GPL-3.0) lives only in gitignored `.cards/`; never commit a Forge `.txt`. `cards/boundary_test.go` stays. Before a push, `git ls-files | grep -c '\.txt$'` prints `0`.
- **Chain heads are goldens — but M2d owns exactly ONE regeneration.** `rules/heads_test.go`'s `acceptanceHeads` pins the four acceptance heads. The design's golden cost (§3) is that a mulligan changes **every** opening hand, so all four heads move — 2/4/6/8, not one or two. **No individual M2d task regenerates `rules/heads_test.go`.** Each task measures which seat counts moved and names the card behaviour that moved them (per FL-76, build-bisect main-before-vs-main-after, never a feature flag), records it in the commit body, and leaves `acceptanceHeads` stale. The single regeneration happens at the merge gate (see "M2d merge gate" below), recomputed from the merged tree. Because M2d is a single milestone with a known cause, the bisect is main-before vs main-after — there is nothing to toggle.
- **The ratchet can move in EITHER direction** (design §3, point 2, controller-settled). A card counted as supported may stop being drawn in the new opening; a card never reached may now be cast. `make report`'s `playable` count and `rules/acceptance_test.go`'s `knownUnsupported` are a *measurement of what the repo decks actually do*, and M2d changes what they do. It is expected to be `0 of 136` once M2r Task 15 merges, which is the state M2d starts from (confirmed on its branch, not yet merged). **The plan does not hard-code a number it cannot know; the ratchet is recomputed from the merged M2d tree at the merge gate.** A task that sees the count move reports it; it does not "fix" it back.
- **Gates every task runs before its commit** (repo discipline — never `go test ./...`, never `-race` on an ordinary run, never `-count=1` on an ordinary gate run; the Go test cache auto-invalidates on a package edit, so a changed `rules/` recompiles and re-runs):

```sh
gofmt -l .                                  # empty
go vet ./rules/ ./decision/ ./seat/ ./events/ ./state/   # as each task's touched packages dictate
go test ./rules/ ./decision/ ./seat/ ./events/ ./state/   # the touched packages + dependents the task names; never ./...
go build ./...
make report | grep '^cards:'                # record the playable count
make sim | grep -c 'replay OK'              # 20
git ls-files | grep -c '\.txt$'             # 0
```

  The race gate is FL-67: it is not on the push path — it stays opt-in in `.githooks/pre-push`, and the controller runs `go test -race ./<package>/` by hand, targeted at a diff that changed lock discipline or concurrency. M2d is engine-internal single-goroutine work, so it likely does not qualify; a task does not run `-race`. `go vet ./...` is the eventual merge gate; a task vets only the packages it touches.
- **Approximations are documented, never silent.** Where M2d leaves a stand-in in place (concede at priority only; `UnlessSwitched$` on Chain Lightning's copy handled as a documented first-pass), the primitive's doc comment and `AGENTS.md`'s "Known approximations" list say so, with the milestone that removes it. R-8's decline is **closed** by M2d-2; every other R-8/R-9-adjacent stand-in keeps its listing in `AGENTS.md`.
- **Isolation.** M2d runs in a worktree on its own branch (per `wt/` convention) based on `main` *after* R15 merges. One slot, sequential — never three worktrees editing `rules/` at once.
- **Git.** No bare `git stash`. No commit carries a `Co-Authored-By` or any AI-attribution trailer (the pre-push hook refuses them).
- **Comment discipline.** Doc comments say what and why; they do not narrate ledger history. A ruling ID is cited only where the code would otherwise be surprising. The two dangling deferral references the design §2 audit found (see the M2d-2 *dangling references* note) are corrected *in the task that does the work*, not left pointing at work that does not exist.

## Plan-level rulings (controller-settled; executors treat them as spec — these close design §8)

- **R-M1 The mulligan is NOT configurable off for the acceptance decks** (closes §8.2, adopting the design's own recommendation verbatim). A mulligan the acceptance suite never exercises is a mulligan nobody tests. `playAcceptance` (`rules/acceptance_test.go`) sets `Config.Mulligans = 1` so keep/mulligan *and* bottoming always run in the 12-deck acceptance games, which is why M2d pays the golden regeneration immediately. Standalone fixture tests that do not set it get `Mulligans == 0` and skip the round — today's behaviour, byte-identical.
- **R-8.4 The free-mulligan count is a `rules.Config` field** (closes §8.4), named `Config.Mulligans int` (doc: "the number of London mulligans each player may take in the pre-game round between the opening deal and turn 1; 0 skips the round entirely, so every Config that never sets it is unchanged — the acceptance config sets it so the suite exercises the round, and it travels in the same Config `replay.Replay(log, cfg)` is handed, which is what lets a replay reproduce the round"). **Not** a table/host setting: `replay.Replay(log, cfg)` re-runs `rules.New(cfg)`, so anything that affects the game must live in that `Config` — a table setting would not be visible on the rules side and a replay could not reproduce the round.
- **R-M3 Concede's shape — least new surface, one hard constraint met** (closes §8.1). **Chosen:** a `"concede"` option on every priority decision, served last in the option list, handled by `handlePriority`. Choosing it emits the **existing** `events.PlayerLost` event with `Text: "conceded"`. That single event (i) marks the seat `Lost` via `PlayerLost`'s existing `Apply` case, (ii) reaches the event log, so a conceded game replays to the same chain head it was played to (the `Submit`-side `DecisionMade` intent + the `PlayerLost` event are both recorded and re-derived), and (iii) reuses `checkGameOver`, which already ends the game with the last remaining seat the winner. **Zero new event kinds, zero new engine mutation paths.** **Rejected alternative:** a standalone `Registry.Concede(tableID, seat)` method that forces the loss outside the `Pending`/`SubmitIntent` decision path. Rejected because (a) it would create a *second* mutation driver that does not go through `decision.Validate`/`Submit` — every other mutation that resolves a player choice travels through the recorded-intent path, and the determinism invariants are stated in terms of that one path; (b) it adds host plumbing (a `Registry` method and an HTTP route) for a case the priority option already serves with strictly less surface; and (c) a concede that "reaches the event log" is already guaranteed by emitting `PlayerLost` from the decision handler, so the method buys nothing. **Also consciously out of scope:** conceding while another player holds a decision (mid-`KTarget`/`KBlockers`, no granted priority). Real Magic allows concession any time, but the fixture's least-surface faithful subset is *at the conceding player's own priority*, which the engine already offers every turn; a human client reaches it at most one decision later. The all-bot acceptance games never pick the option, so concede moves no golden (options are not events).
- **R-M2 M2d-2 is the mid-resolution ask, and it closes R-8.** The ruling M2r Task 17 wrote at `effects/copy.go:34-39` described `UnlessCost$` as "one `if` once mid-resolution asks exist." M2d-2 delivers those asks. **Where the `if` goes:** `effects/copy.go`'s `effCopySpellAbility`, replacing the block `if _, hasUnless := sa.Params["UnlessCost"]; hasUnless { … Note …; return }` at lines 34–39. **What suspends the resolution:** the new mid-resolution ask mechanism — `resolveTop` (`rules/stack.go`) runs the top object's effects and, when an effect asks a decision mid-resolution, leaves the object on the stack pending, sets an engine resume pointer, and returns to the loop; the decision answer re-enters the suspended effect's continuation with the recorded answer. The `if` becomes: on the recorded answer, either the payer pays `UnlessCost` and the copy proceeds (honouring `UnlessSwitched$` as a documented first-pass), or no copy. The cost really is one `if` — the machinery is the deliverable, this ruling's point is to say where the `if` physically sits.
- **R-8.3 `StackView.Text`'s "when known" hedge is out of scope for M2d** (registering §8.3). The design says measure before deciding; M2d does not plan work for it. Stated as a non-goal so the next reader knows it was considered; it is audit item none, and it belongs to a later "measure StackView.Text" task.
- **R-M4 Free-mulligan reading (CORRECTED 2026-09-05, see FL-90).** `Config.Mulligans = N` means each player may take up to N London mulligans. A mulligan **always redraws a full `openingHand` of seven** — it does *not* shrink the draw. The entire penalty is the end-of-round bottoming: a seat that has taken k mulligans puts exactly k cards on the bottom of its library, so its final hand is **7 − k** (CR 103.4). `Mulligans = 1` therefore exercises one mulligan and a bottom of one, ending at six cards — exactly the acceptance suite's need.

  The original wording of this ruling said the draw itself forfeits a card *and* that the seat then bottoms `taken`, which double-penalizes: it stacks London's bottoming on top of the old Vancouver mulligan's shrinking draw, leaving 7 − 2k. M2d-1 implemented the ruling faithfully and shipped a fully green suite whose own tests asserted the wrong hand size. This was a defect in the plan, caught at the M2d-1 merge gate.

## File structure

| Path | Responsibility | Task |
|---|---|---|
| `rules/config.go` (new) or `rules/engine.go` (`Config` + `New` pregame branch) | `Config.Mulligans`; the pregame hand-off | 1 |
| `rules/mulligan.go` (new), `rules/mulligan_test.go` (new) | the round state machine, `handleMulligan` | 1 |
| `rules/engine.go` (pregame/mulligan fields), `rules/turn.go` (step dispatch), `decision/decision.go` (KMulligan doc), `seat/bot.go` + `rules/testbot_test.go` (real KMulligan policy), `rules/acceptance_test.go` (`Config.Mulligans = 1`) | M2d-1 | 1 |
| `rules/stack.go` (`resolveTop` suspension, resume), `rules/engine.go` (`resume`), `effects/registry.go` (`Host.Ask`), `effects/misc.go` (`effCharm` asks), `effects/copy.go` (R-8 `if`), `events/event.go`+`events/apply.go` (`ModeChosen`), `rules/turn.go` (`handleModes`/resume), `decision/decision.go` (KModes doc), `seat/bot.go` + `rules/testbot_test.go` (KModes policy) | mid-resolution ask + KModes + R-8 | 2 |
| `rules/turn.go` (`handlePriority` "concede"), `rules/engine.go` (nothing new beyond dispatch), `events/apply.go` (no change — `PlayerLost` exists), `seat/bot.go` + `rules/testbot_test.go` (ignore the option), `rules/sba_test.go` (concede path) | M2d-3 | 3 |
| `rules/heads_test.go`, `rules/acceptance_test.go` (ratchet recomputed) | **One** golden regeneration, the controller at the merge gate | gate |

## Collision map — why M2d-1/-2/-3 are one slot

State whether they can run in parallel, with a file-level collision map (the same shape as design §5). The three sub-tasks touch these files:

| file | M2d-1 | M2d-2 | M2d-3 |
|---|---|---|---|
| `rules/engine.go` | Config/New/pregame fields | `resume` machinery | (dispatch) |
| `rules/turn.go` | `step` pregame dispatch | `handleModes`/resume | `handlePriority` "concede" |
| `rules/stack.go` | — | `resolveTop` suspension | — |
| `decision/decision.go` | KMulligan doc | KModes doc | — |
| `seat/bot.go` + `rules/testbot_test.go` | KMulligan case | KModes case | ignore-option note |
| `rules/acceptance_test.go` | `Mulligans=1` | — | (none — no table change) |
| `rules/heads_test.go` | (regenerated only at merge) | (same) | (same) |
| `effects/misc.go`, `effects/copy.go`, `effects/registry.go`, `events/event.go`, `events/apply.go` | — | yes | — |

- **M2d-1 ∩ M2d-2**: `rules/engine.go`, `rules/turn.go`, `seat/bot.go`, `rules/testbot_test.go`. The mulligan sets `pregame` state and the modes machinery sets `resume` — both live on the same `Engine` struct and both add cases to the same `botDecide`/`answer` switch. Cannot run together.
- **M2d-1 ∩ M2d-3**: `rules/engine.go`, `rules/turn.go`, plus both change what the acceptance games do (mulligan does; concede does not change theirs but leaves the option in every priority decision the same two files build).
- **M2d-2 ∩ M2d-3**: `rules/turn.go` (the same `handle` dispatch), `rules/engine.go`.
- All three share `seat/bot.go`'s one `switch d.Kind` and its `rules/testbot_test.go` mirror — the two implement the same `switch d.Kind` and must be changed together.

**Conclusion: the three collide on `rules/engine.go`, `rules/turn.go`, `seat/bot.go`, `rules/testbot_test.go`, and ultimately on the single golden regeneration. They form ONE slot, run sequentially as M2d-1 → M2d-2 → M2d-3.** The plan does not imply three agents can run at once. Order: mulligan first (it changes every opening hand, so any later task's hand state moves), then the mid-resolution machinery + modes (the largest, and it owns the R-8 closure), then concede (smallest, builds on the decision path). M2e (the fixture) cannot start until M2d lands.

---

## M2d-1: the London mulligan (`KMulligan`)

**Files:**
- Modify: `rules/engine.go` (`Config.Mulligans`, `New` pregame branch, `pregame`+`mulligan` fields), `rules/turn.go` (`step` pregame dispatch), `decision/decision.go` (doc comment for the already-declared `KMulligan`), `seat/bot.go` (a real `KMulligan` case) and `rules/testbot_test.go` (the mirror), `rules/acceptance_test.go` (`Config.Mulligans = 1` in `playAcceptance`), `rules/genesis_replay_test.go` (the decked-out-at-genesis guard still holds; extend one assertion), `rules/heads_test.go` (NO edit here — see the merge gate)
- Create: `rules/mulligan.go`, `rules/mulligan_test.go`

**Interfaces:**
- Produces:

```go
// in rules.Config:
// Mulligans is the number of London mulligans each player may take in the
// pre-game round between the opening deal and turn 1. 0 (the zero value)
// skips the round entirely, so every Config that never sets it is unchanged
// (all standalone fixture Configs). It sits in the same Config replay is
// handed, so a replay reproduces the round (design R-8.4): the acceptance
// config sets it so the 12-deck suite exercises keep/mulligan and bottoming.
Mulligans int

// in rules/engine.go (Engine):
pregame  bool          // true while the mulligan round runs, between deal and turn 1
mulligan mulliganRound // the round's plain-value state (clone-safe, like cast/choosing)

// in rules/mulligan.go:
// mulliganRound is plain data, never a closure, so Clone copies it (the same
// discipline as cast/choosing in engine.go).
type mulliganRound struct {
	seats  []state.PlayerID // AliveFrom(0) at round start (deterministic order)
	kept   []bool           // per seat: decided to keep
	taken  []int            // per seat: mulligans taken
	bottom bool             // false = keep/mulligan phase, true = bottoming phase
	cursor int              // next seat to ask
}
func (e *Engine) stepPregame()          // issues the single next pregame decision, or hands to beginTurn
func (e *Engine) handleMulligan(d *decision.Decision, in decision.Intent)
func (e *Engine) mulliganTaken(p state.PlayerID) []state.ObjID // kept-hand order for the bottom decision
```

- London round (deterministic, sequential APNAP like the engine's other ordering): each not-yet-kept seat, in `AliveFrom(0)` order, gets a `KMulligan` decision. With `taken < Mulligans` it offers two options (`Kind "keep"`, `Kind "mulligan"`, `Min==Max==1`). On **keep** the seat is marked kept and the cursor advances. On **mulligan** the seat's hand shuffles back to the library (`Shuffle`, `Secret: true`), `taken++`, and it draws a full `openingHand` (`drawCard`, whose events replay exactly like genesis — this is literally the same loop genesis runs; the mulligan penalty is the bottoming, never a smaller draw, R-M4). Once `taken == Mulligans` a seat is offered **only `keep`** (London: after the allowed mulligans you keep what you have). When every alive seat has kept, the round moves to **bottoming**: each seat with `taken > 0` is asked a `KMulligan` decision with `Min==Max==taken` over its kept hand, **one `Kind "bottom"` option per card** — the exact `Validate` distinct-index shape (Ruling U2; "N distinct in-range indices"). A `MoveZone` of each chosen card to the bottom of its owner's library replays. When bottoming completes, `e.pregame = false` and `e.beginTurn(alive[0])` runs exactly as today.
- No new wire format: the wire `Decision`/`Option`/`Intent` shape is unchanged; both mulligan phases ride on the already-declared `KMulligan` kind, distinguished by option vocabulary (`keep`/`mulligan` vs `bottom`) and by `Min`/`Max`. (The KTriggerOrder precedent already demonstrated `Validate`'s distinct-index rule is enough; a bottoming choice is a permutation of `taken` indices.)
- Bot policy (both bots, verbatim mirror): keep/mulligan form — if a second option (`Kind "mulligan"`) is offered, take it with the bot's own `rand.Rand` at probability 1/3 (mirrors how `KTriggerOptional` consumes the bot rng, so reproducibility holds per seed); otherwise keep. Bottoming form — choose the `taken` options with the lowest `Index` (the bot bottoms its oldest cards): `Choices = [0, 1, …, taken-1]`, placed on the bottom in ascending index order. Both branches consume the bot's seeded rng only where a real choice exists, exactly like `KTriggerOptional`/'`exile`'/'`sacrifice`' in `KChoose`.

- [ ] **Step 1: Write the failing tests**

`rules/mulligan_test.go` — a two-seat fixture is not the right vehicle (it sets its own hand directly and, with `Mulligans == 0`, never enters the round), so drive `New` with a real two-deck Config and read `e.Pending()` until `e.G.Turn >= 1`:

```go
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
)

// playRound drives a New-built engine through its pre-game: the bot answers
// every KMulligan until the round ends (Turn >= 1) and returns the engine.
func playRound(t *testing.T, cfg Config) *Engine {
	t.Helper()
	e := New(cfg)
	b := newTestBot(9)
	e.Advance()
	for e.G.Turn == 0 && e.Pending() != nil && !e.G.Over {
		if err := e.Submit(b.answer(false, e.Pending())); err != nil {
			t.Fatalf("pregame intent: %v", err)
		}
	}
	return e
}

func TestMulliganWithKeepsGoesStraightToTurnOne(t *testing.T) {
	// Both bots keep (see bot policy below): no reshuffle, no bottom, and
	// the round emits only the keep/mulligan asks.
	e := playRound(t, twoSeatConfig(t, 41, 1)) // helper: 2 seats, one-deck-each, Mulligans 1
	if e.G.Turn != 1 {
		t.Fatalf("round did not reach turn 1, turn %d", e.G.Turn)
	}
	// Every alive seat still holds a full hand (kept 7).
	for _, p := range e.G.AliveFrom(0) {
		if got := len(e.G.Zone(state.ZHand, p)); got != 7 {
			t.Errorf("seat %d hand %d, want 7", p, got)
		}
	}
}

func TestABotThatMulligansRedrawsSevenAndBottomsOne(t *testing.T) {
	// Drive the round with a stub answerer that mulligans the first seat
	// once then keeps, and bottoms the lowest-index card. Assert the seat
	// holds 6, its library gained exactly the bottomed card at its bottom,
	// and the PlayerLost-less log has no zero-turn TurnChange yet.
	// (Concrete: submit Intents in order; verify a Shuffle event for seat 0
	// and a hand of 6, then a bottom MoveZone to library index len-1.)
}

func TestMulliganBottomIsDistinctIndices(t *testing.T) {
	// A bottoming decision (taken=2) must be rejected by Validate if the
	// intent repeats an index. Reuse decision.Validate directly.
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KMulligan, Min: 2, Max: 2,
		Options: []decision.Option{{Index: 0, Kind: "bottom"}, {Index: 1, Kind: "bottom"}, {Index: 2, Kind: "bottom"}}}
	if d.Validate(decision.Intent{Seq: 1, Player: 0, Choices: []int{0, 0}}) == nil {
		t.Fatal("duplicate bottom index accepted")
	}
	if d.Validate(decision.Intent{Seq: 1, Player: 0, Choices: []int{0, 1}}) != nil {
		t.Fatal("valid distinct bottom rejected")
	}
}

func TestKeptPregameRoundReplaysExactly(t *testing.T) {
	// playAcceptance already replays each game; the focused proof is a
	// New-driven game whose round mixed a keep and a mulligan replayed from
	// its own (Config, Log) gives the same chain head.
	e := playRound(t, twoSeatConfig(t, 42, 1))
	re, err := replayFor(e.G, e.L) // or accept a local replay helper
	if err != nil {
		t.Fatal(err)
	}
	if re.L.Head() != e.L.Head() {
		t.Fatalf("replay head %s != live %s", re.L.Head(), e.L.Head())
	}
}
```

`rules/acceptance_test.go` `playAcceptance`: set `cfg.Mulligans = 1` (the fixture towers — this is the R-M1 "not configurable off" commitment, and it is what moves every golden). The genesis decked-out guard `if e.G.Over { return e }` in `New` stays and must precede any pregame hand-off; extend `genesis_replay_test.go` with one assert that a decked-out-at-genesis game (`Mulligans` whatever) never starts the round. Run: `go test ./rules/ -run 'Mulligan|Pregame|Genesis'` — FAIL (KMulligan falls to the bot last resort today, and `New` has no `pregame` branch, so the round never runs or the hand never shrinks).

- [ ] **Step 2: Implement**

`rules/engine.go`: add `Mulligans int` to `Config`; add `pregame bool` and `mulligan mulliganRound` to `Engine` (doc-commented; Clone copies them like every other value field). `New`, in place of the terminal `e.beginTurn(alive[0])`:

```go
	if !e.G.Over {
		if cfg.Mulligans > 0 {
			// design R-8.4: the round lives between the deal and turn 1.
			e.pregame = true
			e.mulligan = mulliganRound{seats: alive, kept: make([]bool, len(alive)),
				taken: make([]int, len(alive))}
		} else {
			e.beginTurn(alive[0])
		}
	}
```

`rules/turn.go` `step()`: at its top, after `checkStateBased`, `if e.pregame { e.stepPregame(); return }` (and the pregame branch must not leaf into the ordinary `switch e.G.Step`, whose steps assume a live turn). `rules/mulligan.go`: `stepPregame` walks `mulligan.seats` from `cursor`, skips kept seats, and issues the single next decision via `e.ask(...)` — keep/mulligan (`Min==Max==1`, options `/keep`,`/mulligan`) while `taken < config.Mulligans`, else keep-only — or, once every seat is kept and we are not in `bottom`, transitions to the bottom phase and issues the first bottoming decision; when bottoming is done it clears `pregame` and calls `e.beginTurn(alive[0])`. Each `KMulligan` keep/mulligan answer routes to `handleMulligan` (keep→mark kept; mulligan→`Shuffle`, `taken++`, `drawCard` for the smaller hand then `e.Advance()`); each bottoming answer moves each chosen index's card to its library bottom (`MoveZone`, `Text: "bottomed"`) then `e.Advance()`. Every mutation goes through `e.emit`/`drawCard` — no direct field write.

`rule/handle` (`rules/turn.go` or the decision `handle` switch): `case decision.KMulligan: e.handleMulligan(d, in)`. `decision/decision.go`: give the declared `KMulligan` its two-phase doc comment (keep/mulligan at `Min==Max==1`; bottoming at `Min==Max==taken` distinct indices, the KTriggerOrder-shape Validate already enforces). `seat/bot.go` `botDecide` and `rules/testbot_test.go` `answer` gain the verbatim `case decision.KMulligan:` policy above. Update the shared doc comment that today names KMulligan/KModes as "last resort" kinds now that real cases exist (leave KModes in the last resort until M2d-2). `rules/acceptance_test.go`: `cfg.Mulligans = 1` in the one `Config` literal in `playAcceptance` (and in `TestRepoDeckGamesReplayExactly` if it builds its own), with the R-M1 comment.

- [ ] **Step 3: Measure the head movement, record it, commit**

Run the changed packages green, then: `go test ./rules/ -run 'TestRepoDecksPlayAtEverySeatCount|TestHeads' -v` (note: the test cache invalidates automatically because `rules/` changed). Expect `TestHeads` to FAIL for **all four** seat counts (design §3: the opening hand feeds every subsequent decision). Per FL-76, build-bisect main-before (`c15a15f` or wherever R15 lands) vs main-after the M2d-1 commit — the only engine change is the mulligan round, so the divergence is the first reshuffle/hand-shrink event. Record, per seat count, the first divergent event and which deck's cards the new hands make reachable. **Do NOT edit `rules/heads_test.go`** — the one regeneration is the merge gate's. `make sim` must still print `20`. Record `make report | grep '^cards:'`.

```bash
git add rules/engine.go rules/turn.go rules/mulligan.go rules/mulligan_test.go decision/ seat/bot.go rules/testbot_test.go rules/acceptance_test.go
git commit -m "feat(rules): the London mulligan — Config.Mulligans, keep/mulligan and bottoming (M2d-1)

Chain heads move (measured, per FL-76 bisect, main-before vs main-after):
<2/4/6/8 old -> new>. Cause: the acceptance games now run a keep/mulligan
round that changes every opening hand. rules/heads_test.go is NOT edited
here; M2d regenerates it once at the merge gate."
```

---

## M2d-2: `KModes`, the modal pick, and the mid-resolution ask (closes R-8)

**Files:**
- Create: `rules/resolution.go` (the mid-resolution ask + resume; the R-8 public face in `rules`), `rules/resolution_test.go`
- Modify: `effects/registry.go` (`Host.Ask`), `effects/misc.go` (`effCharm` asks instead of always-first-mode), `effects/copy.go` (the R-8 `if` — lines 34–39), `events/event.go` + `events/apply.go` (append `ModeChosen`; add its `case` and `kindNames`), `rules/stack.go` (`resolveTop` detects a mid-resolution ask and suspends), `rules/engine.go` (`resume *resumePoint`), `rules/turn.go` (`handleModes` and the resume re-entry), `decision/decision.go` (doc for the declared `KModes`), `seat/bot.go` + `rules/testbot_test.go` (a real `KModes` case)
- Docs folded in as explicit cleanup (the two dangling references design §2 found — **dangling references note**): `effects/misc.go:158-160`'s "a real choice is Task 20's KModes decision" (M2r Task 20 shipped hygiene minors, no modes work; the comment now describes the real choice), the M2r plan `docs/superpowers/plans/2026-09-05-gorge-m2r-ratchet-to-zero.md` line 41's R-8 text "M2b's KModes/Charm machinery" (M2b-1..5 are all human-seat plumbing; point it at M2d-2), `effects/copy.go`'s own comment (notes the now-real ask), and `AGENTS.md`'s Known-approximations line for `UnlessCost$` (milestone changes from "M2b" to "M2d-2").

**Interfaces:**
- Produces:

```go
// effects.Host gains:
// Ask poses a decision in the middle of a resolution. It sets the host's
// pending decision, sets the mid-resolution resume state, and returns true.
// A true return tells the calling effect to stop and wait: the resolution is
// suspended, and the answered decision re-enters it (rules' Engine.Ask is
// what runs the resume continuation). false means the host cannot ask now.
Ask(d *decision.Decision) bool

// rules/resolution.go:
type resumePoint struct {
	kind string       // "modes" | "unless_pay" (plain tag, clone-safe)
	obj  state.ObjID  // the stack object whose resolution is suspended
}
// Engine gains:
resume *resumePoint  // nil = no suspended resolution (Clone default is nil)
func (e *Engine) Ask(d *decision.Decision) bool { e.ask(d); return true }

// events: append ModeChosen Kind after the last kind, name "mode_chosen":
//   Obj = the stack object, Player = the chooser, Text = csv of the chosen
//   mode/sub-ability labels in execution order. Its Apply records the chose
//   on Ctx only (see below); it does NOT write a state.Object field.
```

- `KModes` decision shape: `Min==Max==CharmNum` (default 1) over one `Kind "mode"` option per `Choices$` sub-ability (`Label` = the `SpellDescription$`, `Obj` = the source), `Player` = the controller. The chosen modes execute in the chosen order, through ordinary `Resolve`, each emitting its own effects/events.
- Mid-resolution ask mechanics (this is R-8's "what suspends the resolution"): `rules/stack.go` `resolveTop` runs the top object's effects; when `e.resume != nil` **after** a resolve pass, it clears `e.resume`'s redundant state but leaves the object on the stack pending and returns to `Advance` (a decision is pending). `handle`'s `KModes` (and the `unless_pay` yes/no, shaped as `KModes` or a `KChoose` "yes"/"no") records the answer — for modes a `ModeChosen` event; for unless_pay a `ModeChosen`-tagged yes/no or a `KChoose`-recorded answer — then calls `e.resumeResolution()` which re-enters the suspended effect's continuation with the answer available on the suspended `Ctx` (the `ModeChosen` event is what the log-carried answer reads, so replay re-derives the same branch). Everything after the ask runs through `events.Emit`, so the whole suspended tail replays.
- `effCharm` (`effects/misc.go`): when the host can ask, pose a `KModes` decision over `Choices$` and run the chosen modes on the answer in order; when the host cannot ask (a rules-internal fuzz context with no engine), fall back to today's first-mode default with a `Note` (keeps R-9's stand-in alive for contexts with no engine, documented).
- `effCopySpellAbility` (`effects/copy.go`) — **the R-8 `if`**:

```go
	// UnlessCost$: on the first mid-resolution ask, the target's controller
	// decides "may pay {cost}?. If they do, the copy is made (UnlessSwitched$
	// retargets to the payer); if they do not, no copy. Declined Note stays
	// only for hosts that cannot ask. R-8 closed by M2d-2.
	if _, hasUnless := sa.Params["UnlessCost"]; hasUnless {
		payer := c.Controller // UnlessPayer$ TargetedOrController (both reference the target's controller here)
		if h.Ask(&decision.Decision{Kind: decision.KModes, Player: payer,
			Min: 1, Max: 1, Options: []decision.Option{{Index: 0, Kind: "mode", Label: "Pay " + sa.Params["UnlessCost"] + " — make a copy"},
				{Index: 1, Kind: "mode", Label: "Don't pay"}}}) {
			return // resolution suspended; resume runs this continuation with the answer
		}
		// Fuzz/no-engine host: deterministic decline (R-9), as today.
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "may pay declined (UnlessCost not asked on this host)"})
		return
	}
```

  The resumed continuation: on "Pay" the payer pays `UnlessCost` (through `payMana`/`effects` payment so it replays) and the copy loop proceeds (the `for n := …` below the block); on "Don't pay" the function returns with no `StackCopy`. `UnlessSwitched$` retargets the copy to the payer — a documented first-pass (the switch token switching is beyond the acceptance decks' need; the `AGENTS.md` approximation list names it with the milestone that removes it).
- Bot policy `KModes` (both bots, verbatim mirror): choose the first `CharmNum` options in order (`Choices = [0, 1, …, Min-1]`). This is the real recorded version of today's engine-side first-mode stand-in (R-9), so bot-vs-bot *behaviour* is largely unchanged — but every `KModes` ask now logs `DecisionAsk`/`DecisionMade` events, so the acceptance heads still move (see gate note below).

- [ ] **Step 1: Write the failing tests**

Under `rules/square`, `rules/resolution_test.go` and `effects`'s `misc_test.go`:

```go
// rules/resolution_test.go
func TestCharmAsksForItsModeAndRunsTheChoice(t *testing.T) {
	// A Charm-shaped card: Choices = two sub-abilities; the bot picks the
	// first, and the log records a KModes decision. With both bots the mode
	// choice is now a decision, so the game terminates exactly as before but
	// the log contains the DecisionAsk/DecisionMade pair plus a ModeChosen.
	e, cfg, id := newFixtureDeckWithModes(t, ...) // a two-choice Charm in seat 0
	e.Advance()
	// until the charm on the stack resolves, the bot answer answers KModes
	passUntilCastAndResolve(t, e)
	if !hasEventKind(e, events.ModeChosen) {
		t.Fatal("no ModeChosen event recorded the modal pick")
	}
	replayCheck(t, e, cfg)
}

func TestUnlessPayPausedAndResumed(t *testing.T) {
	// Chain Lightning carry: during resolution the engine asks a KModes
	// yes/no to the opponent; the opponent's answer (pay or not) resumes the
	// resolution. Assert that the object STAYED on the stack while pending
	// (was not popped), that the StackCopy is made only on "pay", and that
	// the game replays exactly.
	chain := ... // the corpus-shaped UnlessCost$ card (rules/storm_test.go's TestChainLightning)
	e, cfg, ch := newFixtureDeck(t, 92, chain)
	addMana(t, e, 0, "R")
	addMana(t, e, 1, "RR")
	castObj(t, e, ch)
	// the engine must present the pay decision mid-resolution, object on stack
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected a mid-resolution pay decision, got %+v", d)
	}
	if o := e.G.Obj(ch); o.Zone != state.ZStack {
		t.Fatalf("resolution must suspend WITH the spell on the stack, zone %s", o.Zone)
	}
	submitChoices(t, e, 0) // "pay"
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(ch).Zone != state.ZGraveyard { /* and a copy exists */ }
	replayCheck(t, e, cfg)
}
```

`effects/misc_test.go`: augment the existing Charm test to assert that when the host's `Ask` returns true, a `KModes` decision is pending and no mode sub-ability has run yet (the resolution suspended before executing); and that a fake host whose `Ask` returns false keeps today's first-mode behaviour with a `Note`. `events/apply_test.go`: add a purity append for `ModeChosen` (Applies with a valid player record nothing on objects and no-panics on invalid player). Run: `go test ./rules/ -run 'Resolution|Charm|Unless|MidResolution' ; go test ./effects/ -run 'Charm|Unless' ; go test ./events/ -run 'ModeChosen'` — FAIL (no `Host.Ask` yet, `effCharm`/`effCopySpellAbility` do not ask, `resolveTop` does not suspend).

- [ ] **Step 2: Implement**

The above, in dependency order: `events/event.go` append `ModeChosen` + `kindNames`; `events/apply.go` a no-object-mutation guarded `case ModeChosen` (records nothing on `state`; a `Note`-style passthrough so the log carries it). `effects/registry.go` add `Ask(d *decision.Decision) bool` to `Host` and every test `Host` double (grep `AddContinuous(`; a stub returning `false` preserves today's defaults); `rules/engine.go` type-assert `Host` to `*Engine` inside the `Ask`-needing call sites, and implement `(e *Engine) Ask`; add `resume *resumePoint` to `Engine`. `rules/stack.go` `resolveTop`: after each effective resolve pass, `if e.resume != nil { return }` (leave the object on the stack — do not enter the graveyard-exit path — and return to the loop with a decision pending); the answered `handle` for `KModes`/the pay form records `ModeChosen` and calls back into the suspended continuation. `effect/misc.go` `effCharm` and `effects/copy.go` `effCopySpellAbility` as above. `decision/decision.go` KModes doc. Bots: the verbatim `KModes` first-`CharmNum` case. The cleanup comment edits (the **dangling references note**) land in this same commit.

- [ ] **Step 3: Measure, record, commit**

Run changed packages green, plus `TestRepoDecksPlayAtEverySeatCount|TestHeads` → all four heads move again (every Charm/modal in the 12 decks now logs a decision even though the bot keeps the first mode; a chain head hash includes `DecisionAsk`/`DecisionMade`/`ModeChosen`). Per FL-76, the bisect is main-after-M2d-1 vs main-after-M2d-2; the divergence is the first `KModes` ask. Ensure the acceptance games still terminate within the 400 000-intent budget (they must — the bot answers every ask). `make sim` 20. Record `make report`. **Do not touch `heads_test.go`.**

```bash
git add effects/ events/ rules/ decision/ seat/bot.go rules/testbot_test.go docs/superpowers/plans/ AGENTS.md
git commit -m "feat(rules): KModes — the modal pick and the mid-resolution ask; R-8 closed

Mid-resolution asks suspend the top-of-stack resolution and resume from the
recorded answer; effCharm poses a real mode choice; effCopySpellAbility's
UnlessCost\$ closure is one if by the R-8 ruling, physically at
effects/copy.go (Chain Lightning's may-pay). Chain heads move again
(<2/4/6/8 old -> new>, measured by bisect -- the first mode decision in the
dedicated deck's game). heads_test.go still not regenerated (merge gate)."
```

---

## M2d-3: concede (audit item 12)

**Files:**
- Modify: `rules/turn.go` (`handlePriority` "concede" option; the priority option list adds `"concede"` as the last option when the player has a seat), `rules/engine.go` (none beyond what exists — the option lives in `legalActions`-adjacent priority option building), `rules/sba_test.go` (a concede-ending test), `seat/bot.go` + `rules/testbot_test.go` (an explicit comment that the bot never picks the option; no behavioural case needed — the existing per-`Kind` pass logic ignores unknown option kinds)
- Unchanged on purpose: `events/event.go`, `events/apply.go` (reuses `PlayerLost`), `rules/sba.go` (`checkGameOver` already ends the game), `rules/heads_test.go`

**Interfaces:**
- Produces: a `"concede"` option (Kind `"concede"`, `Label "Concede"`, always last, offered to a seat on every priority decision) whose taken path emits `PlayerLost{Player, Text: "conceded"}` and lets `checkGameOver` name the remaining seat winner. The wire shape is unchanged; the option is just another `Option.Kind` on a priority decision. Bots never pick it, so pure-bot games' event streams are unchanged and concede **does not move a golden** (an untaken option is not an event). A conceded game replays to its own head because the conceding answer is a recorded `Intent` and the `PlayerLost` event is a re-derived event.

- [ ] **Step 1: Write the failing test**

```go
// rules/sba_test.go or a new rules/concede_test.go
func TestConcedeAtPriorityEndsTheGameForTheOtherSeat(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 131, "Name:Bear\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	e.Advance()
	// find the priority option whose Kind == "concede"; choose it.
	opt := highestPriorityOptionWithKind(t, e, "concede")
	submitChoices(t, e, opt.Index)
	if !e.G.Over || e.G.Winner != 1 || e.G.Players[0].Lost {
		t.Fatalf("after seat 0 concedes: over=%v winner=%d", e.G.Over, e.G.Winner)
	}
	replayCheck(t, e, cfg) // the conceded game replays to the same head
}
```

`highestPriorityOptionWithKind` — a one-line helper scanning a priority decision's options for a kind. Run: `go test ./rules/ -run 'Concede'` — FAIL (no `"concede"` option is offered).

- [ ] **Step 2: Implement**

In the priority-option builder (`handlePriority`, `rules/turn.go`), after the explicit `pass` option, append `decision.Option{Kind: "concede", Label: "Concede"}` (guarded: only when the seat is alive and the game is running — which it always is at priority). The `handlePriority` switch's option-dispatch gains `case "concede": e.emit(events.Event{Kind: events.PlayerLost, Player: in.Player, Text: "conceded"}); e.checkStateBased()`. `PlayerLost`'s existing `Apply` marks the seat `Lost`; `checkGameOver` records the last remaining seat as `Winner` and sets `Over`. Add a one-sentence note to `handlePriority`'s comment and to `rules/sba.go`'s `checkGameOver` doc that a concession is a way to be `Lost`. Bots: add a brief guarded-comment in `botDecide`/`answer` that unknown option kinds like `"concede"` are never picked (the existing explicit `pass` path already precedes any blind clamp, and clamp's top-up picks legal actions only). Update `rules/heads_test.go` doc? No — unchanged.

- [ ] **Step 3: Run, record, commit**

`go test ./rules/ -run 'Concede|Heads|RepoDeck'` — bots never concede, events unchanged, so `TestHeads` is unchanged from M2d-2's post-commit value (drop a line in the commit body that concede moved no head because no bot picks it). `make sim` 20. Record `make report`.

```bash
git add rules/turn.go seat/bot.go rules/testbot_test.go rules/sba_test.go
git commit -m "feat(rules): concede — a priority option that emits PlayerLost (M2d-3)

Least-surface shape (controller R-M3): no Registry method, no new event kind;
choosing the priority 'concede' option emits the existing PlayerLost event,
so a conceded game replays to the same chain head it was played to. Bots never
pick it, so no golden moves here over M2d-2."
```

---

## M2d merge gate (the one golden regeneration — the controller's job)

When all three M2d tasks are merged to the M2d branch (they are one slot), the controller, **not an individual task**, closes M2d:

1. **Rebase the M2d branch onto `main` once R15 is in** (M2d started after R15; the merge rebases onto whatever `main` has since gained). Resolve the one expected conflict if M2r's protection work touched the same `rules/turn.go`/`engine.go` regions M2d did (its changes are the R-8/R-9-adjacent ones; keep both).
2. **Regenerate `rules/heads_test.go`** — the single allowed write to `acceptanceHeads`. For each of 2/4/6/8, run the acceptance game, take the new head, and edit the table **with a named cause per seat count** in the commit body (the measured build-bisect cause M2d-1 and M2d-2 recorded). If a seat count's new head still equals its pre-M2d value (theoretically impossible per §3, but verify), say so.
3. **Recompute the ratchet** (`knownUnsupported`) from the merged M2d tree — it can move in either direction. If the merged count changed, edit `rules/acceptance_test.go`'s table comment to the *measured* value and do **not** fit it back to a preconceived number. Re-run `TestEveryRepoDeckIsFullySupported` to green.
4. Full merge gate (the controller's own): `make lint`, plus `go test -race ./rules/` **only if** the merged diff touched concurrency or lock discipline (the FL-67 race regime — not a blanket full-tree `-race`; M2d is engine-internal single-goroutine work, so it likely does not qualify). `make sim` = 20, `make report`.
5. Only then is M2r Task 21 sequenced: R21's final numbers are measured **after** this regeneration, so they are not stale on arrival.

```bash
git add rules/heads_test.go rules/acceptance_test.go
git commit -m "chore(rules): M2d merge gate — regenerate the four acceptance chain heads (one regeneration)

Cause per seat count: <2/4/6/8 cause>. Ratchet recomputed from the merged
M2d tree: <measured> of <136> (moved of M2d's own making, not 'fixed')."
```

---

## Testing (design §7 → per-task gates)

M2d-1/2/3 are TDD per decision kind, plus the acceptance decks. The real gates are `TestEveryRepoDeckIsFullySupported` and `TestRepoDecksPlayAtEverySeatCount` (mulligan and modes change what the repo decks do) and, once at the merge gate, `TestHeads` (regenerated with a build-bisected cause). Per-task exact gates the implementer runs (never `go test ./...`, never `-race`, never `-count=1` on an ordinary run):

- **M2d-1** `go test ./rules/ ./decision/ ./seat/ ./events/` and `go test ./rules/ -run 'TestRepoDecksPlayAtEverySeatCount|TestHeads|TestEveryRepoDeck' -v`, `make report | grep '^cards:'`, `make sim | grep -c 'replay OK'`. Budget: the acceptance games gain a short pre-game round before turn 1; each game's event count grows by a few hundred events. The recorded `rules/` suite (budget 10s) will likely tick up by ~1–3s from the acceptance games. If `cmd/testtime ./rules` post-change reports ≥ 10s, stop and request a `Test-Budget-Approved` trailer — the grant is the controller's call.
- **M2d-2** `go test ./effects/ ./events/ ./rules/` and the same acceptance/heads run. Budget: mid-resolution asks add a decision per modal/Charm and per Chain-Lightning-style clause; the acceptance games stay within the 400 000-intent budget (the bot answers every ask). This is the task most likely to push `rules/` over 10s (resolution suspension + replay adds per-aspect cost): **state the expected overshoot** if measured and request the `Test-Budget-Approved` trailer in the commit; effects (5s) should stay under (Charm tests are few) but verify.
- **M2d-3** `go test ./rules/ -run 'Concede|Heads|RepoDeck'`, `make sim`. Budget: one small test; no overshoot.

---

## Self-review checklist (run by the plan author before execution)

1. **Scope coverage.** Design §3's three sub-tasks map 1:1: mulligan → M2d-1, modes + mid-resolution (R-8) → M2d-2, concede → M2d-3. Design §8's four open questions are closed: §8.1 concede shape (R-M3, least surface, rejected alternative written), §8.2 mulligan not configurable off for acceptance (R-M1), §8.3 StackView.Text out of scope (R-8.3 non-goal stated), §8.4 free-mulligan count = `Config.Mulligans` (R-8.4, in the Config replay reads). The two dangling references fold into M2d-2.
2. **Sequencing.** Header and collision map state: M2d starts after R15 merges, R21 lands after M2d, the three M2d sub-tasks collide file-for-file and are one slot (M2d-1 → M2d-2 → M2d-3), one golden regeneration at the merge gate only.
3. **Placeholders.** Every `<old -> new>`, `<cards>` in a commit template is a measurement/lookup the implementer fills at commit time, not a plan gap.
4. **Invariant discipline.** Only one new event kind (`ModeChosen`), one new `Config` field, three engine value/pointer fields; no `events.Event` field changes; no direct `state.Game` writes anywhere (the mulligan reshuffle/re-draw/bottom and the modes/unless-pay continuations all go through `e.emit`/`events.Emit`); no `./...`, no `-race`, no `-count=1` on ordinary gates; determinism preserved (slices + seeded bot rngs, never maps).
5. **Determinism under replay.** The whole mulligan round and every suspended-resolution tail are event-driven, so `replay.Replay(log, cfg)` reproduces them byte-for-byte; `Config.Mulligans` travels in the Config replay is handed because it is the Config (R-8.4).
6. **Ratchet honesty.** No task hard-codes a ratchet or head number it cannot know; every head move is build-bisected (FL-76) and attributed in the commit body; the regenerated goldens and recomputed ratchet are the merge gate's.
