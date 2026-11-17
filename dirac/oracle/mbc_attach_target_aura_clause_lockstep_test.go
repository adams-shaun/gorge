package oracle

import "testing"

// Codsworth, Handy Helper's {T} ability ("Attach target Aura or Equipment you
// control to target creature you control") chains a Pump to a
// `DB$ Attach | Object$ ParentTarget | ValidTgts$ Creature.YouCtrl`. The
// attached object is the first target, never the source, so the source
// creature is itself a legal destination (CR 115.1, 115.4: a target only has
// to fit its own clause; where the Aura or Equipment may go is judged at
// resolution, CR 701.3b). gorge dropped the source from the second slot as it
// does for Equip (CR 301.5c), so the kernel offered Codsworth and gorge did
// not (tail-68 pair 7, decision 432).
func TestMBCAttachTargetAuraClauseReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attach_parenttarget_destination_may_be_source")
}
