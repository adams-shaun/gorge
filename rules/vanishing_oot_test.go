package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// tidewalkerEngine seeds the REAL corpus Tidewalker -- a bare K:Vanishing
// carrier whose own script supplies its counter count (K:etbCounter:TIME:X
// with X = the number of Islands its controller has) -- plus `islands` real
// Islands on seat 0's battlefield, and returns the engine and Tidewalker's id.
// Tidewalker enters with that many TIME counters from its own entry
// replacement, never from any Vanishing expansion, which is exactly the
// dynamic count a synthesized fixed-N Vanishing replacement would corrupt.
func tidewalkerEngine(t *testing.T, islands int) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	tw, ok := reg.Lookup("Tidewalker")
	if !ok {
		t.Fatal("corpus fixture: Tidewalker missing")
	}
	deck := []*cards.Card{tw}
	for i := 0; i < islands; i++ {
		isl, ok := reg.Lookup("Island")
		if !ok {
			t.Fatal("corpus fixture: Island missing")
		}
		deck = append(deck, isl)
	}
	e := New(Config{Seed: 7, Names: []string{"a", "b"}, Decks: [][]*cards.Card{
		append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40),
	}, Tokens: reg.Tokens})
	e.Advance()
	toMain1(t, e)
	for i := 0; i < islands; i++ {
		moveByName(t, e, 0, "Island", state.ZBattlefield)
	}
	id := moveByName(t, e, 0, "Tidewalker", state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Tidewalker is not on battlefield: %+v", o)
	}
	// Tidewalker's own entry replacement only; no Vanishing ETB is expected.
	e.pendingTriggers = nil
	return e, id
}

// TestVanishingTidewalkerDynamicCountUpkeepAndLastCounter proves the bare
// keyword's independent clock on a real carrier whose own script supplies a
// board-derived counter count: Tidewalker enters with three TIME counters
// (three Islands), one controller upkeep removes one, and reaching zero
// sacrifices it.
func TestVanishingTidewalkerDynamicCountUpkeepAndLastCounter(t *testing.T) {
	e, id := tidewalkerEngine(t, 3)
	if got := e.G.Obj(id).Counter("TIME"); got != 3 {
		t.Fatalf("precondition: Tidewalker entered with %d TIME counters, want 3 from its own script (3 Islands)", got)
	}

	// The controller's upkeep queues the Vanishing removal trigger; removal
	// is not a turn-based action.
	e.G.Active, e.G.Priority = 0, 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	if got := e.G.Obj(id).Counter("TIME"); got != 3 {
		t.Fatalf("precondition: TIME changed before the upkeep trigger resolved: %d, want 3", got)
	}
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("controller upkeep put %d stack objects, want one Vanishing removal trigger", len(e.G.Stack))
	}
	e.resolveTop()
	if got := e.G.Obj(id).Counter("TIME"); got != 2 {
		t.Fatalf("after resolving upkeep trigger TIME = %d, want 2 (ticked down from 3)", got)
	}

	// Drain the remainder through the log. The last-counter trigger is a
	// separate stack object, not an immediate sacrifice from CounterChange.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "TIME", Amount: -2})
	if got := e.G.Obj(id).Counter("TIME"); got != 0 {
		t.Fatalf("precondition: last-counter removal left %d TIME counters, want zero (distinct from 2)", got)
	}
	if e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatalf("precondition: last-counter removal sacrificed immediately; zone=%s", e.G.Obj(id).Zone)
	}
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("last-counter removal put %d stack objects, want one sacrifice trigger", len(e.G.Stack))
	}
	e.resolveTop()
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("after resolving last-counter trigger zone = %s, want graveyard", z)
	}
}

// outOfTimeEngine puts the REAL corpus Out of Time -- the other bare
// K:Vanishing carrier -- on seat 0's battlefield and places `counters` TIME
// counters on it directly. Out of Time's own enters trigger counts phased-out
// creatures to place counters, but its `DB$ Phases` step is not implemented in
// this engine (an "unimplemented API Phases" Note; see the report's Issues),
// so the counters this test needs are placed explicitly here. This isolates
// the Vanishing clock -- the thing this ticket fixes -- from that separate
// gap.
func outOfTimeEngine(t *testing.T, counters int32) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	oot, ok := reg.Lookup("Out of Time")
	if !ok {
		t.Fatal("corpus fixture: Out of Time missing")
	}
	e := New(Config{Seed: 7, Names: []string{"a", "b"}, Decks: [][]*cards.Card{
		append([]*cards.Card{oot}, mountainDeck(t, 39)...), mountainDeck(t, 40),
	}, Tokens: reg.Tokens})
	e.Advance()
	toMain1(t, e)
	id := moveByName(t, e, 0, "Out of Time", state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Out of Time is not on battlefield: %+v", o)
	}
	e.pendingTriggers = nil
	if counters > 0 {
		e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "TIME", Amount: counters})
	}
	if got := e.G.Obj(id).Counter("TIME"); got != counters {
		t.Fatalf("precondition: Out of Time holds %d TIME counters, want %d", got, counters)
	}
	return e, id
}

// TestVanishingOutOfTimeUpkeepAndLastCounter proves the other bare carrier
// gets the same parameter-independent clock. Because Out of Time's own count
// step is blocked (see outOfTimeEngine), a one-counter setup is placed and the
// single controller upkeep must remove it and then sacrifice the enchantment.
func TestVanishingOutOfTimeUpkeepAndLastCounter(t *testing.T) {
	e, id := outOfTimeEngine(t, 1)

	e.G.Active, e.G.Priority = 0, 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	if got := e.G.Obj(id).Counter("TIME"); got != 1 {
		t.Fatalf("precondition: TIME changed before the upkeep trigger resolved: %d, want 1", got)
	}
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("controller upkeep put %d stack objects, want one Vanishing removal trigger", len(e.G.Stack))
	}
	e.resolveTop()
	if got := e.G.Obj(id).Counter("TIME"); got != 0 {
		t.Fatalf("after resolving upkeep trigger TIME = %d, want 0 (distinct from 1)", got)
	}
	if e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatalf("precondition: last-counter removal sacrificed immediately; zone=%s", e.G.Obj(id).Zone)
	}
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("last-counter removal put %d stack objects, want one sacrifice trigger", len(e.G.Stack))
	}
	e.resolveTop()
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("after resolving last-counter trigger zone = %s, want graveyard", z)
	}
}
