package oracle

import "testing"

// CR 605.1a: gorge's relaxed proof does not count a Simian Spirit Guide's hand
// mana for Goblin Fireleaper's {1}{R} pump beside a Golgari Signet, so gorge
// does not offer it and dirac must not either (tail-45-c000 pair 4,
// decision 413).
func TestMBCGoblinFireleaperReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hand_mana_ability_activation_offer")
}
