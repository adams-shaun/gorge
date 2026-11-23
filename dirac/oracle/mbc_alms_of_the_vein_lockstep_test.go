package oracle

import "testing"

// CR 702.35a: when a card with madness is discarded it goes to exile and its
// madness trigger lets the owner cast it for the madness cost or put it into
// the graveyard. Alms of the Vein discarded by p1 (decision 123 of
// tail-34-c000 pair 4) was offered yes/no by dirac but only no by gorge.
//
// CR 119.4: p0 at 3 life with three untapped lands under the opponent's
// Manabarbs can pay a {2}{B} or {2}{W} cast only by tapping all three (3
// damage), and gorge's potential planner never takes a plan whose summed
// damage reaches the payer's life. Dirac's census capped each source's colour
// alone and offered Alms of the Vein, Belbe's Percher and Blink Dog
// (tail-34-c000 pair 12, decision 607).
func TestMBCAlmsOfTheVeinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "alms_of_the_vein_madness_offer")
	stdReproNamed(t, "manabarbs_plain_cast_summed_lethal_guard")
}
