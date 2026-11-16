package oracle

import "testing"

// CR 601.2b with CR 601.2g: an announced X is bounded by what the payment
// window can make, and that window may tap a painful source even when its
// damage is lethal (the loss is a state-based action afterwards, CR 704.5a).
// Mask of the Jadecrafter's controller, at 1 life, holds five plain lands and
// Cephalid Coliseum ("{T}: Add {U}, it deals 1 damage to you"): gorge's X ask
// offers 0..6, its cast window paying the sixth with the Coliseum. The kernel
// priced the X window through the planner's lethal guard, left the Coliseum
// out and offered 0..5. The recording is the first divergence of the
// ev-tail-70-c026 focus pool's pair 94 (decision 538, options, gorge leading,
// on the autopay surface).
func TestMBCMaskOfTheJadecrafterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_ask_lethal_pain_source")
}
