package oracle

import (
	"testing"
)

// CR 613.4a with CR 106.6: Veteran Adventurer's "CARDNAME is also a Cleric,
// Rogue, Warrior, and Wizard" is a characteristic-defining ability, so it
// works while the card sits in its controller's hand. The Ringhart Crest's
// "{T}: Add {G}. Spend this mana only to cast a creature spell of the chosen
// type" chose Rogue, and a creature spell of the chosen type is exactly what
// a hand Veteran Adventurer is, so the Crest's restricted {G} may fund its
// {5}{G} cast. The gorge side read the card's printed Human subtype only off
// the battlefield layer table and refused the Crest's mana for the cast,
// withholding a legal option (tail-74 pair 127, seed 9802406336505234860,
// decision 498: options options).
func TestMBCCDAAddTypeHandReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cda_addtype")
}
