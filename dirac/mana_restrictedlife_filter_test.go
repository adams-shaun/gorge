package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckRestrictedLifeUnitFilterCastOffer pins the route the cast of
// Skyshroud Vampire ({3}{B}{B}) takes on p0's board Forest, Forest, Swamp,
// Swamp, Shadowblood Ridge, Untaidake, the Cloud Keeper: the Ridge
// ("{1},{T}: Add {B}{R}", a filter) is p0's fifth mana for the cast, its {1}
// fee funded from a floated Forest while it taps (CR 601.2g-h, CR 605.3a),
// and Untaidake ("{T}, Pay 2 life: Add {C}{C}", spend only on legendary
// spells) is a restricted planner unit the census prices per payment
// (mana_restrictedcost.go), so the board is filter-only (gapOthers empty,
// filtOnly) and the filter route offers the cast. The census prices the cast
// at 4 without the Ridge, so only the filter script offers it: the route
// moved here when censusGapMana stopped reading a restricted planner unit a
// census gap (Skyshroud Vampire's repro, tail-73 pair 27).
//
// The board without a floatable mana for the fee (the Ridge and Untaidake
// alone) offers nothing, and neither does the one whose floated mana leaves
// the cast a unit short (two Swamps: the Ridge's {B}{R} tops the pool at
// {B}{B}{B}{R}) — the filter search runs there and refuses it, as gorge's
// search does.
func CheckRestrictedLifeUnitFilterCastOffer(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	names := []string{"Forest", "Swamp", "Shadowblood Ridge", "Untaidake, the Cloud Keeper", "Skyshroud Vampire"}
	for _, n := range names {
		if _, ok := id[n]; !ok {
			t.Fatalf("precondition: the table lacks %s", n)
		}
	}
	ridge, keeper, vampire := id["Shadowblood Ridge"], id["Untaidake, the Cloud Keeper"], id["Skyshroud Vampire"]
	castOf := func(sc *scen, o Obj) bool {
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
				return true
			}
		}
		return false
	}
	// the lockstep surface the repro ran (full lowers, elision off —
	// NoElide: StepInto would answer a single-option decision in place and
	// the test would read the next seat's decision). The autopay surface
	// drops a plain cast the planner does not price, as gorge's does there
	// (script_gen.go autopayDropsPlainCast), so the script route is the full
	// surface's offer.
	cfg := LockstepConfig(SurfaceFull)
	cfg.NoElide = true
	// the classification: the Ridge a filter gap, the Keeper no census
	// gap, the board filter-only
	sc := newScenTableLib(t, tbl, id["Swamp"], cfg)
	// battlefield order as the repro game's: the fee funds from an
	// earlier-floated mana (CR 601.2g-h); the census is 4, the cast 5
	sc.perm(id["Forest"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Swamp"], 0)
	sc.perm(id["Swamp"], 0)
	r := sc.perm(ridge, 0)
	sc.perm(keeper, 0)
	v := sc.hand(vampire, 0)
	if !sc.k.gen.has(ridge, gfFilt) || !sc.s.scriptFilt(sc.k, 0).has(sc.slot(r)) {
		t.Fatalf("gorgeLowering=%v: want Shadowblood Ridge a live filter gap", cfg.GorgeLowering)
	}
	if sc.k.gen.has(keeper, gfGapOther) {
		t.Fatal("want Untaidake, the Cloud Keeper no census gap: the census prices its restricted unit per payment")
	}
	if !sc.s.filtOnly(sc.k, 0) {
		t.Fatalf("gorgeLowering=%v: want the board filter-only beside the restricted planner unit", cfg.GorgeLowering)
	}
	if !castOf(sc, v) {
		t.Fatalf("gorgeLowering=%v: the Skyshroud Vampire cast not offered: the filter script never ran beside the restricted unit", cfg.GorgeLowering)
	}
	// no floatable mana: the Ridge's {1} fee unfunded
	sc2 := newScenTableLib(t, tbl, id["Swamp"], cfg)
	sc2.perm(ridge, 0)
	sc2.perm(keeper, 0)
	v2 := sc2.hand(vampire, 0)
	if castOf(sc2, v2) {
		t.Fatalf("gorgeLowering=%v: the cast offered with no mana to fund the Ridge's {1} fee", cfg.GorgeLowering)
	}
	// two Swamps: the fee funded, the cast a unit short ({B}{B}{B}{R} of {3}{B}{B})
	sc3 := newScenTableLib(t, tbl, id["Swamp"], cfg)
	sc3.perm(ridge, 0)
	sc3.perm(keeper, 0)
	sc3.perm(id["Swamp"], 0)
	sc3.perm(id["Swamp"], 0)
	v3 := sc3.hand(vampire, 0)
	if !sc3.s.filtOnly(sc3.k, 0) {
		t.Fatalf("gorgeLowering=%v: want the two-Swamp board filter-only too", cfg.GorgeLowering)
	}
	if castOf(sc3, v3) {
		t.Fatalf("gorgeLowering=%v: the cast offered on {B}{B}{B}{R}: a unit short of {3}{B}{B}", cfg.GorgeLowering)
	}
}
