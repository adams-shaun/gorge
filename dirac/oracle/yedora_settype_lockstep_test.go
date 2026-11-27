package oracle

import "testing"

// TestYedoraFaceDownSetTypeLockstep plays Yedora, Grave Gardener's
// ChangeZone FaceDown$ True | FaceDownSetType$ Land & Forest return (a
// face-down basic land) over the facedownfamily pool. The returned permanent
// must be exactly a Land with the Forest subtype in the kernel too, and the
// set-type return must fire: the settype=Land & Forest counter proves the
// gorge side took the path, and the lockstep projection compares its
// derived Chars.Types against the kernel's.
func TestYedoraFaceDownSetTypeLockstep(t *testing.T) {
	div, stat := demoFacedownRun(t, demoFacedownDeck("Yedora, Grave Gardener"), 64, true)
	if div != 0 {
		t.Errorf("%d games diverged", div)
	}
	if stat["setTypeLand"] == 0 {
		t.Errorf("Yedora's FaceDownSetType$ return never fired; shapes %v", stat)
	}
	t.Logf("Yedora set-type return fired %d times; shapes %v", stat["setTypeLand"], stat)
}
