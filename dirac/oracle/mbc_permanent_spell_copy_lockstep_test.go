package oracle

import "testing"

// CR 707.10g: Myojin of Cryptic Dreams copies the permanent spell Ludevic's
// Test Subject, a transforming double-faced card cast front face up. The copy
// enters as a token of the same card and face, exactly as gorge's StackCopy
// mints it; dirac used to stall the game at the copy (StallCopyFace)
// (tail-29-c004 pair 31, decision 744).
func TestMBCTail29ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "permanent_spell_copy_of_transform_card")
}
