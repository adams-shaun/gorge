package oracle

import "testing"

// The std-trk sweep's options divergences around phased-out permanents (pairs
// 1062, 1359, 604) first stopped on their phasing rows; those are fixed on
// land, and the games then ran into two Play CopyCard$ casts (Q, Stacking the
// Deck's granted "exile a card from your hand: create a copy of this card and
// cast it") that neither engine priced alike. Both repros are whole clean
// games.
func TestPhasedOutOptionsLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-trk", []stdRepro{
		// CR 733.1, 707.10a: the Play resolves with every land tapped, gorge's copy cast is
		// reversed before any mana window opens and the copy ceases; the kernel stalled
		{"copy_cast_reversed_without_mana_source", true, false},
	})
}
