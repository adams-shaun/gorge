package oracle

import "testing"

// Divine Smite phases out the opponent's Veteran Adventurer, whose
// characteristic-defining static makes it also a Cleric, Rogue, Warrior and
// Wizard. The two engines part on the phased-out card's projected subtypes
// (decision 434; tail-74-c000 pair 154, seed 353297, autopay).
func TestMBCVeteranAdventurerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phased_out_cda_subtypes")
}
