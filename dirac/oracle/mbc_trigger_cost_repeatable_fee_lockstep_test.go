package oracle

import "testing"

// CR 118.3, 605.3a: Energy Storm's cumulative upkeep reaches {4} beside a
// Plains, a Mountain-like Pathway and Agent of Stromgald ("{R}: Add {B}", no
// {T}, repeatable). gorge's trigger-cost gate prices the repeatable ability's
// colour unbounded, so it offers "pay"; the payer elects it, the manual window
// taps both lands and converts the red into black, the charge fails and the
// payer declines. The kernel offered decline only (decision 431), then, once
// offered, tapped nothing in the window (its potential gate lacked the same
// bound, and its fee-conversion skip applied to a window with no clamp).
func TestMBCTriggerCostRepeatableFeeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_cost_repeatable_fee_ability_potential")
}
