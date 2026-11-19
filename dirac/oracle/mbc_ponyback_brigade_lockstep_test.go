package oracle

import "testing"

// Ponyback Brigade (Morph {2}{R}{W}{B}): gorge offered the turn-face-up special
// action at a priority decision where dirac offered none (decision 720 of 720,
// tail-61-c002 pair 2).
func TestMBCPonybackBrigadeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ponyback_brigade_turn_face_up_offer")
}
