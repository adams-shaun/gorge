package oracle

import "testing"

// CR 707.9b: Dino DNA's copy is "except it's a 6/6 green Dinosaur creature
// with trample", a type-line exception that replaces the copied card's card
// types. The token copy of Watchful Automaton (an artifact creature) is a
// Creature only; the kernel kept the Artifact bit and projected types 6
// against gorge's 2 (decision 963, tail-28 focus game).
func TestMBCFangbladeBrigandReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copy_exception_replaces_card_types")
}
