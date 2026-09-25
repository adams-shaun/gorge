package view

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// TestBattlefieldProjectionSkipsPhasedOutPermanents pins CR 702.25b in the
// view: a phased-out permanent is treated as though it does not exist, so it
// is absent from every projection of its zone. The phased-in control object
// next to it still appears, so the absence is the flag's doing.
func TestBattlefieldProjectionSkipsPhasedOutPermanents(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	bear, d := cards.ParseBytes("b.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	bear.Link()

	// Precondition: a plain battlefield permanent IS projected.
	seen := g.AddObject(bear, 0)
	seen.Zone = state.ZBattlefield
	gone := g.AddObject(bear, 0)
	gone.Zone = state.ZBattlefield
	gone.PhasedOut = true
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{seen.ID, gone.ID})

	v := Project(g, nil, 0, nil)
	bf := v.Players[0].Battlefield
	if len(bf) != 1 || bf[0].ID != seen.ID {
		t.Fatalf("battlefield = %+v, want only the phased-in permanent %d (phased-out %d must be absent)", bf, seen.ID, gone.ID)
	}
	// Phasing it back in makes it visible again (the flag is the only cause).
	gone.PhasedOut = false
	v = Project(g, nil, 0, nil)
	if bf := v.Players[0].Battlefield; len(bf) != 2 {
		t.Fatalf("battlefield after phase-in = %+v, want both permanents", bf)
	}
}
