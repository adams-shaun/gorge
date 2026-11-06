package oracle

import "testing"

// Suki, Kyoshi Warrior's lowered {2}{G/W}{G/W} cast beside Jasmine Dragon Tea
// Shop: the cast window must tap the Ally-only any-colour ability for the
// hybrid face it owes, not the land's fixed {C} ability, or the cast strands
// with four green and one colourless mana floating (CR 106.6, 601.2g).
func TestStdTlaSukiHybridWindowReprosLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "suki_hybrid_ally_mana_window", "std-tla", Config{})
}
