package oracle

import "testing"

// CR 605.1a/106.12a: p1 holds Sylvan Tutor ({G}) with Jungle Basin tapped for
// {C}{G} under Mirari's Wake (an extra mana of a type that land produced), so the
// tap's reflected extra is a pick of the produced types. gorge offers the cast
// at the decision-600 priority; the kernel's census must price the pick and offer
// it too, not stall on the tap's extra (tapextra).
func TestMBCSylvanTutorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflected_pick_cast_offer")
}
