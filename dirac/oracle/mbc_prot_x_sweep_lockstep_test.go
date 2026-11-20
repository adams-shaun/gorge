package oracle

import (
	"testing"
)

// Dregs of Sorrow's "X target creatures" is a count bound, not a ValidTgts$
// filter one, so gorge's xBoundTargetPayable early-returns for it and its
// potential-priced route offers the cast off the potential pool alone (pool 0,
// five untapped sources, one a paid filter, CR 601.2b). The kernel's X-bound
// sweep must offer it too: the recording is the game (decision 610, pool
// tail-49) where the priority menus once parted on exactly that cast. It pins
// the offer parity as the tree stands; the dirac/mana_xboundgap.go cut-self-fee
// pricing is what funds the {4}{B} announcement there, and this recording does
// not discriminate any narrower hunk in xBoundPayable's protection arm.
func TestMBCDregsOfSorrowReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prot_x_sweep")
}
