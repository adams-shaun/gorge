package oracle

import "testing"

// A delayed trigger's sacrifice (an evoke's "sacrifice it" line) whose move
// two Rest in Peace compete for parks for the CR 616.1 order ask: gorge leaves
// the creature where it is and asks its controller which applies, and the
// sacrifice happens with the move, after the answer. The table holds an
// until-leaves program (Sheltered by Ghosts), but nothing is held, so the
// parked move cannot mis-order a return. Before the park the sacrifice
// happened at once and the kernel stalled at replleave.
func TestDelayedSacrificeParksOrderAskLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "delayed_sacrifice_parks_order_ask")
}
