package oracle

import "testing"

// CR 107.4f, 119.4, 602.2b: at life 1 neither of Mondrak's {W/P} pips can be
// paid with life, so {1}{W/P}{W/P} takes {W} from the Plains and from Katilda
// (a Human, granted a mana ability) before she is sacrificed beside
// Songstitcher; the kernel offers that, as the rules do. The gorge harness
// priced only the {1} (potentialAbilityPlan), lowered it, and dropped the play
// as one gorge refuses (decision 803 of 803, tail-55 pair 5). The recording is
// pooled custom so tail-55's membership cannot unseat it.
func TestMBCMondrakGloryDominusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_activation_low_life_granted_mana_sacrificed")
}
