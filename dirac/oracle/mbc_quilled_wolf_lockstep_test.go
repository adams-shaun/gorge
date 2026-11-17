package oracle

import "testing"

const shrineTapSacDynamicRepro = "shrine_of_boundless_growth_tap_sac_dynamic_mana_unmodelled"

// CR 605.1a, 602.2b: Shrine of Boundless Growth's "{T}, Sacrifice this
// artifact: Add {C} for each charge counter on it" is a dynamic sacrifice
// source gorge's potential planner activates while paying, so gorge offers a
// six-mana activation (Quilled Wolf's {5}{G}) that four Forests, a Plains and
// the Shrine pay. The kernel's varPC variants now carry the {T}, sacrifice
// cost for an activated-ability offer (availDynSac), as they carry a bare {T}
// (tail-34 pair 7, autopay, decision 362).
func TestMBCQuilledWolfReprosLockstep(t *testing.T) {
	t.Parallel()
	if res := replayRawRepro(t, shrineTapSacDynamicRepro); res.Div != nil || res.StallUnmodelled {
		t.Fatalf("divergence at %d: %s %s %v (stall %q)", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall)
	}
}
