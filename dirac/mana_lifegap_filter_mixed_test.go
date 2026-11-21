package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckLifegapFilterMixedCastOffer pins the route the cast of Skyshroud
// Vampire ({3}{B}{B}) takes on p0's board Forest, Forest, Swamp, Shadowblood
// Ridge, Treasonous Ogre (its "Pay 3 life: Add {R}" turned into a census gap
// by the {T}: Add {R} beside it, kop.GapLifeManaAb without
// kop.SoleGapLifeManaAb): the Ridge ("{1},{T}: Add {B}{R}", a filter) is p0's
// fifth mana for the cast, its {1} fee funded from a floated Forest while it
// taps (CR 601.2g-h, CR 605.3a), and the Ogre is a life-only census gap the
// mix search's otherLife step activates (mana_lifegaps.go, mana_mixgaps.go).
// The census prices the cast at 4 without the Ridge, so only the mix route
// offers it.
//
// The board without floatable mana for the fee (the Ridge and the Ogre alone)
// offers nothing, as the restricted-planner-unit board of
// CheckRestrictedLifeUnitFilterCastOffer does — the life gap beside the
// filter is the mix board's second kind there, not a funding source for the
// fee gorge's payment has not floated.
func CheckLifegapFilterMixedCastOffer(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	names := []string{"Forest", "Swamp", "Shadowblood Ridge", "Treasonous Ogre", "Skyshroud Vampire"}
	for _, n := range names {
		if _, ok := id[n]; !ok {
			t.Fatalf("precondition: the table lacks %s", n)
		}
	}
	ridge, ogre, vampire := id["Shadowblood Ridge"], id["Treasonous Ogre"], id["Skyshroud Vampire"]
	addTapBeside(tbl, ogre) // the sole life-only unit becomes the mixed card's life gap
	castOf := func(sc *scen, o Obj) bool {
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
				return true
			}
		}
		return false
	}
	cfg := LockstepConfig(SurfaceFull)
	cfg.NoElide = true
	// the classification: the Ridge a filter gap, the Ogre a payable life
	// gap, the board no filter-only one
	sc := newScenTableLib(t, tbl, id["Forest"], cfg)
	// battlefield order as gorge's search walks it: the fee funds from a
	// floated Forest (CR 601.2g-h); the census is 4, the cast 5
	sc.perm(id["Forest"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Swamp"], 0)
	r := sc.perm(ridge, 0)
	o := sc.perm(ogre, 0)
	v := sc.hand(vampire, 0)
	if !sc.k.gen.has(ridge, gfFilt) || !sc.s.scriptFilt(sc.k, 0).has(sc.slot(r)) {
		t.Fatalf("gorgeLowering=%v: want Shadowblood Ridge a live filter gap", cfg.GorgeLowering)
	}
	if !sc.s.lifeGapMask(sc.k, 0).has(sc.slot(o)) {
		t.Fatalf("gorgeLowering=%v: want Treasonous Ogre a payable life gap", cfg.GorgeLowering)
	}
	if sc.s.filtOnly(sc.k, 0) {
		t.Fatalf("gorgeLowering=%v: want the board no filter-only one beside the life gap", cfg.GorgeLowering)
	}
	if !sc.s.mixBoard(sc.k, 0) {
		t.Fatal("want the filter beside the life gap a mix board: the mix route is the cast's only route")
	}
	if !castOf(sc, v) {
		t.Fatalf("gorgeLowering=%v: the Skyshroud Vampire cast not offered: the mix route never searched the Ridge beside the life gap", cfg.GorgeLowering)
	}
	// no floatable mana: the Ridge's {1} fee unfunded
	sc2 := newScenTableLib(t, tbl, id["Forest"], cfg)
	sc2.perm(ridge, 0)
	sc2.perm(ogre, 0)
	v2 := sc2.hand(vampire, 0)
	if castOf(sc2, v2) {
		t.Fatalf("gorgeLowering=%v: the cast offered with no mana to fund the Ridge's {1} fee", cfg.GorgeLowering)
	}
}
