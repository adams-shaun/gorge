package oracle

import "testing"

// Orim's Thunder ({2}{W}, kicker {R}) is offered as a kicked cast by both
// engines: gorge listed the kicked cast option and the kernel withheld it at
// the same priority decision. Recorded as tail-08 pair 8 (surface autopay,
// seed 17338), on a custom pool: tail-08's membership moves.
func TestMBCOrimSThunderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "orims_thunder_kicker")
}
