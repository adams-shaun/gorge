package oracle

import "testing"

// CR 601.2b, 605.1a: beside Manabarbs every untapped land of the payer is a
// Taps-deferred gap whose tap queues a punishing trigger, so the only paying
// script for Charix, the Raging Isle taps the Islands and skips the lands
// whose taps would kill the payer before the cast (decision 333 of pool
// tail-34): gorge's search (scriptSearch.visit) branches every remaining gap
// as a node's child; the kernel's prefix search dropped the cast because its
// gap order's prefixes all over-tapped into death.
func TestMBCCharixTapsDeferGapSkipReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_script_skips_the_lethal_tapsdefer_gaps")
}
