package oracle

import "testing"

// CR 601.3a with CR 709.3a: a Play's ValidSA$ mana-value bound tests the spell
// that would be cast, and a split card is cast as one half, so the bound reads
// that half's mana value, not the sum of both halves the card has in the
// graveyard (CR 709.4b). Diviner of Mist's "cast an instant or sorcery spell
// from your graveyard with mana value 4 or less" after the mill four offered
// gorge Give // Take ({2}{G} half, 3) beside Moment of Truth and the two Select
// for Inspections; the kernel summed the halves to 6 and withheld it, so its
// modes decision held three picks to gorge's four (decision 574, options,
// gorge leading, autopay surface; Archmage Emeritus was only the attacker
// beside it). Forge reads the cast half's cost for this bound
// (SpellAbilityProperty "cmc"), as gorge's validSAOK reads the printed face.
func TestMBCArchmageEmeritusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "play_validsa_cmc_split_front_half")
}
