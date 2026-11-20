package oracle

import "testing"

// CR 602.2, 602.5a: a permanent's activated ability is one of the priority
// options whenever its controller can pay and activate it, whether or not the
// kernel models the effect. At decision 552 of tail-26-c000 pair 16 gorge
// offered both of Sunscape Master's tap abilities while dirac listed only the
// pump: the kernel's mana search could not fund the {U}{U} bounce, so it
// dropped the activation from the menu instead of offering the unpriced play.
// Autopay passes, so the game only ever compares the menus.
func TestMBCSunscapeMasterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unmodelled_activation_offer")
}
