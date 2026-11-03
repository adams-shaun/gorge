package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPlanUnderRiderFoldsHybrid: the mana gorge's planner funds under a
// may-play rider has no hybrid pip left (pay.PlanManaUnderRider), and the pips
// still to announce after a face is chosen are payable by any mana.
func TestPlanUnderRiderFoldsHybrid(t *testing.T) {
	var c kop.ManaCost
	c.Hyb[0], c.Hyb[1] = 1<<1|1<<4, 1<<1|1<<4
	for _, r := range []struct{ mana, color bool }{{true, false}, {false, true}} {
		got := planUnderRider(c, r.mana, r.color)
		if got.Generic != 2 || got.Hyb != ([4]kop.ColorMask{}) {
			t.Fatalf("rider %+v: generic %d hyb %v", r, got.Generic, got.Hyb)
		}
	}
	if got := planUnderRider(c, false, false); got.Hyb != c.Hyb {
		t.Fatal("no rider: the hybrid pips must stay")
	}
}
