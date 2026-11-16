package oracle

import "testing"

// Spirited Companion (alone on the blocking side of Wayward Giant, which has
// menace) is offered a block of the Giant by dirac but not by gorge at the
// blockers decision (decision 607 of dr-mbc-bbab pair 58 at land a2df03bb00).
func TestMBCSpiritedCompanionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spirited_companion_blocks_menace_giant")
}
