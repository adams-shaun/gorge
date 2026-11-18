package oracle

import "testing"

// CR 707.10g: Myojin of Cryptic Dreams copies the permanent spell Ludevic's
// Test Subject, a transforming double-faced card cast front face up. The copy
// enters as a token of the same card and face, exactly as gorge's StackCopy
// mints it; dirac used to stall the game at the copy (StallCopyFace)
// (tail-29-c004 pair 31, decision 744).
//
// CR 702.51a, 702.187a: We Ride at Dawn grants Convoke to Green Goblin, Back
// for More, a Mayhem card cast from hand at its printed cost; the kernel
// stalled (stackkwgrant) on the card's Mayhem flag alone instead of on a
// mayhem cast (tail-29-c001 pair 47).
func TestMBCTail29ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "permanent_spell_copy_of_transform_card")
	stdReproNamed(t, "granted_convoke_on_mayhem_card_cast_from_hand")
}
