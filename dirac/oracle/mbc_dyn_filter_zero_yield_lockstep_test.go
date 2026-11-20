package oracle

import "testing"

// CR 605.1a: Cabal Coffers' "{2}, {T}: Add {B} for each Swamp you control" is a
// fee filter whose amount is dynamic. With no Swamp it adds nothing, so gorge's
// payment window (payment_window_yield.go windowSourceYields) never lists it:
// Depala's pay-{X} trigger cost, announced at 44 against the over-bound a
// dynamic source prices unbounded, taps the Plains and River of Tears, floats
// their W and B, and stops. The kernel's trigger-cost window listed the filter
// anyway (windowFiltFirstUsable), paid its {2} from that floating mana and
// tapped it for nothing, leaving Cabal Coffers tapped and the pool empty.
func TestMBCZeroYieldFilterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "zero_yield_dynamic_filter_not_listed_in_trigger_cost_window")
}
