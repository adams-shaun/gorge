package oracle

import "testing"

// CR 608.2h: Emergent Woodwurm backs up onto itself (7/7), attacks, and is
// bounced by Whisk Away with its attack trigger on the stack. "Where X is its
// power" reads the creature's last-known power (7), so the trigger looks at
// seven cards and offers every permanent card with mana value 7 or less. gorge
// read the card now in the library (printed 4, its counters gone) and offered
// only the top four cards' matches (Assassin Den, Krakilin and Moon Sprite
// missing; decision 786 of tail-59-c000 pair 191).
func TestMBCAssassinDenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "triggered_attacker_power_reads_departed_counters")
}
