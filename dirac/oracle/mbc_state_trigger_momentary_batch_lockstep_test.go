package oracle

import "testing"

// CR 603.8: a state trigger fires the moment its condition matches, even when
// the match is momentary. Last Laugh's "when there are no creatures on the
// battlefield, sacrifice Last Laugh" fires inside the state-based pass that
// kills the last two creatures, in the window before the batch close returns
// the creature one of them had exiled (Fairgrounds Warden's until-host-leaves
// exile holding Avalanche Caller). The kernel's state-trigger recheck ran only
// after the whole pass (sbaClean), by which time the condition was false again,
// so it offered two trigger_order options where gorge offered three.
func TestMBCLastLaughReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "state_trigger_momentary_batch")
}
