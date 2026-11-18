package oracle

import "testing"

// Feldon, Ronom Excavator, Keldon Strike Team and Tuktuk the Explorer are cast
// with mana from a Lotus Blossom (CR 106.1, CR 605.3b) whose petal counters
// give X mana of one colour; gorge offered the three casts at priority and
// dirac offered only pass and the land (decision 112 of tail-74 pair 5).
func TestMBCFeldonRonomExcavatorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lotus_blossom_pays_every_cast")
}
