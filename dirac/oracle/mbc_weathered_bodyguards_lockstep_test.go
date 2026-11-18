package oracle

import "testing"

// CR 702.37c with CR 601.2: after a hard cast of Weathered Bodyguards is
// backed out, the card is back in hand and its face-down (morph) cast is
// still offered. The kernel offered castface(Weathered Bodyguards) at the
// priority decision where gorge offered only pass and Tyrant's Machine
// (tail-53-c000 pair 100, decision 290).
func TestMBCWeatheredBodyguardsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "weathered_bodyguards_morph_cast_after_failed_cast")
}
