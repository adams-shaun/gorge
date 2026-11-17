package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPotentialManaSubCounterXIsBoundedByCounters: Icatian Store's
// "{T}, Remove X storage counters: Add {W} for each counter removed" makes at
// most as much mana as the source has counters, not an unbounded amount, when
// the resolution-time pay gate asks (the view's PotentialMana stays unbounded).
func TestPotentialManaSubCounterXIsBoundedByCounters(t *testing.T) {
	t.Parallel()
	const src = "Name:Test Store\nManaCost:0\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T SubCounter<X/STORAGE> | Produced$ W | Amount$ X | SpellDescription$ Add {W} for each counter removed.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	e, _, srcID := newFixtureDeck(t, 7812, src)
	e.emit(events.Event{Kind: events.MoveZone, Obj: srcID, From: e.G.Obj(srcID).Zone, To: state.ZBattlefield})
	e.pending = nil
	e.Advance()
	gate := paymentDescriptor{Class: pay.PurposeOther, SubCounterBound: true}
	if got := e.potentialManaFor(0, &gate)[state.MW]; got != 0 {
		t.Fatalf("no counters: potential W = %d, want 0", got)
	}
	if got := e.PotentialMana(0)[state.MW]; got != potentialUnbounded {
		t.Fatalf("precondition: the view's PotentialMana keeps the unbounded read, got %d", got)
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: srcID, Counter: "STORAGE", Amount: 2})
	if got := e.potentialManaFor(0, &gate)[state.MW]; got != 2 {
		t.Fatalf("two counters: potential W = %d, want 2", got)
	}
}
