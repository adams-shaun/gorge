package oracle

import "testing"

// TestMBCKiriOnnaReprosLockstep replays Kiri-Onna in hand beside a Scald
// (Taps-deferred Islands), Overflowing Basin and a payer at one life: the
// plain search runs out of its 160 nodes walking the Island taps, and gorge's
// fee-order run (covered funding, the Basin's {1} fee gap, then the other
// gaps) pays {4}{U} through the Basin alone. The kernel ran that fallback only
// on a board of filter gaps, so it left the cast out (CR 601.2b, 605.3b).
func TestMBCKiriOnnaReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "taps_defer_fee_order_filter")
}
