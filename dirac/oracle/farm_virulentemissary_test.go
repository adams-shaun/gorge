package oracle

import "testing"

// TestPinnedFarmVirulentEmissary plays standard-ECL games that reach Virulent
// Emissary (1/1 deathtouch; "whenever another creature you control enters, you
// gain 1 life", CR 603.2) in lockstep. The farm's ECL-s300 game xe:RAND:4:15
// parted from gorge at turn 5 with the card implicated: a projection of
// Cards[79].Chars.Keywords showed haste on the kernel side only (ref=0,
// kernel=256). The farm's tape is not kept and its decks no longer deal, so the
// pin is games recorded from the std-ecl pool that reach the card, played on
// the full surface: a kernel that grants a keyword gorge does not fails it.
func TestPinnedFarmVirulentEmissary(t *testing.T) {
	pinnedGames(t, "std-ecl", Config{}, onBattlefield("Virulent Emissary"))
}
