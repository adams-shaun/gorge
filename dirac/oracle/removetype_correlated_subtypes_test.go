package oracle

import (
	"testing"
)

// A static's RemoveType$ Creature takes the subtypes correlated with the
// removed card type with it (CR 205.1a): Heliod, Sun-Crowned at devotion
// below five reads Legendary Enchantment, not Legendary Enchantment God --
// gorge chars/types.go's RemoveTypes block strips them beside the card type,
// so the kernel's type-subtype walk (ovrCreatSub) strips the God word too.
func TestRemoveTypeCorrelatedSubtypesLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "removetype_correlated_subtypes")
}
