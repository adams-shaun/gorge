package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestCastInfoModeFlagsKeepX: OptionalCostPaid, OffspringPaid and
// TeamworkPaid are cast MODE flags riding the main pay-time CastInfo, whose
// Amount is the cast's X (CR 107.3: X is chosen on casting). The fold sets
// the bool AND keeps the X -- Celestial Reunion cast with its optional cost
// paid searches for a card with mana value X or less -- and a trailing count
// event that carries the accumulated mode flag routes by its own count flag.
func TestCastInfoModeFlagsKeepX(t *testing.T) {
	for _, flag := range []uint64{state.FlagOptionalCostPaid, state.FlagOffspringPaid, state.FlagTeamworkPaid} {
		g, id := gameWithOneCard(t)
		Apply(g, Event{Kind: CastInfo, Obj: id, Amount: 2, Counter: FlagsString(flag)})
		o := g.Obj(id)
		if o.X != 2 {
			t.Errorf("flag %v: X = %d after the main CastInfo, want 2", flag, o.X)
		}
		Apply(g, Event{Kind: CastInfo, Obj: id, Amount: 5, Counter: FlagsString(flag | state.FlagManaSpent)})
		if o.X != 2 || o.ManaSpent != 5 {
			t.Errorf("flag %v: trailing mana-spent event gave X=%d ManaSpent=%d, want 2 and 5", flag, o.X, o.ManaSpent)
		}
	}
	g, id := gameWithOneCard(t)
	Apply(g, Event{Kind: CastInfo, Obj: id, Amount: 2, Counter: FlagsString(state.FlagOptionalCostPaid)})
	if !g.Obj(id).OptionalCostPaid {
		t.Error("the OptionalCostPaid bool was not folded")
	}
}

// TestCastInfoManaExpendStoresNothing: the expend wake-up's Amount is the
// trigger matcher's, read from the event. It carries the flags accumulated by
// earlier CastInfo events but must not write any field those events set.
func TestCastInfoManaExpendStoresNothing(t *testing.T) {
	for _, flag := range []uint64{0, state.FlagReplicated, state.FlagConverged, state.FlagManaSpent} {
		g, id := gameWithOneCard(t)
		o := g.Obj(id)
		o.X, o.ReplicateTimes, o.ConvergeColours, o.ManaSpent = 1, 2, 3, 4
		Apply(g, Event{Kind: CastInfo, Obj: id, Amount: 9, Counter: FlagsString(flag | state.FlagManaExpendCast)})
		if o.X != 1 || o.ReplicateTimes != 2 || o.ConvergeColours != 3 || o.ManaSpent != 4 {
			t.Errorf("flag %v: X=%d ReplicateTimes=%d ConvergeColours=%d ManaSpent=%d, want 1 2 3 4",
				flag, o.X, o.ReplicateTimes, o.ConvergeColours, o.ManaSpent)
		}
	}
}
