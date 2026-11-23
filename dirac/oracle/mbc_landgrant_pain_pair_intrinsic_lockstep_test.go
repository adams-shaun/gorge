package oracle

import "testing"

// CR 305.6: a land granted a basic land type has that type's intrinsic
// "{T}: Add [c]" mana ability. Gorge appends it for every granted colour no
// listed ability's Produced$ names exactly, and a painland's painful choice
// ({B} or {G}) is a combo, so a Forest granted to Llanowar Wastes adds a
// plain {G} beside the card's {C} and its painful {B} or {G} (Llanowar
// Wastes#17, Navigator's Compass#21, tail-15-c000 pair 0, decision 130). The
// kernel's painland view holds only the normal alternative's colours
// (prepPainMana: manaCol {C}, the pain pair on painCol), so blgView reads the
// granted {G} as a new production and stalls landgrant on a card state gorge
// prices exactly as before the grant.
func TestMBCTail15ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "landgrant_pain_pair_intrinsic")
}
