package oracle

import "testing"

// Treasonous Ogre's "Pay 3 life: Add {R}" (CR 118.3, 602.2) has no {T}, so a
// summoning-sick or tapped Ogre still pays. With one untapped Swamp and the
// Ogre, gorge's planner prices Seal of Fire ({R}) and Goblin Bombardment
// ({1}{R}) as plain casts; the kernel offered neither.
func TestMBCGoblinBombardmentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ogre_life_mana_prices_plain_cast")
}
