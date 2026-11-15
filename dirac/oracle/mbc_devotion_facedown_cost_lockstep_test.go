package oracle

import "testing"

// CR 708.2 (with CR 700.5): a face-down permanent has no mana cost, so it
// contributes nothing to devotion. Evangel of Heliod's ETB counts devotion to
// white with a manifested (Reality Shift's face-down land) Flock Impostor on
// the battlefield: the kernel read the Flock Impostor's printed {2}{W} cost
// and created one Soldier token more than gorge's two. The recording is the
// first divergence of dr-mbc-bbaa pair 28: projection NInst at decision 583,
// gorge leading, on the autopay surface.
func TestMBCEvangelOfHeliodReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_devotion_facedown_cost")
}

// CR 702.26b: a phased-out permanent is treated as though it does not exist,
// so it is not a permanent you control and contributes nothing to devotion
// (the rule's own example: "draw a card for each creature you control" with
// one of three phased out draws two). Evangel of Heliod's ETB counts devotion
// to white with Guardian of Faith's ETB having phased Linvala, the Preserver
// out: gorge counted the phased-out Linvala's {4}{W}{W} and created two
// Soldier tokens more than the kernel's four. The recording is the first
// divergence of dr-mbc-bbaa pair 29: projection NInst at decision 438, gorge
// leading, on the autopay surface.
func TestMBCPhasedOutNoDevotionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_devotion_phased_out_cost")
}
