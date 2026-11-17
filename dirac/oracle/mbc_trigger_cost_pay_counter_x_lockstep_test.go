package oracle

import "testing"

// CR 118.3 with CR 117.3: a player can't pay a cost they can't afford.
// p1's upkeep trigger granted by Farmstead ("you may pay {W}{W}") resolves
// with only an Icatian Store (one STORAGE counter, "{T}, remove X storage
// counters: add {W} for each counter removed") untapped, so at most {W} can
// be made and the only answer is to decline. Gorge priced the Store's
// Amount$ X as unbounded and offered "pay" beside it (decision 328 of
// tail-09-c000, pair 4).
func TestMBCTail09ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_cost_pay_counter_x_mana_bound")
}
