// Task 29: rules/trigger.go's applyReplacements used to treat every
// matching R:Event$ Moved replacement as Forge's ReplacementResult$
// Replaced -- resolve ReplaceWith$, discard the original event. That is
// wrong for the corpus's dominant shape, ReplacementResult$ Updated ("the
// event still happens, augmented"): a permanent's own MoveZone onto the
// battlefield got swallowed, so it never left the stack and resolveTop
// (rules/stack.go) re-resolved the same object forever. See
// .superpowers/sdd/2026-09-03-mtgcore-m0-m1/task-29-brief.md and Task 26's
// report (grep ReplacementResult) for the corpus measurements and the
// Geralf's Messenger / Hallowed Fountain / Celestial Colonnade repro.
//
// Card text below is authored for this test, in the same R:/SVar$ shape as
// the corpus cards that found the bug -- never copied from Forge's
// .cards/cardsfolder.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// tappedLandSrc is the Hallowed Fountain shape the brief names: a land whose
// only R: line says "you may pay 2 life... enters tapped" -- simplified here
// to the unconditional "enters tapped" the brief's own excerpt shows, since
// the UnlessCost$ branch is not what Task 29 is about.
const tappedLandSrc = `Name:Tide Gate
Types:Land
R:Event$ Moved | ValidCard$ Card.Self | Destination$ Battlefield | ReplaceWith$ DBTap | ReplacementResult$ Updated | Description$ enters tapped
SVar:DBTap:DB$ Tap | Defined$ Self | ETB$ True
Oracle:x
`

// tappedCreatureSrc is the Geralf's Messenger shape: the identical R: line
// on a creature instead of a land, cast and resolved off the stack rather
// than played directly.
const tappedCreatureSrc = `Name:Grave Walker
ManaCost:B
Types:Creature Zombie
PT:2/2
R:Event$ Moved | ValidCard$ Card.Self | Destination$ Battlefield | ReplaceWith$ DBTap | ReplacementResult$ Updated | Description$ enters tapped
SVar:DBTap:DB$ Tap | Defined$ Self | ETB$ True
Oracle:x
`

// fullyReplacedEntrySrc is a permanent spell whose ETB replacement is an
// explicit ReplacementResult$ Replaced -- unlike tappedCreatureSrc, its
// ReplaceWith$ (RepLife) never moves the object anywhere, which is exactly
// the resolveTop totality hazard: nothing else ever takes it off the stack.
const fullyReplacedEntrySrc = `Name:Hollow Construct
ManaCost:2
Types:Artifact Creature Golem
PT:2/2
R:Event$ Moved | ValidCard$ Card.Self | Destination$ Battlefield | ReplaceWith$ RepLife | ReplacementResult$ Replaced | Description$ x
SVar:RepLife:DB$ GainLife | Defined$ You | LifeAmount$ 1
Oracle:x
`

// watcherSrc is an unrelated permanent with a plain "whenever a permanent
// enters the battlefield" trigger, for test 6: proof that checkTriggers
// still sees an Updated Move.
const watcherSrc = `Name:Sentinel
ManaCost:1
Types:Artifact
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card | Execute$ TrigGain | TriggerDescription$ x
SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 1
Oracle:x
`

// newFixtureDeck builds a 2-seat engine the same way newSeats does, except
// seat 0's deck leads with fixture instead of an all-mountain 40, so
// cfg.Decks -- and therefore replayFromLog's own genesis reconstruction --
// actually contains it (Ruling: an object added to a live Engine after New
// via a direct AddObject call, the way several existing tests in this
// package do it, is invisible to replayFromLog, which only rebuilds from
// cfg.Decks; test 5 below needs the object to line up by ID in both).
//
// Genesis's shuffle (seeded, but not something this test controls the
// outcome of) may land fixture in the opening hand or leave it in the
// library; either way it ends up in seat 0's hand before returning, via a
// logged MoveZone bridge in the library case. That bridge is deliberately
// NOT counted by any assertion below that cares how many times the fixture
// entered the BATTLEFIELD -- it is unrelated setup, not part of what Task
// 29 changed.
//
// This deliberately stops at turn 1's upkeep (Advance's own first priority
// ask) rather than driving all the way to main1 itself: a pending
// decision's Options is a snapshot legalActions computed once, when ask()
// built it, so any further raw e.emit setup a caller still needs (funding
// mana for a cast, say) must happen BEFORE the main1 priority ask is built,
// not after -- callers that need such setup must do it here, at upkeep,
// then call driveToStep(t, e, 1, 0, state.StepMain1) themselves once done.
func newFixtureDeck(t *testing.T, seed uint64, fixtureSrc string) (*Engine, Config, state.ObjID) {
	t.Helper()
	fixture := card(t, fixtureSrc)
	name := fixture.Faces[0].Name
	cfg := Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{fixture}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
		}}
	e := New(cfg)
	e.Advance()

	var id state.ObjID
	for _, cand := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(cand).Face().Name == name {
			id = cand
		}
	}
	if id == 0 {
		for _, cand := range e.G.Zone(state.ZLibrary, 0) {
			if e.G.Obj(cand).Face().Name == name {
				id = cand
			}
		}
		if id == 0 {
			t.Fatalf("fixture %q not found in seat 0's hand or library", name)
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
	}
	return e, cfg, id
}

// countMoves reports how many times id was moved to exactly `to` anywhere in
// the log -- the metric applyReplacements' Updated fix is actually about,
// independent of whatever unrelated setup (a library->hand bridge, mana
// funding) also touched the log.
func countMoves(log []events.Event, id state.ObjID, to state.Zone) int {
	n := 0
	for _, ev := range log {
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.To == to {
			n++
		}
	}
	return n
}

func countKind(log []events.Event, k events.Kind, id state.ObjID) int {
	n := 0
	for _, ev := range log {
		if ev.Kind == k && ev.Obj == id {
			n++
		}
	}
	return n
}

// passUntilStackEmpty answers "pass" priority decisions until the stack is
// empty, stopping the instant it is -- unlike passAll's fixed budget, which
// would happily keep passing turn after turn once the spell under test has
// long since resolved, running the very next turn's untap step and undoing
// the "entered tapped" state this whole test file exists to check. Bounded
// the same way passThroughStep is, so a real non-termination regression
// (this file's whole point) fails an assertion instead of hanging the test
// binary.
func passUntilStackEmpty(t *testing.T, e *Engine, limit int) int {
	t.Helper()
	n := 0
	for ; n < limit && !e.G.Over && len(e.G.Stack) > 0; n++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("non-priority decision %+v while draining the stack", d)
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	return n
}

// playTappedLand drives newFixtureDeck's land through the play_land option.
// Shared by test 1 (direct assertions) and test 5 (replay fidelity) so both
// exercise the exact same path.
func playTappedLand(t *testing.T, seed uint64) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg, id := newFixtureDeck(t, seed, tappedLandSrc)
	driveToStep(t, e, 1, 0, state.StepMain1)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected seat 0's priority after genesis, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "play_land" && opt.Obj == id {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no play_land option for the fixture land: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit play_land: %v", err)
	}
	return e, cfg, id
}

// castAndResolveTappedCreature drives newFixtureDeck's creature through the
// cast option, funds its cost, and then drains priority -- bounded, per the
// brief's instruction not to let a real regression hang the test process --
// until it resolves off the stack. Shared by test 2 and test 5.
//
// Funding happens AFTER driving to main1, not before: setStep clears every
// player's mana pool on every step transition (CR 500.4, turn.go's own
// setStep), so mana funded back at upkeep (before Draw's and Main1's own
// transitions) would already be gone by the time main1's priority is asked.
// But a pending decision's Options is a snapshot legalActions computed once
// at ask() time (the same fact newFixtureDeck's doc comment calls out), so
// funding mana after driveToStep returns would land in the pool too late for
// the "cast" option to already be in the Options this test then needs to
// search -- e.priorityRound() is called again directly (the unexported
// method this package's own priority machinery uses, safe to call twice:
// grantPriority re-asks the same holder, it does not advance anything) so
// the decision this test actually inspects reflects the funded pool.
func castAndResolveTappedCreature(t *testing.T, seed uint64) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg, id := newFixtureDeck(t, seed, tappedCreatureSrc)
	driveToStep(t, e, 1, 0, state.StepMain1)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "B", Amount: 1})
	e.priorityRound()

	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected seat 0's priority after genesis, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == id {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the fixture creature: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}
	if len(e.G.Stack) != 1 || e.G.Stack[0] != id {
		t.Fatalf("stack = %v, want [%d] right after casting", e.G.Stack, id)
	}

	const bound = 60
	passUntilStackEmpty(t, e, bound)
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v after %d bounded priority passes -- did not resolve "+
			"(possible infinite re-resolution, Task 29's defect)", e.G.Stack, bound)
	}
	return e, cfg, id
}

// TestUpdatedReplacementAppliesTheOriginalMove is brief test 1. At c19097f
// this fails: applyReplacements discards the land's hand->battlefield Move
// unconditionally, DBTap's own Tap primitive is a no-op for an object not
// already on the battlefield (effects/combatfx.go effTap), and the land is
// simply left in hand, untapped, forever.
func TestUpdatedReplacementAppliesTheOriginalMove(t *testing.T) {
	e, _, id := playTappedLand(t, 100)

	if got := e.G.Obj(id).Zone; got != state.ZBattlefield {
		t.Fatalf("zone = %s, want battlefield -- an Updated replacement must not discard the "+
			"original Move", got)
	}
	if !e.G.Obj(id).Tapped {
		t.Fatal("Tide Gate entered untapped -- ReplaceWith$ DBTap did not run, or ran before " +
			"the Move and found nothing on the battlefield to tap")
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want empty (a land never touches the stack)", e.G.Stack)
	}
	if n := countMoves(e.L.Events, id, state.ZBattlefield); n != 1 {
		t.Fatalf("logged %d MoveZone(->battlefield) for the land, want exactly 1: an Updated "+
			"replacement augments the original event once, it does not re-fire it", n)
	}
	if n := countKind(e.L.Events, events.Tap, id); n != 1 {
		t.Fatalf("logged %d Tap events for the land, want exactly 1 -- the ReplaceWith ran once", n)
	}
}

// TestUpdatedReplacementOnACreatureSpellResolvesOnce is brief test 2. At
// c19097f this hangs (bounded here to 60 priority passes rather than the
// engine's own 400000-intent budget): applyReplacements swallows the
// permanent's stack->battlefield Move every time, so resolveTop finds the
// same object on top again on the next pass and resolves it again, forever.
func TestUpdatedReplacementOnACreatureSpellResolvesOnce(t *testing.T) {
	e, _, id := castAndResolveTappedCreature(t, 101)

	if got := e.G.Obj(id).Zone; got != state.ZBattlefield {
		t.Fatalf("zone = %s, want battlefield", got)
	}
	if !e.G.Obj(id).Tapped {
		t.Fatal("Grave Walker resolved untapped -- ReplaceWith$ DBTap did not take effect")
	}
	if n := countKind(e.L.Events, events.Resolve, id); n != 1 {
		t.Fatalf("logged %d Resolve events, want exactly 1 -- no re-resolution", n)
	}
	if n := countMoves(e.L.Events, id, state.ZBattlefield); n != 1 {
		t.Fatalf("logged %d MoveZone(->battlefield) for the creature, want exactly 1", n)
	}
	resolveAt := -1
	for i, ev := range e.L.Events {
		if ev.Kind == events.Resolve && ev.Obj == id {
			resolveAt = i
			break
		}
	}
	if resolveAt < 0 {
		t.Fatal("no Resolve event found for the creature")
	}

	// The game must actually proceed, not sit resolving the same object:
	// some StepChange must follow the (sole) Resolve event. Driven further
	// AFTER the Tapped/Zone/count assertions above, deliberately: the next
	// turn's untap step would otherwise legitimately untap the creature
	// again before this got to look, which is correct game behaviour but
	// would make this test flaky about the wrong thing.
	passAll(t, e, 20)
	advanced := false
	for _, ev := range e.L.Events[resolveAt+1:] {
		if ev.Kind == events.StepChange {
			advanced = true
			break
		}
	}
	if !advanced {
		t.Fatal("no StepChange logged after the creature resolved -- the game did not proceed " +
			"to the next step")
	}
}

// TestReplacedReplacementStillDiscardsTheMove is brief test 3: the Task 22
// fixture shape (no ReplacementResult$ at all, i.e. absent == Replaced) on a
// permanent being destroyed must keep discarding the Move exactly as before
// -- unchanged behaviour, pinned so the Updated branch above cannot regress
// it.
func TestReplacedReplacementStillDiscardsTheMove(t *testing.T) {
	e := newSeats(t, 2)
	guardian := onBoard(t, e, 0, `Name:Relic Guardian
ManaCost:1 G
Types:Creature Wall
PT:0/4
R:Event$ Moved | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | ReplaceWith$ RepLife
SVar:RepLife:DB$ GainLife | Defined$ You | LifeAmount$ 1
Oracle:x
`)
	before := e.G.Players[0].Life

	e.emit(events.Event{Kind: events.MoveZone, Obj: guardian, From: state.ZBattlefield, To: state.ZGraveyard})

	if got := e.G.Obj(guardian).Zone; got != state.ZBattlefield {
		t.Fatalf("zone = %s, want battlefield -- absent ReplacementResult$ must still fully "+
			"replace (discard) the Move", got)
	}
	if got := e.G.Players[0].Life - before; got != 1 {
		t.Fatalf("life gained = %d, want 1", got)
	}
}

// TestPermanentSpellWhoseEntryIsFullyReplacedDoesNotStickOnTheStack is brief
// test 4: an explicit ReplacementResult$ Replaced ETB replacement whose
// ReplaceWith$ never moves the object is exactly the totality hazard
// resolveTop's new guard exists for. At c19097f this hangs the same way
// test 2 does (bounded here for the same reason).
func TestPermanentSpellWhoseEntryIsFullyReplacedDoesNotStickOnTheStack(t *testing.T) {
	e, _, id := newFixtureDeck(t, 102, fullyReplacedEntrySrc)
	driveToStep(t, e, 1, 0, state.StepMain1)
	// Fund and re-ask, not fund-then-drive: see castAndResolveTappedCreature's
	// doc comment (setStep clears pools every step transition, and a pending
	// decision's Options is a one-time snapshot) for why.
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "C", Amount: 2})
	e.priorityRound()

	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected seat 0's priority after genesis, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == id {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the fixture creature: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}

	const bound = 60
	passAll(t, e, bound)
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v after %d bounded priority passes -- the fully-replaced entry stuck "+
			"on the stack instead of being swept off it (Task 29's resolveTop guard)", e.G.Stack, bound)
	}
	if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
		t.Fatalf("zone = %s, want graveyard", got)
	}
	found := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Obj == id {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no Note explaining the graveyard sweep was logged")
	}
	if e.Pending() == nil {
		t.Fatal("no further decision pending -- the engine did not reach the next decision")
	}
}

// TestUpdatedReplacementReplaysFaithfully is brief test 5: fold the logs of
// test 1 and test 2's scenarios alone into fresh state.Games (replayFromLog,
// this package's own log-alone reconstruction harness) and diff them
// against the live games. Separate seeds from tests 1/2 -- deliberately not
// sharing state with them -- but the exact same helper functions, so this
// exercises precisely what those tests exercised.
func TestUpdatedReplacementReplaysFaithfully(t *testing.T) {
	t.Run("land", func(t *testing.T) {
		e, cfg, _ := playTappedLand(t, 200)
		fresh := replayFromLog(t, cfg, e.L.Events)
		if diff := diffGames(e.G, fresh); diff != "" {
			t.Fatalf("replay from the log alone diverged from the live game:\n%s", diff)
		}
	})
	t.Run("creature", func(t *testing.T) {
		e, cfg, _ := castAndResolveTappedCreature(t, 201)
		fresh := replayFromLog(t, cfg, e.L.Events)
		if diff := diffGames(e.G, fresh); diff != "" {
			t.Fatalf("replay from the log alone diverged from the live game:\n%s", diff)
		}
	})
}

// TestUpdatedReplacementStillFiresETBTriggers is brief test 6: a "whenever a
// permanent enters" trigger must still see an Updated entry -- proof that
// routing the original event through events.Emit + checkTriggers directly
// (rather than through e.emit, which would re-run replacement matching on
// it) still reaches checkTriggers, exactly as an ordinary unreplaced Move
// would.
func TestUpdatedReplacementStillFiresETBTriggers(t *testing.T) {
	e, _, landID := newFixtureDeck(t, 202, tappedLandSrc)
	onBoard(t, e, 0, watcherSrc)
	driveToStep(t, e, 1, 0, state.StepMain1)
	before := e.G.Players[0].Life

	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected seat 0's priority after genesis, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "play_land" && opt.Obj == landID {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no play_land option for the fixture land: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit play_land: %v", err)
	}

	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want the Sentinel's ETB trigger queued -- the Updated replacement "+
			"must not hide the Move from checkTriggers", e.G.Stack)
	}
	e.resolveTop()
	if got := e.G.Players[0].Life - before; got != 1 {
		t.Fatalf("life gained = %d, want 1 from the ETB trigger firing once for the land's entry", got)
	}
}
