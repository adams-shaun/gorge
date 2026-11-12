package kgen

import "testing"

// TestEveryPlayerDrainTapRefused: Defined$ Player (every player) on DrainMana
// and TapAll is refused, because the kernel's playerRef resolves one player
// (RefEachPlayer reads 255 and the effect would silently do nothing).
func TestEveryPlayerDrainTapRefused(t *testing.T) {
	root := "SP$ Draw | Cost$ B"
	kdefWant(t, kdefRefusals(t, root, "DB$ DrainMana | Defined$ Player"), "DrainMana Defined$")
	kdefWant(t, kdefRefusals(t, root, "DB$ TapAll | ValidCards$ Land | Defined$ Player"), "TapAll Defined$")
	if got := kdefRefusals(t, root, "DB$ DrainMana | Defined$ You"); len(got) != 0 {
		t.Errorf("DrainMana Defined$ You refused: %q", got)
	}
}
