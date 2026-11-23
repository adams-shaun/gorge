package oracle

import "testing"

// CR 609.4b with CR 702.51a: a convoke cast through Stolen Strategy's
// any-colour may-play grant (MayPlayIgnoreColor) over Conclave's Blessing
// {3}{W}. Three green creatures convoke the generic and the floating {B} (a
// Locus Cobra landfall) pays {W}: gorge's offer gate prices the pool under the
// rider and offers the cast, while the kernel's gorgeOffersMay read the pool
// without it and left the cast out (decision 537: options options priority
// decision for p1).
func TestMBCConclaveSBlessingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "may_play_any_color_convoke_pool_pays")
}
