package oracle

import "testing"

// CR 616.1: Jaya, Venerated Firemage's plus-1 damage and Absorbing Man and
// Titania's doubling match one combat damage event, and the two do not
// commute (2(n+1) against 2n+1), so gorge asks the affected player their
// order; the kernel's damage-order walk now collects the addition beside the
// doubler and applies the picks as made (repl_pluswalk.go). The recording is
// the unmodelled replplus stall of a tail-49 focus deal (pair 55).
func TestMBCTail49ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "replplus_doubler_order")
}
