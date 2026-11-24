package oracle

import "testing"

// CR 616.1: two Worships on the battlefield both apply to combat damage that would
// reduce its controller's life below 1. Gorge asks the affected player to order them;
// the kernel never posed that decision (decision 692 of 692, tail-55-c000 pair 2).
func TestMBCHappyHoganBodyguardReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "worship_pair_replacement_order_skipped")
}
