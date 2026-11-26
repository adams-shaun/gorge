package oracle

import "testing"

// Frenetic Sliver grants every Sliver (a Changeling included) "{0}: flip a
// coin ... exile this permanent and return it ... at the beginning of the next
// end step". The delayed return the granted ability registers lives on the
// GRANTOR's face (the grantee has no such SVar), so gorge must remember the
// grantor on the phase registration, as it already does for the SpellCast
// kind (CR 603.7): without it the Changeling's registration is collected at
// the next turn change and its return trigger never fires, while the kernel
// still queues it (kfuzz tail-73 pair fx-swirl2_00398_UG vs fx-swirl2_00040_UR,
// `projection NStack` ref=1 kernel=2).
func TestFreneticSliverGrantedAbilityStack(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_phase_delayed_trigger_keeps_grantor")
}
