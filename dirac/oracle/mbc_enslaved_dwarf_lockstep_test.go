package oracle

import "testing"

// CR 603.6c with CR 608.2h: Lagonna-Band Storyteller's enters trigger puts a
// target enchantment card from its controller's graveyard on top of its
// owner's library (RememberChanged$ True) and then gains life equal to the
// remembered card's mana value. The kernel's single-object ToLibrary move did
// not carry RememberChanged$ for a graveyard-origin target, so the following
// GainLife read an empty remembered set and gained 0 (decision 655 of 655 in
// the recorded game: Life[1] ref=15 kernel=12).
//
// The ticket names this test for the card its evidence loop suspected at the
// land it filed from; at this land the dealt game holds Lagonna-Band
// Storyteller instead, and that is the measured first divergence.
func TestMBCEnslavedDwarfReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lagonna_band_storyteller_remembered_life")
}
