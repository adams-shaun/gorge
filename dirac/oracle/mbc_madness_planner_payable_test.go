package oracle

import "testing"

// CR 702.35a, 106.1: the "yes" of a madness cast-or-graveyard ask is offered
// only while the payment planner can pay the cast from untapped sources, not
// on offerCastablePotential's per-colour upper bound. The recording is a
// tail-45 game where Stensia Masquerade was discarded (decision 809 of the
// game) and gorge posed a yes/no menu on an unpayable {2}{R} cast where the
// kernel offered the decline alone.
func TestMBCMadnessPlannerPayableReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_yes_needs_a_planner_plan")
}
