package oracle

import "testing"

// A-Hullbreaker Horror's "whenever you cast a spell" charm (choose up to one)
// with the opponent's Bloodstoke Howler on the stack: its "return target spell
// you don't control" mode has TgtZone$ Stack and no TargetType$ (the Alchemy
// script), which the kernel lowered as a target with no candidates, so it
// offered only the nonland-permanent mode where gorge offers both (CR 603.3c).
func TestMBCFalseSummoningReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hullbreaker_bounce_mode_offered")
}
