package oracle

import "testing"

// CR 601.2b, 601.2h: Knollspine Invocation's "{X}, Discard a card with mana
// value X" announces X before the discard is chosen, so an X no card in hand
// can pay is never offered. The kernel offered every mana-payable X while
// gorge withheld the values the hand cannot discard at, and the X = 1 the
// hand (Mobilized District 0, everything else 2 or more) could not pay left
// the activation on the kernel's stack and aborted on gorge's (tail-53 pair 0,
// projection NStack).
func TestMBCKnollspineXAnnounceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "discard_cost_x_announce_unpayable")
}
