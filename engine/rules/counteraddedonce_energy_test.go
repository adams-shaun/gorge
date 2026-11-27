// trig:CounterAddedOnce on a PLAYER counter gain — "Whenever you get one or
// more {E}" (Brotherhood Scribe, Fabrication Module, Territorial Gorger,
// Aether Revolt). The placement is a PlayerCounterChange, not a CounterChange.

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestCounterAddedOnceEnergyGain(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	// The Brotherhood Scribe / Aether Revolt line shape, inline.
	youEnergy := func(name string) string {
		return "Name:" + name + "\nTypes:Creature\nPT:1/1\n" +
			"T:Mode$ CounterAddedOnce | ValidPlayer$ You | TriggerZones$ Battlefield | CounterType$ ENERGY | Execute$ Pump\n" +
			"SVar:Pump:DB$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n"
	}
	// The object recipient shape (Simic Ascendancy): never a player.
	objEnergy := "Name:Object watcher\nTypes:Creature\nPT:1/1\n" +
		"T:Mode$ CounterAddedOnce | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | CounterType$ ENERGY | Execute$ Pump\n" +
		"SVar:Pump:DB$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n"
	scribe0 := onBoardCard(t, e, 0, card(t, youEnergy("Scribe zero")))
	scribe1 := onBoardCard(t, e, 1, card(t, youEnergy("Scribe one")))
	obj0 := onBoardCard(t, e, 0, card(t, objEnergy))
	if e.G.Obj(scribe0).Zone != state.ZBattlefield || e.G.Obj(scribe1).Zone != state.ZBattlefield ||
		e.G.Obj(obj0).Zone != state.ZBattlefield || len(e.G.Obj(scribe0).Face().Triggers) != 1 {
		t.Fatal("precondition: the three watchers must be on the battlefield with one parsed trigger")
	}
	fired := func() map[state.ObjID]int32 {
		m := map[state.ObjID]int32{}
		for _, pt := range e.pendingTriggers {
			m[pt.Source]++
		}
		return m
	}

	// An energy gain by seat 0 queues seat 0's watcher once (a batch of 3 is
	// one occurrence), not seat 1's, and not the object-recipient watcher.
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "ENERGY", Amount: 3})
	if got := fired(); len(got) != 1 || got[scribe0] != 1 {
		t.Fatalf("energy gain queued %v, want exactly one trigger from seat 0's watcher %d", got, scribe0)
	}

	// A poison gain is another kind: nothing.
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "POISON", Amount: 1})
	if got := fired(); len(got) != 0 {
		t.Fatalf("poison gain queued %v, want none", got)
	}

	// An energy payment (negative Amount) is not a gain.
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "ENERGY", Amount: -1})
	if got := fired(); len(got) != 0 {
		t.Fatalf("energy loss queued %v, want none", got)
	}

	// An object CounterChange of the same kind keeps its object reading: the
	// ValidCard$ watcher fires, and the You-player lines follow the permanent's
	// controller exactly as before.
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.CounterChange, Obj: obj0, Counter: "ENERGY", Amount: 1})
	got := fired()
	if got[obj0] != 1 || got[scribe0] != 1 || got[scribe1] != 0 {
		t.Fatalf("object ENERGY put queued %v, want the object watcher and seat 0's You watcher once each", got)
	}
}
