package oracle

import "testing"

// CR 709.4: in every zone but the stack a split card has the characteristics
// of both halves, so Reduce // Rubble in a hand is blue and red. Addle's
// "choose a color" ask (red) over a hand holding it let gorge offer Reduce
// beside the other red cards; the kernel read the front half's colour alone
// and left it out (synth tail-56 pair 894, seed 17139333880468823371).
func TestMBCReduceModesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_reduce_split_hand_colors")
}
