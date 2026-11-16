package oracle

import "testing"

// CR 702.26b with CR 506.3 and CR 508.1b: a phased-out permanent is treated as
// though it doesn't exist, so a planeswalker that has phased out can't be
// attacked. Kaito Shizuki phases out at the beginning of its controller's end
// step on the turn it entered and stays out through the opponent's turn, so
// that opponent's creatures may attack only the player; the kernel offers
// exactly that, while gorge's attack enumeration kept the phased-out walker as
// a defender and offered every attacker a second pair at it (decision 510 of
// 510, attackers, gorge leading, on the autopay surface).
func TestMBCBlightKeeperReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phased_out_planeswalker_not_attackable")
}
