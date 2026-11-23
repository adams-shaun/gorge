package oracle

import "testing"

// CR 119.4 with CR 605.3b: Manabarbs deals 1 damage to a player for each land
// they tap for mana, so every land of p0 is a Taps-deferred source. At 3 life
// with five untapped lands, gorge offers neither Kobold Drill Sergeant ({1}{R})
// nor Crackle with Power (X = 0, {R}{R}) while the kernel offers both
// (tail-34-c000 pair 12, decision 293).
func TestMBCCrackleWithPowerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "manabarbs_deferred_lands_cast_offer")
}
