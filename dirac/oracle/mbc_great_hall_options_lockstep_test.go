package oracle

import "testing"

// Great Hall of Starnheim's "{W}{W}{B}, {T}, Sacrifice Great Hall of Starnheim
// and a creature you control" token ability is affordable only through
// Llanowar Envoy's "{1}{G}: Add one mana of any colour" (with Gemstone Mine
// making the other white). gorge offers the activation; the kernel's uncapped
// filter prefix search spent its whole 160-node budget on funding permutations
// and withheld it (decision 1128 of the recorded game, pool tail-28, autopay).
// The capped staged search finds the same funding line (CR 601.2g-h, 602.2b).
func TestMBCGreatHallOfStarnheimReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "great_hall_of_starnheim_options")
}
