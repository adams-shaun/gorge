package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// Task movecounter1: api:MoveCounter end to end on the REAL corpus carriers.
// Before this task `DB$ MoveCounter` / `AB$ MoveCounter` was unregistered, so
// every such line was one "unimplemented API MoveCounter" Note and moved
// nothing -- and the ACTIVATED carriers (Weapon Rack, Diamond City,
// Explorer's Cache) were not even offered. These leaves drive the real
// compiled SAs: the origin's count falls, the destination's rises, and the
// CounterChange pair is on the event stream. Every fixture mutation goes
// through e.emit, so each game stays replay-verified.

// moveCounterEngine deals a corpus-only game whose seat 0 library holds the
// named fixtures, drives to seat 0's Main1, and returns the engine.
func moveCounterEngine(t *testing.T, reg *cards.Registry, fixtures ...string) *Engine {
	t.Helper()
	e, _ := proliferateEngine(t, reg, fixtures...)
	return e
}

// TestSpikeCannibalMovesAllPlusOneCountersOntoItself drives the real Spike
// Cannibal: `ValidSource$ Creature | Defined$ Self | CounterType$ P1P1 |
// CounterNum$ All` -- the sweep origin, the Define$ Self destination and the
// deterministic All amount (no ask).
func TestSpikeCannibalMovesAllPlusOneCountersOntoItself(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := moveCounterEngine(t, reg, "Spike Cannibal", "Grizzly Bears")

	bearA := putNamedOnBattlefield(t, e, "Grizzly Bears")
	bearB := putNamedOnBattlefield(t, e, "Grizzly Bears")
	e.priorityRound()
	putCountersOn(t, e, 0, "Grizzly Bears", "P1P1", 2) // lands on the first bear
	// Tag the two bears by id: putCountersOn targets the first match, so put
	// the second bear's counter by id to be unambiguous.
	e.emit(events.Event{Kind: events.CounterChange, Obj: bearB, Counter: "P1P1", Amount: 1})
	e.priorityRound()

	spike := putNamedOnBattlefield(t, e, "Spike Cannibal")
	passUntilStackEmpty(t, e, 60)

	if got := e.G.Obj(bearA).Counter("P1P1"); got != 0 {
		t.Fatalf("bear A P1P1 = %d, want 0 (swept)", got)
	}
	if got := e.G.Obj(bearB).Counter("P1P1"); got != 0 {
		t.Fatalf("bear B P1P1 = %d, want 0 (swept)", got)
	}
	// Spike entered with 1 (K:etbCounter:P1P1:1). Every creature -- Spike
	// included -- loses its counters and Spike gains their total 2 + 1 + 1 = 4,
	// so it ends at 1 - 1 + 4 = 4: the whole board's counters are on Spike.
	if got := e.G.Obj(spike).Counter("P1P1"); got != 4 {
		t.Fatalf("Spike Cannibal P1P1 = %d, want 4 (the whole board's counters, its own included)", got)
	}

	// The event stream carries the real CounterChange pair, not just the
	// post-state: a -2 on bear A and a +4 onto Spike.
	sawMinusTwo, sawPlusFour := false, false
	for _, ev := range e.L.Events {
		if ev.Kind != events.CounterChange || ev.Counter != "P1P1" {
			continue
		}
		if ev.Obj == bearA && ev.Amount == -2 {
			sawMinusTwo = true
		}
		if ev.Obj == spike && ev.Amount == 4 {
			sawPlusFour = true
		}
	}
	if !sawMinusTwo || !sawPlusFour {
		t.Fatalf("CounterChange pair missing: -2 on bear A=%v, +4 on Spike=%v", sawMinusTwo, sawPlusFour)
	}
}

// TestAetherbornMarauderMovesAnyNumberOntoItself drives the real Aetherborn
// Marauder: `ValidSource$ Permanent.YouCtrl+Other+counters_GE1_P1P1 |
// Defined$ Self | CounterType$ P1P1 | CounterNum$ Any` -- the COUNTERS_GE1
// filter sweep, the Define$ Self destination, and the real any-number ask.
func TestAetherbornMarauderMovesAnyNumberOntoItself(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := moveCounterEngine(t, reg, "Aetherborn Marauder", "Grizzly Bears")

	bear := putNamedOnBattlefield(t, e, "Grizzly Bears")
	e.priorityRound()
	putCountersOn(t, e, 0, "Grizzly Bears", "P1P1", 3)
	e.priorityRound()

	marauder := putNamedOnBattlefield(t, e, "Aetherborn Marauder")
	d := passUntilNonPriority(t, e, 60)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "move_counter" {
		t.Fatalf("decision = %+v, want the CounterNum$ Any amount ask (ResumeKind move_counter)", d)
	}
	if d.Player != 0 {
		t.Fatalf("ask player = %d, want the controller 0", d.Player)
	}
	// The offer list is 0..the origin's held count (here 3, so 0..3). Answer
	// the maximum to move everything the sweep admits.
	best := -1
	bestAmount := -1
	for i, o := range d.Options {
		if o.Amount > bestAmount {
			bestAmount = o.Amount
			best = i
		}
	}
	if best < 0 || bestAmount != 3 {
		t.Fatalf("amount options = %+v, want 0..3", d.Options)
	}
	submitChoices(t, e, d.Options[best].Index)
	passUntilStackEmpty(t, e, 60)

	if got := e.G.Obj(bear).Counter("P1P1"); got != 0 {
		t.Fatalf("bear P1P1 = %d, want 0 (all three moved)", got)
	}
	if got := e.G.Obj(marauder).Counter("P1P1"); got != 3 {
		t.Fatalf("Aetherborn Marauder P1P1 = %d, want 3", got)
	}
	sawMinus, sawPlus := false, false
	for _, ev := range e.L.Events {
		if ev.Kind != events.CounterChange || ev.Counter != "P1P1" {
			continue
		}
		if ev.Obj == bear && ev.Amount == -3 {
			sawMinus = true
		}
		if ev.Obj == marauder && ev.Amount == 3 {
			sawPlus = true
		}
	}
	if !sawMinus || !sawPlus {
		t.Fatalf("CounterChange pair missing: -3 on bear=%v, +3 on Marauder=%v", sawMinus, sawPlus)
	}
}

// TestWeaponRackAbilityIsOfferedAndMovesACounter pins the activated
// `Source$ Self | ValidTgts$ Creature` shape: registering the API is what
// makes the ability appear in the offered options at all (before this task the
// offer gate skipped it and Weapon Rack's second line was dead), and the
// chosen activation moves one counter from the Rack onto the target creature.
func TestWeaponRackAbilityIsOfferedAndMovesACounter(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := moveCounterEngine(t, reg, "Weapon Rack", "Grizzly Bears")

	rack := putNamedOnBattlefield(t, e, "Weapon Rack")
	bear := putNamedOnBattlefield(t, e, "Grizzly Bears")
	e.priorityRound()
	passUntilStackEmpty(t, e, 60)
	// Weapon Rack enters with three +1/+1 counters (K:etbCounter:P1P1:3).
	if got := e.G.Obj(rack).Counter("P1P1"); got != 3 {
		t.Fatalf("Weapon Rack P1P1 = %d, want 3 (etbCounter)", got)
	}

	// The {T} ability must be OFFERED. The priority decision's options carry
	// an "activate" kind anchored to the source; find the Rack's.
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending priority decision at Main1")
	}
	activate := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == rack {
			activate = o.Index
		}
	}
	if activate < 0 {
		t.Fatalf("Weapon Rack's MoveCounter ability is not offered: %+v", d.Options)
	}

	// Activate, then answer the target ask with the bear.
	submitChoices(t, e, activate)
	td := passUntilNonPriority(t, e, 60)
	if td == nil || td.Kind != decision.KTarget {
		t.Fatalf("decision = %+v, want the target ask for the Rack's ability", td)
	}
	target := -1
	for _, o := range td.Options {
		if o.Obj == bear {
			target = o.Index
		}
	}
	if target < 0 {
		t.Fatalf("bear not offered as a target: %+v", td.Options)
	}
	submitChoices(t, e, target)
	passUntilStackEmpty(t, e, 60)

	if got := e.G.Obj(rack).Counter("P1P1"); got != 2 {
		t.Fatalf("Weapon Rack P1P1 = %d, want 2 (one moved off)", got)
	}
	if got := e.G.Obj(bear).Counter("P1P1"); got != 1 {
		t.Fatalf("bear P1P1 = %d, want 1 (one landed)", got)
	}
	sawMinus, sawPlus := false, false
	for _, ev := range e.L.Events {
		if ev.Kind != events.CounterChange || ev.Counter != "P1P1" {
			continue
		}
		if ev.Obj == rack && ev.Amount == -1 {
			sawMinus = true
		}
		if ev.Obj == bear && ev.Amount == 1 {
			sawPlus = true
		}
	}
	if !sawMinus || !sawPlus {
		t.Fatalf("CounterChange pair missing: -1 on Rack=%v, +1 on bear=%v", sawMinus, sawPlus)
	}
}
