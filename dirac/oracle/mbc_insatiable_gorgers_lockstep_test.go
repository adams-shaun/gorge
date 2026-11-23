package oracle

import "testing"

// Insatiable Gorgers' madness trigger (CR 702.35b): the owner is offered
// "yes" (cast for its madness cost) only when that cost can be paid. Pair 4
// of tail-61-c000 (autopay, seed 32382) parted at decision 593: gorge offered
// only "no", the kernel offered "yes" and "no".
func TestMBCInsatiableGorgersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_offer_unpayable_cost")
}
