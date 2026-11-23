package oracle

import "testing"

// CR 303.4f, 702.16c: a black Aura put onto the battlefield (Nomad Mythmaker)
// can't be attached to a creature that has protection from black, here a
// Benalish Knight wearing Sword of Feast and Famine. Gorge leaves it out of
// the "enchant" election; dirac offered it because enchantHostOK read only
// printed protection, not the keyword a static grant adds.
func TestMBCBenalishKnightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "aura_enters_protection_granted")
}
