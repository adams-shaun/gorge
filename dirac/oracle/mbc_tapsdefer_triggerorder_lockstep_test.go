package oracle

import "testing"

// CR 603.3b and CR 117.5: gorge's prefix search taps each Taps-deferred source
// as its own activation and drains the trigger queue into the stack before the
// next one, so two lands each under a different Contaminated Ground stack
// their "becomes tapped" triggers in tap order (battlefield order). The kernel
// tapped every gap first and only then flushed, so the active player's trigger
// went onto the stack first whatever the tap order (tail-34 pair 25).
func TestMBCTapsDeferredPrefixTriggersStackInTapOrderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "taps_deferred_prefix_triggers_stack_in_tap_order")
}
