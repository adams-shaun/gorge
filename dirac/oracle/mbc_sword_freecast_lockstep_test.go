package oracle

import (
	"testing"
)

// Sword of Once and Future's "surveil 2. Then you may cast an instant or
// sorcery spell with mana value 2 or less from your graveyard without paying
// its mana cost" (CR 601.2c): the cast-free ask's candidate list is the
// graveyard's instant and sorcery cards of mana value 2 or less the controller
// owns. The first divergence (options options, decision 686 of pool tail-68's
// pair 3) is the two engines disagreeing on that list.
func TestMBCFadeIntoAntiquityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sword_freecast_graveyard")
}
