package oracle

import "testing"

// CR 601.2c, 601.2f: an opponent's Syr Elenora, the Discerning taxes the spell
// that targets it {2}. The caster floats five mana, has an untapped Swamp and a
// Selesnya Signet ({1},{T}: add {G}{W}) and chooses the modes of an entwined
// Barbed Lightning ({2}{R} + {2}): the Swamp and the Signet's filter make the
// seven mana the taxed cast costs, so gorge keeps Syr on the target menu. dirac
// priced the candidate from the sources' plain mana alone and withheld her
// (decision 667 of tail-27 pair 2, before the menu read the window's filters).
func TestMBCSyrElenoraTheDiscerningReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "target_raise_window_filter_source")
}
