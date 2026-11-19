package oracle

import "testing"

// CR 715.4 with CR 707.10f: Lithoform Engine copies the permanent spell
// Smitten Swordmaster, an adventurer card cast front face up. An adventurer
// card not on the stack as an Adventure has only its normal characteristics,
// so the copy is an ordinary copy and its token is the front face; dirac
// stalled the game at the copy (StallCopyFace)
// (tail-15-c001 pair 3, decision 416).
func TestMBCTail15AdventurePermCopyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "permanent_spell_copy_of_adventure_card")
}
