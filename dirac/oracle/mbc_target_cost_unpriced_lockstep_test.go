package oracle

import "testing"

// CR 601.2f-h with CR 115.1: Fantastic Bounce costs {2} less only when it
// targets a tapped creature, and its caster holds two floating mana beside a
// single Phyrexian Lens ({T}, pay 1 life: one mana of any colour). An untapped
// target costs {3}{U}, which the pool plus the Lens (three mana) can't reach,
// so only the tapped creatures are targets. Gorge kept every nonland permanent
// on the menu because the Lens left the window census incomplete and the
// historical fail-open offered the unaffordable targets (decision 808 of 829,
// tail-41-c000 pair 7).
func TestMBCALuminarchAspirantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "target_cost_unpriced_source_ceiling")
}
