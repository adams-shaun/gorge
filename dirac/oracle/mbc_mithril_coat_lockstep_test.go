package oracle

import "testing"

// CR 117.3c, 704.3: Mithril Coat's and Pariah's Shield's Equip {3} is funded
// by Cephalid Coliseum's {U} into Apprentice Wizard's "{U}, {T}: Add {C}{C}{C}",
// but Pariah's Shield on the Wizard redirects the Coliseum's damage to it, and
// the state-based check after that activation kills the 0/1 before it can be
// tapped. gorge's prefix search finds no script and leaves the equips unpriced;
// dirac funded them anyway (decision 587 of 587, tail-66 pair 6).
func TestMBCMithrilCoatReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "redirected_pain_kills_filter_gap")
}
