package oracle

import "testing"

// Two Fated Firepower enchantments both replace one damage event's amount by
// a commuting addition (CR 616.1: each fire counter adds to the same total in
// every order). gorge still poses its order ask, so the kernel must pose the
// same no-op ask instead of stalling replplus, then apply both additions.
func TestReplPlusCommutingOrderLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-tla", []stdRepro{
		{"replplus_commuting_order", true, false}, // CR 616.1: two plus-N replacements are a sum no order changes
	})
}
