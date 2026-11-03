package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Neriv doubles damage from a creature that entered this turn. A sacrificed source is
// read as the permanent it last was (CR 608.2h), so its entry into the graveyard does
// not count, and its entry into the battlefield does.
func TestDamageSourceThatLeftReadsItsOwnEntry(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name         string
		enteredToday bool
	}{
		{"entered this turn", true},
		{"entered on an earlier turn", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := handEngine(t)
			neriv := addToBattlefield(t, e, corpusCard(t, "Neriv, Heart of the Storm"), 0)
			src := addToBattlefield(t, e, corpusCard(t, "Stadium Headliner"), 0)
			if c.enteredToday {
				e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZBattlefield, To: state.ZHand})
				e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZHand, To: state.ZBattlefield})
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: src, From: state.ZBattlefield, To: state.ZGraveyard})
			e.pending = nil
			defer e.SetDamageSource(e.SetDamageSource(src))
			r := e.G.Obj(neriv).Face().Repls[0]
			got := e.damageReplacementMatches(r, neriv, events.Event{Kind: events.Damage, Player: 1, Amount: 2}, e.controllerOf(neriv), nil, nil, chosenCapture{})
			if got != c.enteredToday {
				t.Fatalf("Neriv matches a departed source: %v, want %v", got, c.enteredToday)
			}
		})
	}
}
