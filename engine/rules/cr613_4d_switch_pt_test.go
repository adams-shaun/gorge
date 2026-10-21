package rules

// CR 613.4d: "Effects that switch a creature's power and toughness" apply
// after every other layer-7 effect, counters included (CR 613.4c), in
// timestamp order. The corpus spells every switch as a Pump/PumpAll grant of
// `KW$ HIDDEN CARDNAME's power and toughness are switched` (28 files: Twisted
// Image, Inside Out, Aquamoeba, Wandering Fumarole, Crag Puca, ...). Real
// corpus cards read as script text; the board is built eventlessly and funded
// through the pool.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr613CastTwistedImage casts the Twisted Image in seat 0's hand at target
// and resolves it.
func cr613CastTwistedImage(t *testing.T, e *Engine, spell, target state.ObjID) {
	t.Helper()
	e.askPriority(0)
	cr601Choose(t, e, "cast Twisted Image", func(o decision.Option) bool { return o.Kind == "cast" && o.Obj == spell })
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		cr601Choose(t, e, "target", func(o decision.Option) bool { return o.Obj == target })
	}
	resolveCast(t, e)
}

// TestCR613_4dTwistedImageSwitchesAfterCounters: Phantasmal Fiend (1/5) with
// a +1/+1 counter is 2/6; Twisted Image switches it to 6/2 (the counter
// applies before the switch).
func TestCR613_4dTwistedImageSwitchesAfterCounters(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "t/twisted_image.txt")))
	spell := e.G.Zone(state.ZHand, 0)[0]
	fiend := battlePerm(t, e, 0, corpusCardText(t, "p/phantasmal_fiend.txt"))
	e.emit(events.Event{Kind: events.CounterChange, Obj: fiend, Counter: "P1P1", Amount: 1})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 1})
	cr613CastTwistedImage(t, e, spell, fiend)
	if p, tg := e.Power(fiend), e.Toughness(fiend); p != 6 || tg != 2 {
		t.Fatalf("CR 613.4d: Phantasmal Fiend with a +1/+1 counter is %d/%d after Twisted Image, want 6/2", p, tg)
	}
}

// TestCR613_4dSwitchToZeroToughnessDies: Twisted Image on the opponent's
// Wall of Stone (0/8) makes it 8/0, and CR 704.5f puts it into the graveyard.
func TestCR613_4dSwitchToZeroToughnessDies(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "t/twisted_image.txt")))
	spell := e.G.Zone(state.ZHand, 0)[0]
	wall := battlePerm(t, e, 1, corpusCardText(t, "w/wall_of_stone.txt"))
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 1})
	cr613CastTwistedImage(t, e, spell, wall)
	if z := e.G.Obj(wall).Zone; z != state.ZGraveyard {
		t.Fatalf("CR 613.4d/704.5f: Wall of Stone switched to 8/0 is in %s (P/T %d/%d), want the graveyard", z, e.Power(wall), e.Toughness(wall))
	}
}

// TestCR613_4dTwoSwitchesCancel: two Twisted Images on Phantasmal Fiend
// (1/5) apply in timestamp order and return it to 1/5.
func TestCR613_4dTwoSwitchesCancel(t *testing.T) {
	t.Parallel()
	ti := card(t, corpusCardText(t, "t/twisted_image.txt"))
	e := handEngine(t, ti, ti)
	first, second := e.G.Zone(state.ZHand, 0)[0], e.G.Zone(state.ZHand, 0)[1]
	fiend := battlePerm(t, e, 0, corpusCardText(t, "p/phantasmal_fiend.txt"))
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 2})
	cr613CastTwistedImage(t, e, first, fiend)
	if p, tg := e.Power(fiend), e.Toughness(fiend); p != 5 || tg != 1 {
		t.Fatalf("CR 613.4d: Phantasmal Fiend is %d/%d after one Twisted Image, want 5/1", p, tg)
	}
	cr613CastTwistedImage(t, e, second, fiend)
	if p, tg := e.Power(fiend), e.Toughness(fiend); p != 1 || tg != 5 {
		t.Fatalf("CR 613.4d: Phantasmal Fiend is %d/%d after two Twisted Images, want 1/5", p, tg)
	}
}
