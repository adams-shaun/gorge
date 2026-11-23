package oracle

import "testing"

// CR 305.6, 605.3a: Cabal Coffers ("{2}, {T}: Add {B} for each Swamp you
// control") with every land a Swamp (Realmwright's choice) is also given the
// intrinsic "{T}: Add {B}" a Swamp has: the costed filter is a different
// ability, so it does not stand in for it (pay.ManaAbilitiesCoverIntrinsic).
// That unit plus three Plains funds Indebted Samurai's and Marked by Honor's
// {3}{W} as a plain cast, which gorge and the kernel both list (tail-41 pair 0,
// decision 661; gorge once offered [pass] alone).
func TestMBCIndebtedSamuraiReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dynamic_fee_filter_scripted_cast_not_planned")
}
