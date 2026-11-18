package oracle

import "testing"

// CR 601.2c with CR 701.23a: Life's Finale's "search target opponent's
// library" target is chosen at cast time, so the ChangeZone sub-ability's
// search must read the recorded pre-ask instead of Ctx.Targets (which holds
// the root spell's list, empty here). gorge searched its own library and
// offered the caster's creatures; the kernel offered the targeted opponent's
// (decision 298 of pool tail-04, pair 4).
func TestMBCAlleyStranglerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "search_target_preask_owner")
}
