package oracle

import "testing"

// Wild Hunger in a graveyard with flashback {3}{R}: gorge and dirac must offer
// the same flashback cast at the same priority decision (tail-15-c000, pair 4,
// decision 1248).
func TestMBCWildHungerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "wild_hunger_flashback_offer")
	// The same game four decisions later (decision 1252): p1 has cast Abundant
	// Growth onto its own Plains, whose granted "{T}: Add one mana of any
	// colour" is the only {R} source, so both engines must now OFFER the
	// flashback cast -- gorge's offer walk prices it against its potential
	// bound, which lists granted mana abilities.
	stdReproNamed(t, "wild_hunger_flashback_grant")
}
