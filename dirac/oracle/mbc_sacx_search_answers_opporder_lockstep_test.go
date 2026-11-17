package oracle

import "testing"

// CR 601.2g-h with CR 603.3b: p1's Skirk Prospector is the only Goblin its own
// "Sacrifice a Goblin: Add {R}" can sacrifice, and activating it for the
// Sabertooth Wyvern or Sigardian Savior cast makes the opponent's two Profane
// Mementos trigger at once (a creature card put into an opponent's
// graveyard). gorge's potential prefix search answers that trigger-order ask
// (the options in pose order) and prices the cast from the floated {R}, so it
// lists both casts; the kernel's search left the line dead at the ask and
// offered neither (decision 904 of pool tail-18, pair 51).
func TestMBCSabertoothWyvernReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sacx_search_answers_opporder")
}
