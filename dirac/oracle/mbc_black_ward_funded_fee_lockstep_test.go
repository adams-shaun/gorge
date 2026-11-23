package oracle

import "testing"

// Black Ward's {W} cast from hand (pool tail-51, surface full, the recorded
// shrunk game; decision 202): the board's only untapped mana is Artifact
// Unknown Shores ("{1}, {T}: Add one mana of any color") and its {1} fee is
// funded by Treasonous Ogre's life-only activation ("Pay 3 life: Add {R}") --
// the pain unit's floating {R} pays the fee no pip of the cast needs, and the
// paid activation makes the {W} (CR 605.1a, 601.2h). gorge's payment planner
// prices the cast from its potential bound, its witness replay activating the
// Ogre as the plan's last resort, so the priority menu offers
// cast(Black Ward) onto the Ogre. The kernel's filter search once never
// activated a lifeOnlyPain unit, so the cast stayed unpriced and the menu
// dropped it: the same cause the sibling recording
// cast_script_misses_sole_life_pain_unit pins, where the pain unit funds a
// pipped pip instead of another activation's fee.
func TestMBCBlackWardReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_script_misses_sole_life_pain_unit_funded_fee")
}
