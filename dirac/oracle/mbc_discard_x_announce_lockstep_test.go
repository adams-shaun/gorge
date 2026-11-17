package oracle

import "testing"

// CR 601.2b-c: Restless Dreams' additional cost is "discard X cards" and it
// returns X target creature cards from the graveyard. X is announced before
// targets are chosen, so with an empty graveyard X = 0 is a legal announcement
// and the spell is castable (tail-01-c001 pair 0, decision 838). The gorge side
// read the TargetMin$ X as a mandatory target and withheld the cast.
func TestMBCRestlessDreamsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "discard_x_announce_target_min")
}
