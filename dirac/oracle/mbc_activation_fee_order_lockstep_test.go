package oracle

import "testing"

// CR 601.2g, 605.3b: Rampant Elephant's {G} is paid through Shimmering Grotto's
// "{1}, {T}: Add one mana of any color", itself funded by Dimir Signet's
// "{1}, {T}: Add {U}{B}". gorge's plain prefix search spends its whole
// 160-node budget below the Grotto's free {C} tap, and its fee-ordered run
// then funds both filters' fees from covered sources (Mountain, then Plains)
// before activating either. Both engines must tap the same sources.
func TestMBCRampantElephantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "activation_fee_order_run_funds_both_filter_fees")
}
