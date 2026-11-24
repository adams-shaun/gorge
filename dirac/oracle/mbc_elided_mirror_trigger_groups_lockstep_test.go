package oracle

import "testing"

// The elided-mana mirror hands the kernel the taps of a recorded span in one
// batch, but gorge's engine drains its trigger queue per plan activation: the
// Taps triggers (Scald) one pre-push activation's tap queued are on the stack
// before the next source is tapped, so two Scalds of different controllers
// land [p0,p1,p0,p1] where the kernel's APNAP placement of the whole batch
// gave [p0,p0,p1,p1] (dirac/mana_elided.go coldElideFifo, placeTriggers).
// The cost payment's own taps (after the play's push) are not drained: their
// triggers wait for the play's APNAP placement, so marking them as groups
// asked a trigger order the active player's own cast trigger comes before.
// The third recording floats mana that also funds another kernel-line cast the
// engine withholds. The fourth stacks two Scalds of different controllers one
// plan tapped as [p0,p1,p0,p1], a full game. These recordings guard the mirror's
// own placement of the plan's taps and floats; they replay clean without it too
// (the kernel's filter prefix stacks each tap's triggers itself), so the game
// the mirror cleans is pinned in TestMBCElidedMirrorBaseDivergencesReprosLockstep.
func TestMBCElidedMirrorTriggerGroupsReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"elided_mirror_taps_drain_per_activation",
		"elided_mirror_cost_taps_wait_for_apnap",
		"elided_mirror_floats_fund_another_cast",
		"elided_mirror_taps_stack_per_activation_full_game",
	} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}

// A restricted source (a Powerstone's {C}) is tapped by the mirror and floats
// restricted, and a spend of it leaves the kernel's batch: floated plain it
// left a unit in the pool gorge had spent, and left to the kernel's own
// payment its tap landed on another source. A cast the payment reverses (cost
// no longer payable) keeps the Taps triggers of the mana abilities mirrored
// after its push (CR 733.1): the kernel's reversal dropped them as the
// cast's own.
func TestMBCElidedMirrorRestrictedAndReversedReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"elided_mirror_restricted_mana_spend_leaves_batch",
		"elided_mirror_restricted_source_stays_tapped",
		"elided_mirror_reversed_cast_keeps_late_triggers",
	} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
