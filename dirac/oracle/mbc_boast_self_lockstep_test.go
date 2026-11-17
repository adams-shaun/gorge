package oracle

import "testing"

// CR 702.142a: Boast reads whether this creature attacked this turn. Fearless
// Liberator's gate marked every declared attacker and matched Card.Self against
// the ones that had since died (a card, where the Self flag was never read), so
// a teammate's dead attacker in the graveyard let the untouched Liberator offer
// its boast (decision 167; tail-08 pair 5).
func TestMBCFearlessLiberatorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "boast_self_dead_attacker")
}
