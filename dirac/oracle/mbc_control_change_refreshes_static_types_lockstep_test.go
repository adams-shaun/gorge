package oracle

import "testing"

// CR 613.1c with CR 611.3f: a layer-4 override static's Affected$ set is
// evaluated continuously, so when Lay Claim ("You control enchanted
// permanent") moves River Bear to Ashaya, Soul of the Wild's controller,
// River Bear is a Forest land from that moment on. The kernel settles the
// control change in the state-based control pass (ctrlSBA) but recomputed the
// override-derived types only at the next step or resolution instruction, so
// the priority ask right after the steal projected River Bear still
// Creature-only (dr-mbc-baaa pair 137, decision 291).
func TestMBCRiverBearReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "control_change_refreshes_static_types")
}
