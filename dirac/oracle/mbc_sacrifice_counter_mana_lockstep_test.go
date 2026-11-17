package oracle

import "testing"

const shrineOfBoundlessGrowthRepro = "shrine_of_boundless_growth_potential_offer_unmodelled"

// CR 605.1a, 602.2b: Shrine of Boundless Growth's "{T}, Sacrifice this
// artifact: Add {C} for each charge counter on it" funds Resurrection Orb's
// Equip {4} through gorge's potential walk. The kernel now prices the dynamic
// {T}, sacrifice ability as a varPC variant for an activated-ability offer
// (avail.dynSac) while a cast offer still leaves it out, so the Equip is
// offered with the Shrine's mana (tail-34 pair 1, decision 579).
func TestMBCResurrectionOrbReprosLockstep(t *testing.T) {
	t.Parallel()
	if res := replayRawRepro(t, shrineOfBoundlessGrowthRepro); res.Div != nil || res.StallUnmodelled {
		t.Fatalf("divergence at %d: %s %s %v (stall %q)", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall)
	}
}
