package oracle

import "testing"

// CR 305.6: a land granted a basic land type has the intrinsic "{T}: Add [c]"
// beside its own abilities, so Urborg, Tomb of Yawgmoth's Swamp grant gives
// Barbarian Ring the intrinsic {T}: Add {B} beside its printed {T}: Add {R}.
// Barbarian Ring deals 1 damage to you, a pain unit the kernel's one-ability
// pcard holds in pc.painCol for every tap of the source, and the intrinsic
// production has no pain: the two alternatives are not one pcard's shape, so
// blgPlain rejects the painland and the payment stalls (static_landtype.go:
// stall landgrant, tail-32-s000 pair 21). Barbarian Ring is refused by name,
// as Thermopod is, and the recorded game ends in the declared stall.
func TestMBCTail32ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "barbarian_ring_swamp_grant_over_a_pain_land_unmodelled")
}
