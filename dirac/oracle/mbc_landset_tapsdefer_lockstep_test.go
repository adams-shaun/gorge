package oracle

import "testing"

// CR 305.7 and CR 603.2: a Swamp under Contaminated Ground still taps for {B}
// (the setter keeps the intrinsic ability of the land's basic type), but its
// tap fires a "becomes tapped" watcher whose life-loss body gorge's payment
// planner defers (pay.PaymentPlanAbilityTier), so the planner's census never
// lists that Swamp. With the other Swamp the only planned source, a {2}
// Cogworker's Puzzleknot is not one of gorge's autopay casts (its potential
// play needs the scripted tap first), and the kernel must not offer it either.
// The kernel once kept the enchanted Swamp an ordinary unit (decision 74 of
// the recording, tail-55 pair 44).
func TestMBCCogworkerSPuzzleknotReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "landset_taps_deferred_cast")
}
