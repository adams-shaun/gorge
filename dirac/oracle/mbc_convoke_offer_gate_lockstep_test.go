package oracle

import "testing"

// A plain Convoke cast credited by untapped creatures is one of gorge's
// ordinary offers: castOfferBase prices the taps before offerCastable gates
// the rest (CR 702.51a), so the cast is listed whenever its credited cost is
// affordable, with no census gap on the board. The kernel's lowering surface
// refused the play as a planner shape:contribution and the offer vanished
// (Worldsoul Colossus at decision 787 of tail-36 pair 1).
func TestMBCConvokeOfferGateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_offer_gate")
}
