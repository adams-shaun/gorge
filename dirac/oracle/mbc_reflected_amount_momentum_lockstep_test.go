package oracle

import "testing"

// TestMBCMomentumReprosLockstep replays tail-55 pair 89 (the evidence loop's
// run, 293 decisions): p1 holds a Forest, a Swamp and an Incubation Druid, and
// gorge offers Momentum ({2}{G}) while the kernel once did not, because the
// Druid's reflected Amount$ mana source folded its {C} through
// canTapForManaPC, which rejects every census-gap source. The fold now reads
// the gap's own liveness (8fe74388d1, CR 605.1a, 601.2f).
func TestMBCMomentumReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflected_amount_source_colorless_fold_gap_liveness_momentum")
}
