package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func onBoard(t *testing.T, e *Engine, p state.PlayerID, src string) state.ObjID {
	t.Helper()
	o := e.G.AddObject(card(t, src), p)
	o.Zone = state.ZBattlefield
	e.G.Clock++
	o.Timestamp = e.G.Clock
	e.G.SetZone(state.ZBattlefield, p, append(e.G.Zone(state.ZBattlefield, p), o.ID))
	return o.ID
}

func layerEngine(t *testing.T) *Engine {
	t.Helper()
	return New(Config{Seed: 1, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
}

func TestDerivedStartsFromPrintedCharacteristics(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nK:Trample\nOracle:x\n")
	d := e.Derived(id)
	if d.Power != 2 || d.Toughness != 2 {
		t.Fatalf("P/T = %d/%d", d.Power, d.Toughness)
	}
	if !e.HasKeyword(id, "Trample") {
		t.Error("printed keyword missing")
	}
}

func TestLayer7dCountersAddToPowerAndToughness(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.G.Obj(id).AddCounter("P1P1", 3)
	if got := e.Power(id); got != 5 {
		t.Fatalf("power = %d, want 5", got)
	}
	if got := e.Toughness(id); got != 5 {
		t.Fatalf("toughness = %d, want 5", got)
	}
}

func TestLayer7cModificationsStack(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{Source: id, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", Controller: 0, AddPower: 3, AddToughness: 3, UntilEOT: true})
	e.AddContinuous(ContinuousEffect{Source: id, Timestamp: 2, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", Controller: 0, AddPower: 1, AddToughness: 0, UntilEOT: true})
	if got := e.Power(id); got != 6 {
		t.Fatalf("power = %d, want 6", got)
	}
	if got := e.Toughness(id); got != 5 {
		t.Fatalf("toughness = %d, want 5", got)
	}
}

// Layer order is the whole point: a set-P/T effect must be overwritten by a
// later set effect, and both must be applied before modifications and counters.
func TestSetBeforeModifyRegardlessOfTimestamp(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	// The +2/+2 has the EARLIER timestamp; the 1/1 set has the later one.
	e.AddContinuous(ContinuousEffect{Source: id, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", AddPower: 2, AddToughness: 2, UntilEOT: true})
	e.AddContinuous(ContinuousEffect{Source: id, Timestamp: 2, Layer: LPT, Sub: SubSet,
		Affects: "Card.Self", SetPower: 1, SetToughness: 1, HasSet: true, UntilEOT: true})
	e.G.Obj(id).AddCounter("P1P1", 1)
	// 1/1 set, then +2/+2 modification, then +1/+1 counter.
	if got, want := e.Power(id), int32(4); got != want {
		t.Fatalf("power = %d, want %d", got, want)
	}
}

func TestLayer6GrantsKeywords(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	if e.HasKeyword(id, "Flying") {
		t.Fatal("bear should not start with flying")
	}
	e.AddContinuous(ContinuousEffect{Source: id, Timestamp: 1, Layer: LAbilities,
		Affects: "Card.Self", AddKeywords: []string{"Flying"}, UntilEOT: true})
	if !e.HasKeyword(id, "Flying") {
		t.Fatal("granted keyword missing")
	}
}

func TestAffectsFilterScopesTheEffect(t *testing.T) {
	e := layerEngine(t)
	mine := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	theirs := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{Source: mine, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Creature.YouCtrl", Controller: 0, AddPower: 1, AddToughness: 1})
	if e.Power(mine) != 3 {
		t.Errorf("own creature power = %d, want 3", e.Power(mine))
	}
	if e.Power(theirs) != 2 {
		t.Errorf("opponent creature power = %d, want 2", e.Power(theirs))
	}
}

func TestEndOfTurnCleanupDropsUntilEOTEffects(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{Source: id, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", AddPower: 3, UntilEOT: true})
	e.AddContinuous(ContinuousEffect{Source: id, Timestamp: 2, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", AddPower: 1})
	if e.Power(id) != 6 {
		t.Fatalf("power = %d, want 6", e.Power(id))
	}
	e.EndOfTurnCleanup()
	if e.Power(id) != 3 {
		t.Fatalf("power after cleanup = %d, want 3", e.Power(id))
	}
}

func TestEffectsFromObjectsThatLeftAreDropped(t *testing.T) {
	e := layerEngine(t)
	lord := onBoard(t, e, 0, "Name:Lord\nManaCost:2 W\nTypes:Creature Human\nPT:2/2\nOracle:x\n")
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{Source: lord, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Creature.YouCtrl+Other", Controller: 0, AddPower: 1, AddToughness: 1})
	if e.Power(bear) != 3 {
		t.Fatalf("power = %d, want 3", e.Power(bear))
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: lord,
		From: state.ZBattlefield, To: state.ZGraveyard})
	if e.Power(bear) != 2 {
		t.Fatalf("power after the lord left = %d, want 2", e.Power(bear))
	}
}

// TestAddContinuousClockAdvanceIsReplayable is the Ruling T19-a regression
// test: AddContinuous's Timestamp auto-stamp must advance Game.Clock only
// through a logged ClockTick event, never a direct field write, so folding
// the events these two calls produced into a snapshot taken just before
// them -- through events.Apply alone, no Engine, no rules code involved --
// reaches the same Clock the live game did. This is the same invariant
// fix1_test.go's TestReplayReconstructsPassesAndPriority locks in for
// Passes/Priority.
//
// The snapshot is taken immediately before the calls under test, rather than
// replaying the whole log from genesis, because onBoard (this file's own
// test helper, above) stamps Object.Timestamp with a direct e.G.Clock++ of
// its own -- test code is allowed to bypass events, but comparing from
// genesis would conflate that unrelated bypass with the one this test
// exists to catch.
func TestAddContinuousClockAdvanceIsReplayable(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	snapshot := e.G.Clone()
	firstNewEvent := len(e.L.Events)

	e.AddContinuous(ContinuousEffect{Source: id, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", AddPower: 1})
	e.AddContinuous(ContinuousEffect{Source: id, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", AddPower: 1})

	wantClock := e.G.Clock
	if wantClock != snapshot.Clock+2 {
		t.Fatalf("live Clock advanced by %d across two AddContinuous calls, want 2", wantClock-snapshot.Clock)
	}

	for _, ev := range e.L.Events[firstNewEvent:] {
		events.Apply(snapshot, ev)
	}
	if snapshot.Clock != wantClock {
		t.Fatalf("Clock reconstructed from the log alone = %d, want %d (live)", snapshot.Clock, wantClock)
	}
}

// TestHasKeywordIsCaseInsensitive is the Ruling T19-b regression test:
// Engine.HasKeyword must match case-insensitively like its sibling
// cards.Face.HasKeyword, for both a printed keyword and one granted by a
// continuous effect.
func TestHasKeywordIsCaseInsensitive(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nK:Trample\nOracle:x\n")
	if !e.HasKeyword(id, "trample") {
		t.Error("HasKeyword should match a printed keyword case-insensitively (lowercase query)")
	}
	if !e.HasKeyword(id, "TRAMPLE") {
		t.Error("HasKeyword should match a printed keyword case-insensitively (uppercase query)")
	}

	e.AddContinuous(ContinuousEffect{Source: id, Timestamp: 1, Layer: LAbilities,
		Affects: "Card.Self", AddKeywords: []string{"flying"}, UntilEOT: true})
	if !e.HasKeyword(id, "Flying") {
		t.Error("HasKeyword should match a granted keyword case-insensitively even when the grant itself is lowercase")
	}
}
