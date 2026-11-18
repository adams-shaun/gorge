package oracle

import "testing"

// CR 702.183a, 601.2f: Cloud's Limit Break's Omnislash tier adds {3}{W} to
// its printed {1}{W}; at the mode ask a tier whose total cannot be paid is
// not offered. The recording stops at the modes decision (tail-26-c003).
//
// CR 702.183a, 601.2b: Cross-Slash ({0}) needs a tapped creature, so with none
// on the board the cheapest legal mode is Blade Beam ({1}, no targets needed).
// gorge's planner prices such a cast at its printed cost alone and runs no
// prefix search through Mercadian Bazaar's storage counters, so with one
// Plains and the Bazaar untapped it offers no cast; dirac offered it because
// its free-mode test ignored the mode's targets (tail-26-c001 pair 47,
// decision 353).
func TestMBCCloudSLimitBreakReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tiered_omnislash_unaffordable_mode")
	stdReproNamed(t, "tiered_blade_beam_storage_counter_mana")
}
