package oracle

import "testing"

// Redress Fate (miracle {3}{W}) is drawn as its owner's first draw of the turn
// and revealed; as the miracle trigger resolves the owner is asked whether to
// cast it. The recording is the first divergence of the evidence loop's
// tail-41 pair 1123: gorge offered the yes where the kernel offered only no.
func TestMBCRedressFateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "miracle_cast_offer_potential_pool")
}
