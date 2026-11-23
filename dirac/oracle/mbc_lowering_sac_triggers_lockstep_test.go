package oracle

import "testing"

// CR 603.3b, 605.3b: Kinetic Ooze (X=7) is paid by sacrificing two Treasures
// beside Senator Peacock, so each sacrifice triggers "target creature can't be
// blocked". Gorge's lowering activates each source at priority and the
// targeted trigger is put on the stack and resolved before the next source
// is tapped, so the two triggers are never ordered against each other; the
// kernel left both pending and posed a trigger-order choice above the cast
// (decision 432 of 432; tail-10-c000 pair 77).
func TestMBCKineticOozeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_lowering_resolves_targeted_sac_triggers_in_turn")
}
