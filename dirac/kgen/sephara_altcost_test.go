package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSepharaAltCost: Sephara, Sky's Blade's "{W} and tap four untapped
// creatures you control with flying rather than pay this spell's mana cost"
// (Cost$ W tapXType<4/Creature.withFlying>) lowers as a StatAlternativeCost
// whose Cost carries the {W} mana and the tapped set (CostTapX, N 4, a
// flying-creature filter). The kernel's alternative cast poses the tap picks
// and the mana (cast_altfree.go altTapCost, offerAltFreeHand); the lockstep
// is dirac/oracle/sephara_altcost_lockstep_test.go.
func TestSepharaAltCost(t *testing.T) {
	all, bs := costProgCastBuild(t, []string{"Sephara, Sky's Blade"})
	if len(all) != 1 || len(bs) != 1 {
		t.Fatalf("lowered %d cards, built %d, want one of each", len(all), len(bs))
	}
	if got := costProgCastRouteReason(all[0]); got != "" {
		t.Fatalf("Sephara: refusals %q, want none", got)
	}
	var alt *kop.Ability
	for i := range bs[0].def.Abilities {
		a := &bs[0].def.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAlternativeCost {
			alt = a
			break
		}
	}
	if alt == nil {
		t.Fatal("Sephara lowered without a StatAlternativeCost ability (the alternative cast would never be offered)")
	}
	if alt.Cost.Flags != kop.CostTapX || alt.Cost.N != 4 || alt.Cost.Filter == 0 {
		t.Errorf("alternative cost %+v, want four tapped flyers (CostTapX N=4 with a filter)", alt.Cost)
	}
	if want := (kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.W: 1}}); alt.Cost.Mana != want {
		t.Errorf("alternative mana %+v, want {W}", alt.Cost.Mana)
	}
}
