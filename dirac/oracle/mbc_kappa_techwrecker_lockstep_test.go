package oracle

import "testing"

// CR 400.7 with CR 603.5: Kappa Tech-Wrecker's "you may remove a deathtouch
// counter from it" trigger is still on the stack when two Ninjutsu activations
// bounce the Kappa and put it back, so the permanent now holding a deathtouch
// counter is a new object the trigger's cost cannot take a counter from. The
// gorge side read the counter off the same ObjID and offered to pay; the
// choice is decline-only (decision 707 of dr-mbc-aabb pair 194).
func TestMBCKappaTechWreckerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_subcounter_cost_source_bounced")
}
