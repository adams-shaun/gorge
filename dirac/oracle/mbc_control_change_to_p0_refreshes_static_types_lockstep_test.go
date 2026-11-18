package oracle

import "testing"

// CR 613.1c with CR 611.3f, the mirror of control_change_refreshes_static_types:
// p0's Lay Claim enchants p1's Ashaya, Soul of the Wild, and the state-based
// control pass registers Ashaya's FIRST-EVER control record, moving it to p0.
// Ashaya's "nontoken creatures you control are Forest lands" is keyed on the
// controller, so its cached override-derived types must be re-derived for p0
// (Kitsa, Otterball Elite becomes a Forest) and dropped for p1 (Storm Caller
// stops being one). The gate that decided whether to refresh compared the
// post-pass controllers against a baseline filled only for slots that already
// carried a control record at entry: a first-ever record's bearer was
// zero-baselined, so a steal that ends at controller 0 looked like no move and
// the refresh was skipped (dr-mbc-baaa pair 278, decision 434).
func TestMBCControlChangeToP0ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "control_change_to_p0_refreshes_static_types")
}
