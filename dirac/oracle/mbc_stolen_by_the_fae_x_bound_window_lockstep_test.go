package oracle

import "testing"

// CR 601.2b, 601.2c and 601.2h: Stolen by the Fae's {X}{U}{U} target is a
// creature with mana value X, so one announced X must both leave a legal
// target and be payable. On this board the only creature has mana value 3,
// which prices the cast at five mana, while the sources make four: Wooded
// Bastion's "{G/W}, {T}: Add {G}{G}" spends the Plains's {W} for its fee
// instead of adding to it. gorge's potential bound counted the same filter
// land's other ability beside that production and offered the cast (decision
// 252 of 452; tail-15-c001 pair 5, seed 281703), so the target ask had no
// affordable announcement; the cast window's source-exclusive reach refuses
// it. The sibling recording hybrid_filter_xbound_never_funded pins the other
// half of the sweep (the kernel offering an X the planner cannot fund).
func TestMBCStolenByTheFaeXBoundWindowReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "stolen_by_the_fae_x_bound")
}
