package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEachPickPlannedTapFloatsTheDefaultType: a planned tap of an all-at-once
// {W}{U} source under a reflect carrier floats its reflected pick as the first
// produced type in W U B R G C order, the one gorge's unasked carrier
// resolution takes (engine/effects ManaReflectedCandidates), whatever colour
// the plan needs, and stalls on no ask. A cost the default pick cannot pay
// is not planned at all.
func TestEachPickPlannedTapFloatsTheDefaultType(t *testing.T) {
	sc, sl := eachReflectScen(t)
	cost := func(w, u int) kop.ManaCost {
		var c kop.ManaCost
		c.Pips[kop.W], c.Pips[kop.U] = uint8(w), uint8(u)
		return c
	}
	if pc := sc.s.manaPCGen(sc.k, sl); sc.s.srcEachPick(sc.k, sl, pc) != 1 {
		t.Fatal("precondition: the source has no reflected pick")
	}
	for _, tc := range []struct {
		name string
		c    kop.ManaCost
		w, u uint8
	}{
		{"{W}{U}: the production alone", cost(1, 1), 2, 1},
		{"{W}{W}{U}: the pick is the default W", cost(2, 1), 2, 1},
	} {
		sc, _ := eachReflectScen(t)
		sc.s.g.stalled = 0
		c := tc.c
		if !sc.s.tapForTry(sc.k, 0, &c, 0, NoObj, false) {
			t.Fatalf("%s: no plan", tc.name)
		}
		if sc.s.g.stalled != 0 {
			t.Errorf("%s: stalled %d", tc.name, sc.s.g.stalled)
		}
		// the whole tap floats, spent by nothing yet: the production and the pick W
		if pl := sc.s.pl[0].pool; pl[kop.W] != tc.w || pl[kop.U] != tc.u {
			t.Errorf("%s: pool W%d U%d, want W%d U%d", tc.name, pl[kop.W], pl[kop.U], tc.w, tc.u)
		}
	}
	// {W}{U}{U} needs the pick to be U: the tap would be asked, which stalls
	sc2, _ := eachReflectScen(t)
	sc2.s.g.stalled = 0
	c := cost(1, 2)
	sc2.s.tapForTry(sc2.k, 0, &c, 0, NoObj, false)
	if sc2.s.g.stalled != StallTapExtra {
		t.Errorf("{W}{U}{U}: stalled %d, want StallTapExtra (%d)", sc2.s.g.stalled, StallTapExtra)
	}
}
