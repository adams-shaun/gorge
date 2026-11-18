package oracle

import "testing"

// Brainstealer Dragon's end-step exile carries "you may cast the card ... and
// spend mana as though it were mana of any colour to cast it" (MayPlayIgnoreColor$).
// At p0's priority with the permission live, casting the exiled Underworld
// Breach ({1}{R}) with only green and black lands untapped is a legal option:
// gorge's potential planner prices the cast's coloured pips generic under the
// rider (pay.PlanManaUnderRider). The kernel's grant walk re-priced the cast
// strictly by colour and grantCap's grant search, finding no red source,
// dropped the offer gorge made (decision 533 of 533; dr-mbc-aabb pair 58,
// seed 1149398558717970743, surface autopay).
func TestMBCUnderworldBreachReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mayplay_any_colour_walk_ignores_rider")
}
