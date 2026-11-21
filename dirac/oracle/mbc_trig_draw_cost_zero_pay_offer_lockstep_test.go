package oracle

import "testing"

// Katara, Waterbending Master's attack trigger ("you may draw a card for each
// experience counter you have. If you do, discard a card", Cost$ Draw<X/You>
// with SVar:X Count$YourCountersExperience) is a trigger's own draw cost whose
// count resolves to 0 with no experience counters. gorge offers the pay
// election anyway (cumulative.go triggeredCostPayable asks only that
// triggeredCostDrawCounts resolve and the mana half be chargeable -- a resolved
// count of 0 draws nothing and runs the body, gorge's pay arm marks paid before
// the draws); the kernel's PayDraw gate demanded amt > 0 and posed the
// decline-only ask (tail-75-c000 pair 1, decision 275, pool tail-75, surface
// autopay).
func TestMBCKataraWaterbendingMasterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trig_draw_cost_zero_pay_offer")
}
