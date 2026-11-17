package oracle

import "testing"

// CR 605.1a, 601.2g: a first-and-only filter land's {1} fee can be funded by a
// granted sacrifice mana ability. In tail-08 pair 34, Rain of Filth gave p1's
// lands "Sacrifice this land: Add {B}{B}", and the six black mana paid
// Darkwater Catacombs' fee so its {U}{B} cast Wydwen, the Biting Gale
// ({2}{U}{B}); the kernel counted neither the filter as a census unit nor the
// granted sacrifice as potential mana for its fee, so it offered only pass at
// decision 450 where gorge offered the cast. The recording is on a custom
// pool: the tail-08 bucket no longer holds these decks.
func TestWydwenCastOfferReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "wydwen_cast_offer")
}
