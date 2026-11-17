package oracle

import "testing"

// CR 111.7 with CR 707.10h: Spin into Myth puts the opponent's Offspring token
// copy of Coruscation Mage on top of its library, and the fateseal 2 that
// follows looks at that top pair. The token copy is in a window it has ceased
// to exist in, so it matches no ChangeValid$ filter and is not offered as a
// pick; gorge offers only the real card. The kernel offered the token too
// (decision 339 of dr-mbc-babb pair 114).
func TestMBCDrMbcBabbReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ceased_token_copy_in_dig_window")
}
