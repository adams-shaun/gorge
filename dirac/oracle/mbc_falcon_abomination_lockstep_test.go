package oracle

import "testing"

// CR 611.2b: Sly Instigator's "target creature an opponent controls can't be
// blocked until your next turn" outlasts the turn it resolves in. The grant
// lowered as an until-end-of-turn keyword, so on the opponent's next turn
// Falcon Abomination was offered a block on Quicksilver, Pietro Maximoff that
// gorge withheld (decision 496, tail-45 pair 3).
func TestMBCFalconAbominationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unblockable_until_your_next_turn")
}
