package oracle

import "testing"

// CR 700.2a with CR 601.2c: a mode with no legal target can't be chosen, so a
// charm with no choosable mode can't be cast. Soul Transfer ({1}{B}{B},
// exile target creature or planeswalker, or return one from your graveyard)
// sits in hand with {B}{B}{B} floating; the only creature is the caster's
// Priest of Gix, and the opponent's Dense Foliage says creatures can't be the
// targets of spells, while neither graveyard holds a creature or planeswalker
// card. gorge once classified the exile mode (a DB link) as no spell while the
// card was still in hand, so the restriction failed open and it offered the
// cast that dirac rightly withholds (decision 277; kfuzz -synth tail-39,
// focus Soul Transfer + Dense Foliage, pair 26, autopay, mulligans 0).
func TestMBCSoulTransferReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "charm_mode_spell_cant_target")
}
