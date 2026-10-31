package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestReturnWithCountersEntersWithThem: a ChangeZone's WithCountersType$/
// WithCountersAmount$ ("return it to the battlefield with two +1/+1 counters
// on it") puts the counters on as part of the entry (CR 122.6, 614.1c), so
// the permanent is already a 4/3 when "whenever a creature with power 4 or
// greater enters" looks at it. Infernal Vessel dies, its own trigger returns
// it with two counters, and Garruk's Uprising draws a card. Before the fix
// the counters were a separate CounterChange after the move and the Uprising
// saw a 2/1 (dirac census pairs 1664 3312 3457 9536).
func TestReturnWithCountersEntersWithThem(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := tokenWithCountersEngine(t, reg, "Garruk's Uprising", "Infernal Vessel")
	searchMoveByName(t, e, "Garruk's Uprising", state.ZBattlefield)
	drainStackPassing(t, e, 10)
	vessel := searchMoveByName(t, e, "Infernal Vessel", state.ZBattlefield)
	drainStackPassing(t, e, 10)
	hand := len(e.G.Zone(state.ZHand, 0))

	e.emit(events.Event{Kind: events.MoveZone, Obj: vessel, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.priorityRound()
	drainStackPassing(t, e, 20)

	var back *state.Object
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Infernal Vessel" {
			back = o
		}
	}
	if back == nil {
		t.Fatal("Infernal Vessel did not return to the battlefield")
	}
	if got := back.Counter("P1P1"); got != 2 {
		t.Fatalf("returned Vessel P1P1 counters = %d, want 2 (placed once, at entry)", got)
	}
	if der := e.Derived(back.ID); der.Power != 4 || der.Toughness != 3 {
		t.Fatalf("returned Vessel derived P/T = %d/%d, want 4/3", der.Power, der.Toughness)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("hand %d -> %d: Garruk's Uprising did not see a power-4 creature enter", hand, got)
	}
	replayCheck(t, e, cfg)
}
