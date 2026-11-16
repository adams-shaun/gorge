package oracle

import "testing"

// CR 107.4f, 601.2b: Captive Weird's transform ({3}{R/P}) beside Battery
// Bearer's granted mana ability is payable with four untapped lands and two
// life, so the offer stays on the menu. The kernel's printed-source census
// (ungrAvail) read the payer's life as 0, found the Phyrexian pip unpayable
// and handed the play to the grant search, which dropped it (decision 620 of
// 620; ev-tail-24 pair 272).
func TestMBCCaptiveWeirdReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_activation_beside_granted_mana")
}
