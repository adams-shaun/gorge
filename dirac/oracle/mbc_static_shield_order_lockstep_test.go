package oracle

import "testing"

// CR 616.1 with CR 615.7: two Spheres of Grace (each prevents 2 of a black
// source's damage to its controller) match one combat hit, so gorge asks the
// affected player which applies first. The kernel stalled on that ask (replplus);
// the shields commute, so it poses the same ask and applies both.
func TestMBCTail34ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "combat_static_shield_order")
	// CR 707.10f with CR 707.10g: Archmage of Echoes copies the permanent
	// spell Umara Wizard, a modal double-faced card cast front face up; the
	// copy's token is the front face. The kernel stalled at the copy
	// (copyface); tail-34-c000 pair 22.
	stdReproNamed(t, "permanent_spell_copy_of_modal_card")
}
