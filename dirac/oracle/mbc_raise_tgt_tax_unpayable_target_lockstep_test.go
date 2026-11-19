package oracle

import "testing"

// CR 601.2c with CR 601.2f: Kasmina, Enigmatic Mentor taxes {2} on a spell that
// targets a creature its controller owns. The opponent's Repulse, with the
// mana left on the board, can pay {2}{U} but not {2}{U} plus the tax, so a
// target Kasmina's controller owns is not a choice the cast can complete
// (dr-mbc-baaa pair 73, decision 533 of 533: the two engines offered a
// different set of targets for Repulse).
func TestMBCWeaponcraftEnthusiastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "raise_tgt_tax_unpayable_target")
}
