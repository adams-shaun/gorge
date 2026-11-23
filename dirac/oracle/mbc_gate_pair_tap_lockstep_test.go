package oracle

import "testing"

// CR 605.3b with CR 106.1: Whisperer of the Wilds' two {T} abilities ({G},
// and {G}{G} while its controller has a power 4 creature) are one tap that
// makes one of them, so the source is worth two mana, never three. Beside
// nine other sources that is eleven mana, and Urza's Rage kicked
// ({2}{R} plus {8}{R}) costs twelve. The kernel's potential census recorded
// the pair's second ability as a unit of its own and offered the kicked cast
// gorge's planner never does (decision 497, tail-15 pair 9).
func TestMBCUrzaSRageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gate_pair_tap_counts_larger_alternative")
}
