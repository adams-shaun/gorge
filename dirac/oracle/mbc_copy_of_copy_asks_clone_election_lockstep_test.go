package oracle

import "testing"

// CR 707.2 and 707.9b: Urza, Prince of Kroog copies Phyrexian Metamorph, which
// is itself a copy of an as-enters clone card. The token copy is minted on the
// copying permanent's printed card with the copy it entered as its layer-1
// basis (copyTokenBasis), so its own "enter as a copy" line is asked while it
// waits outside every zone. dirac modelled the mint but not that as-enters ask
// (StallCopyAllAsk, decision 414 of a tail-03 game).
func TestMBCTail03CopyOfCopyAsksCloneReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copy_of_copy_asks_clone_election")
}
