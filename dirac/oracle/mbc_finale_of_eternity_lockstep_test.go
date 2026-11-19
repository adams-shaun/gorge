package oracle

import "testing"

// Finale of Eternity ({X}{B}{B}, destroy up to three target creatures with
// toughness X or less) is legal at the announced X of zero with no targets, and
// Phyrexian Altar (sacrifice a creature: add one mana of any colour, no {T}) can
// fund {B}{B} by activating twice in the same payment. Gorge offers the cast;
// the kernel's X-bound sweep priced the Altar once per "tap" and withheld it.
// The recording is on a synthetic pool, the table its own decks need: the tail
// buckets a synthetic pool deals re-cut whenever the lowered card set moves.
func TestMBCFinaleOfEternityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "finale_of_eternity_xbound")
}
