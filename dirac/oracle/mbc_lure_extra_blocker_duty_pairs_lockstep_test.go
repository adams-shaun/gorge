package oracle

import "testing"

// CR 509.1c: a lure (Nemesis Mask on Grotag Thrasher) makes every creature able
// to block it do so, and Trueheart Duelist ("can block an additional creature")
// is on the board. A duty blocker's pair on another attacker (Sledding
// Otter-Penguin -> Hematite Golem) is in no legal declaration, so the kernel
// drops it; the Duelist's second pair stays. The gorge side's per-pick list
// once kept all 30 pairs (decision 511, tail-11 pair 111).
func TestMBCSleddingOtterPenguinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lure_extra_blocker_duty_pairs")
}
