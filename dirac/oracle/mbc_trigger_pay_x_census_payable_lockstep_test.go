package oracle

import "testing"

// CR 601.2b with CR 605.1b: the announced X of a "you may pay {X}" is payable
// (the payment's census can settle it). The copy's trigger beside Mana Flare
// announced X = 13 over a Condition-split River of Tears (planner-deferred, a
// census gap the bound's census and the payment's window both count), and the
// election read the plain window census without the gap (10 mana + pool 1) and
// answered decline only (decision 934 of the tail-41 interaction run, pair 10).
func TestMBCTrigXCensusPayableLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_pay_x_census_payable")
}
