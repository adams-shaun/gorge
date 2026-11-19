package oracle

import "testing"

// CR 508.4: Adeline, Resplendent Cathar's attack trigger makes a Human token
// for each opponent that "enters attacking that player or a planeswalker they
// control", and its controller chooses which. With the opponent controlling a
// planeswalker gorge asks the pick (token.go tokenAttackingRider); the kernel
// stalled on it (StallDefenderPW; dr-mbc-abba pair 40).
func TestMBCDrMbcAbbaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "adeline_token_attacks_planeswalker_choice")
}
