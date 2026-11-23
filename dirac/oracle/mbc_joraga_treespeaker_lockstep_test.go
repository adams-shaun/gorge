package oracle

import "testing"

// Joraga Treespeaker's LEVEL 5+ static sets its toughness to 4 (CR 711.2a,
// CR 613.4b); the kernel left the base toughness showing (decision 1115 of
// tail-55-c001, seed 518217).
func TestMBCJoragaTreespeakerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "joraga_treespeaker_level_toughness")
}
