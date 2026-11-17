package oracle

import "testing"

// CR 702.94a, 733.1: Reforge the Soul's miracle trigger resolves with every
// land tapped and only Nardole, Resourceful Cyborg (no counters) untapped.
// gorge prices the cast against its potential pool, where Nardole's Amount$ X
// is open in every colour, and offers it; dirac offered only keep (decision
// 229; kfuzz tail-20 pair 8, full surface, mulligans 1, seed 3; first filed as
// tail-61 pair 3 at land 81a6ccdb4b). The recording then answers yes: gorge's
// window lists no source that yields nothing, so the cast aborts with nothing
// tapped, where dirac's window tapped Nardole. Played on its own decks'
// table: tail pool membership moves with the corpus.
func TestMBCReforgeTheSoulReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "miracle_cast_offer_reforge_the_soul")
}
