package oracle

import "testing"

// CR 118.3c with CR 702.32a: one tap of a fixed-choice mana source (a
// "Combo U B" dual) makes ONE mana of one of its colours, and gorge's
// potential-play planner prices a kicked cast with real plans built on that
// bound, so a kicked cast unpayable from the board's actual mana is never
// offered. Bog Badger ({2}{G}, Kicker {B}) at a priority decision whose
// board makes 3 mana (two Forests and Submerged Boneyard) offers only the
// plain cast; the kernel's potential-pool mirror once counted the dual's
// single tap once per colour and offered the kicked cast too (decision 104,
// the Yahenni's Expertise free-cast game).
func TestMBCkickedChoicePoolReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kicked_choice_pool_unaffordable")
}
