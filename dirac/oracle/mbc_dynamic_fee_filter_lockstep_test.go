package oracle

import "testing"

// CR 605.3a, 601.2g: Cabal Coffers ("{2}, {T}: Add {B} for each Swamp you
// control") with every land a Swamp (Realmwright's choice) pays Indebted
// Samurai's and Marked by Honor's {3}{W} only through a fee-carrying
// dynamic-amount source. gorge's cast planner prices no such source (the
// census is incomplete) and the autopay surface lists no plain cast its prefix
// search alone funds, so the kernel must not offer it either (tail-41 pair 0,
// decision 661: gorge [pass], kernel [pass, cast, cast]).
func TestMBCIndebtedSamuraiReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dynamic_fee_filter_scripted_cast_not_planned")
}
