package oracle

import "testing"

// Treasonous Ogre's "Pay 3 life: Add {R}" is a unit of gorge's payment
// planner's last-resort phase: with four lands and the Ogre untapped, Ganax,
// Astral Hunter's {4}{R} is a planned cast (the plan's fifth activation is the
// Ogre's), so the autopay surface offers it. The kernel dropped it as a plain
// cast the planner does not price, though the same board's prefix search
// finds the play (decision 284 of 284; dr-mbc-bbbb pair 170).
func TestMBCGanaxAstralHunterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "autopay_life_gap_plain_cast_priced")
}
