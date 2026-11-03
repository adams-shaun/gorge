package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestStatusMarkersAreNotCounters pins CR 122.1 across the readers of "a
// counter": a regeneration shield and a deathtouch mark ride Object.Counters
// for want of a status field, and neither fires a counter trigger on arrival
// or at cleanup, satisfies HasCounters, or pays an any-kind removal. Hollowmurk
// Siege drew off a deathtouch hit, and Venus's "creature with a counter on it"
// read a creature carrying only the mark.
func TestStatusMarkersAreNotCounters(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := combatTriggerBoard(t, reg, nil, []string{
		"Name:Watcher\nTypes:Creature Bear\nPT:2/2\n" +
			"T:Mode$ CounterAdded | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ Gain | TriggerDescription$ x\n" +
			"T:Mode$ CounterRemoved | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ Gain | TriggerDescription$ x\n" +
			"T:Mode$ CounterAddedAll | Valid$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ Gain | TriggerDescription$ x\n" +
			"T:Mode$ CounterPlayerAddedAll | ValidObject$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ Gain | TriggerDescription$ x\n" +
			"SVar:Gain:DB$ GainLife | LifeAmount$ 1\nOracle:x\n",
	}, nil, nil)
	id := findBattlefield(t, e, 0, "Watcher", 0)
	o := e.G.Obj(id)
	trigs := o.Face().Triggers
	sc := e.specCtx(id, 0)

	for _, marker := range []string{"Deathtouched", "Shield"} {
		add := events.Event{Kind: events.CounterChange, Obj: id, Counter: marker, Amount: 1}
		e.emit(add)
		for _, i := range []int{0, 2, 3} {
			if e.triggerMatches(trigs[i], id, add, nil) {
				t.Errorf("%s fired on a %s mark", trigs[i].Mode, marker)
			}
		}
		if e.matchesSpec("Creature.HasCounters", id, sc) {
			t.Errorf("a %s mark alone satisfied HasCounters", marker)
		}
		if got := pay.SubCounterAvailable(e.G.Obj(id), "Any"); got != 0 {
			t.Errorf("a %s mark offers %d counters to an any-kind removal", marker, got)
		}
		if got := triggerSubCounterKinds(e.G.Obj(id), Cost{}, nil); len(got) != 0 {
			t.Errorf("a %s mark offers %v to a trigger's any-kind removal", marker, got)
		}
		remove := events.Event{Kind: events.CounterChange, Obj: id, Counter: marker, Amount: -1}
		e.emit(remove)
		if e.triggerMatches(trigs[1], id, remove, nil) {
			t.Errorf("CounterRemoved fired on a %s mark clearing", marker)
		}
	}

	// A real counter still answers every one of them.
	real := events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 1}
	e.emit(real)
	for _, i := range []int{0, 2, 3} {
		if !e.triggerMatches(trigs[i], id, real, nil) {
			t.Errorf("%s did not fire on a +1/+1 counter", trigs[i].Mode)
		}
	}
	if !e.matchesSpec("Creature.HasCounters", id, sc) {
		t.Error("a +1/+1 counter did not satisfy HasCounters")
	}
	if got := pay.SubCounterAvailable(e.G.Obj(id), "Any"); got != 1 {
		t.Errorf("a +1/+1 counter offers %d to an any-kind removal, want 1", got)
	}
}
