package oracle

import (
	"testing"
)

// The X-bound offer sweep's protection arm (xBoundPayable's anyProtF arm) runs
// for every targeted {X} spell once any card in the table has a protection
// quality, and its affordability proofs (the paid-gap subsets and the count
// proof) then refuse an announcement gorge's own sweep never prices: gorge's
// xBoundTargetPayable early-returns for a target no ValidTgts$ filter bounds
// (Dregs of Sorrow's "X target" is a count bound, not a filter one), so its
// priced route offers the cast off the potential pool alone (pool 0, five
// untapped sources, one a paid filter). The kernel withheld the cast and the
// priority menus parted (decision 610 of the recording, pool tail-49).
func TestMBCDregsOfSorrowReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prot_x_sweep")
}
