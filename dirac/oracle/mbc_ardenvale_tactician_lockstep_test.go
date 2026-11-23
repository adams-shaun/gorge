package oracle

import "testing"

// TestMBCArdenvaleTacticianReprosLockstep replays tail-51-c000 pair 99
// (autopay, seed 959984): at decision 256 the kernel offered Ardenvale
// Tactician's Dizzying Swoop face at priority and gorge did not.
func TestMBCArdenvaleTacticianReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "ardenvale_tactician_face_offer")
}
