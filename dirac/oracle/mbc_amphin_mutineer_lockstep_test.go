package oracle

import "testing"

// Cryptic Spires ("Produced$ Combo ColorID") adds no mana in gorge
// (cards.ProducedCounts claims nothing for the word), so a player holding it
// among six lands has five mana to spend. The kernel lowered the word as a
// colour list and offered the {5}{B} Drana's Silencer anyway (decision 254;
// tail-00 pair 1, the game the evidence loop filed under Amphin Mutineer).
func TestMBCAmphinMutineerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "circled_colour_land_adds_no_mana")
}
