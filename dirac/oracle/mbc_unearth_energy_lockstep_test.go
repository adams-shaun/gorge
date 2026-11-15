package oracle

import "testing"

// CR 702.84a with CR 118.3 and CR 107.14: Salvation Colossus's Unearth costs
// eight {E}, and a player with no energy can't pay it, so a Colossus in the
// graveyard (here discarded to Mind Rot) offers no activation. The kernel's
// graveyard activation loop gated only the sacrifice / tap / life picks, so a
// pure energy cost never reached its energyPayable check and the kernel offered
// the activation at decision 1183 (options options, autopay, gorge leading).
func TestMBCSalvationColossusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unearth_energy_unpayable_from_graveyard")
}
