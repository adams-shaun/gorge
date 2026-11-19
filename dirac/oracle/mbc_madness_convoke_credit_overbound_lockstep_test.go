package oracle

import "testing"

// CR 702.51a with CR 702.35b: Markov Baron discarded as a Blood Token cost on
// a turn whose every land is tapped: its madness cast carries Convoke, so
// gorge's greedy credit commits the one untapped creature (Mad Auntie, the {B}
// pip), and with a live tap credit the planner's "insufficient" verdict is
// "unsupported cost:credit" (potential_plan.go potentialModeCredit) -- no
// proof -- so the offer stands whenever the potential bound affords the
// residual {2}: Chromatic Lantern's one mana of any colour counted once per
// colour. The kernel priced the credited cost by its exact census (the
// Lantern once), so it offered only the graveyard put (dr-mbc-baba pair 176,
// seed 918033, decision 553).
func TestMBCMadnessConvokeCreditOverboundReprosLockstep(t *testing.T) {
	t.Parallel()
	corpusDirOrSkip(t) // skip with no corpus before the recording is read
	stdReproNamed(t, "madness_cast_convoke_credit_overbound")
}

// TestMBCMadnessConvokeCreditOverboundGlob replays the pool's other recordings
// by glob, so the new recording is replayed without a Go edit.
func TestMBCMadnessConvokeCreditOverboundGlob(t *testing.T) {
	stdReprosGlobPools(t, "dr-mbc-baba")
}
