package oracle

import "testing"

// Wild Hunger in a graveyard with flashback {3}{R}: gorge and dirac must offer
// the same flashback cast at the same priority decision (tail-15-c000, pair 4,
// decision 1248).
func TestMBCWildHungerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "wild_hunger_flashback_offer")
}
