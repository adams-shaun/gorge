package oracle

import "testing"

// CR 702.52a: Dredge N is CardDef.Dredge, a replacement of the controller's
// draw read at every draw (kgen lower_dredge.go; gorge cardflow drawFor), not a
// battlefield characteristic, so the chars projection lists it in
// gorgeKwElsewhere like Persist and Modular. Before it did, Dakmor Salvage
// entering the battlefield was a projection error (decision 138 of tail-03
// pair 0: gorge keyword "Dredge:2" has no kernel bit). The recording is pool
// "custom" (a table built from its own decks): a tail-NN bucket moves whenever
// the lowered set grows, and the replay then fails with a card "not in the
// pool" before it plays a decision.
func TestMBCDakmorSalvageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dredge_has_no_chars_bit")
}
