package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MBC filters-10 spelling, lockstep against gorge: Needle Drop's
// ValidTgts$ Any.wasDealtDamageThisTurn -- ONE target slot that may name a
// seat dealt damage this turn as well as a creature or planeswalker dealt
// damage this turn, both arms reading the same wasDealtDamageThisTurn word
// (gorge's state.Player / state.Object flags; the kernel's genState.plDmgT
// beside dmgTurn through kop.F2WasDamaged). Lightning Bolt deals the damage
// the slot needs, so a Needle Drop cast after a Bolt this turn is offered and
// its target ask lists the damaged seat and the damaged bear. The expected
// answer at every decision is gorge's.
var kmbcFilters10Decks = [2][]cardset.Entry{
	{
		{Name: "Needle Drop", Count: 8}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Mountain", Count: 14},
		{Name: "Sulfurous Springs", Count: 6},
	},
	{
		{Name: "Needle Drop", Count: 6}, {Name: "Lightning Bolt", Count: 6},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Mountain", Count: 16},
		{Name: "Sulfurous Springs", Count: 6},
	},
}

func TestMBCFilters10Lowers(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "mbcfilters10", kmbcFilters10Decks, 400, "KMBCFILTERS10_SEEDS")
}

// TestMBCFilters10LockstepMutation proves the pool exercises both arms of the
// word: with the seat arm's damage record dropped (dirac's test-only gate
// bypass) or the flag cleared from every filter (both arms ungated), some
// seed diverges -- gorge still withholds the undamaged target the mutated
// kernel offers.
func TestMBCFilters10LockstepMutation(t *testing.T) {
	if isolate(t) { // the seat-gate bypass is a package global: its own process
		return
	}
	for _, m := range []struct {
		name string
		set  func()
		fn   func(*kop.Table)
	}{
		{"seatDamageRecord", func() { dirac.DebugTgtAnyPlDmgNoGate = true }, nil},
		{"F2WasDamaged", func() {}, func(tbl *kop.Table) {
			fs := append([]kop.Filter(nil), tbl.Filters...)
			for i := range fs {
				fs[i].Flags2 &^= kop.F2WasDamaged
			}
			tbl.Filters = fs
		}},
	} {
		t.Run(m.name, func(t *testing.T) {
			m.set()
			if m.name == "seatDamageRecord" {
				defer func() { dirac.DebugTgtAnyPlDmgNoGate = false }()
			}
			n := lockstepRun(t, "mbcfilters10", kmbcFilters10Decks, 400, false, m.fn)
			if n == 0 {
				t.Fatalf("no seed diverged with the %s read mutated", m.name)
			}
		})
	}
}
