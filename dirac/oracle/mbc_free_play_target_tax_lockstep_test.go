package oracle

import "testing"

// CR 601.2c with CR 601.2f, 601.2g: Urza, Lord High Artificer's play-free
// permission casts Repulse for no mana, and the opponent's Kasmina, Enigmatic
// Mentor taxes a spell that targets her side {2}. Urza's own mana ability
// (tap an artifact: add {U}) and an Island pay that tax through the mana
// window, so gorge's target menu keeps the taxed creatures; the kernel priced
// a free cast's taxed candidates from the plain sources alone and withheld
// Ophidian and both Wizard tokens (decision 415, dr-mbc-baaa pair 60).
func TestMBCFreePlayTargetTaxReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_play_target_tax_window_pays")
}
