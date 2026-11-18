package oracle

import "testing"

// A Mode$ Unattached trigger whose Equipment and its bearer leave the
// battlefield in one simultaneous sacrifice batch (God-Eternal Bontu's
// "sacrifice any number of other permanents" picking Captain's Hook beside
// the Grizzled Angler it equipped) publishes the detach ONCE: gorge's CR
// 603.10a look-back fires it at the attachment's own departure, and its
// pending arm only publishes for attachments still on the battlefield. The
// kernel's flushSac fireLeave arm fired it and removePerm's bearer-left
// detach loop fired it again, so the kernel posed a two-trigger ordering ask
// where gorge pushed its one trigger and went to priority (decision 451 of
// 451; pool tail-42 pair 58, seed 4680430306246908591).
func TestMBCIvyElementalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sacbatch_unattach_refire")
}
