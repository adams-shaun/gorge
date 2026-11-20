package oracle

import "testing"

// Cybermen Squadron ({7} artifact creature) is offered as a cast by gorge but
// not by dirac at the same priority decision (tail-55 pair 4, decision 246).
func TestMBCCybermenSquadronReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cybermen_squadron_cast_offer")
}
