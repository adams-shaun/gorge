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

// CR 117.1a, CR 307.1 and CR 702.103a: a bestowed cast is an Aura spell, cast
// with the stack empty like the creature card it is, so gorge's prefix search
// passes while the Contaminated Ground trigger a scripted tap queued resolves
// before the bestow cast (tail-34 pair 23). The kernel kept the trigger on the
// stack beneath the spell: castNeedsEmptyStack read only a plain cast.
func TestMBCBestowCastWaitsForPrefixTriggerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bestow_cast_waits_for_prefix_trigger")
}

// CR 117.1a and CR 702.34a: a flashback cast of a sorcery is cast with the
// stack empty, so gorge's prefix search lets the Contaminated Ground trigger
// its scripted tap queued resolve first (tail-34 pair 37). The kernel's
// castNeedsEmptyStack read only a cast with no mode bit set, so a flashback
// (or kicked) cast left the trigger beneath the spell.
func TestMBCFlashbackCastWaitsForPrefixTriggerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "flashback_cast_waits_for_prefix_trigger")
}

// CR 603.3b: a scripted tap that queues two of one controller's triggers (two
// Contaminated Grounds on one land) is gorge's trigger-order ask, answered
// before the next tap, so the pair sits beneath the later tap's trigger
// (tail-34 pair 25, decision 269). The kernel left the pair pending until the
// end of the prefix, placing the active player's later trigger first.
func TestMBCTapsDeferredPrefixOneTapTwoTriggersOrderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "taps_deferred_prefix_one_tap_two_triggers_order")
}
