package oracle

import "testing"

// CR 115.1 with CR 701.3a: Codsworth, Handy Helper's "{T}: Attach target Aura or
// Equipment you control to target creature you control" may name Codsworth
// itself as the creature: the source is a creature its controller controls, and
// the object attached is the first target (Object$ ParentTarget), not the
// source. Gorge once dropped the source from the creature target's options as
// if Codsworth were attaching itself (decision 179; kfuzz -synth tail-68 pair 0
// over a Codsworth, Handy Helper and Dimir Locket focus).
func TestMBCCodsworthHandyHelperReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attach_parent_target_to_its_own_source")
}
