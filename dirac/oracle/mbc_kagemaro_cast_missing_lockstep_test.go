package oracle

import "testing"

// TestMBCKagemaroFirstToSufferReprosLockstep: a Taps trigger whose only effect
// is a poison counter on a player the payment cannot move is a post-payment
// consequence, not interference (CR 405.6c/603.2, 605.4a), so the tapped
// source stays an ordinary unit of the payment plan. gorge's planner once
// deferred a mana source wearing Relic Putrescence ("whenever enchanted
// artifact becomes tapped, its controller gets a poison counter"), priced no
// payment over its taps, and offered no cast of the {3}{B}{B} Kagemaro, First
// to Suffer or the Stromkirk Patrol beside it, while the kernel's open walk
// listed both casts at the same priority decision (synth tail-13 pair 83,
// decision 555, class options options). The planner row is
// PaymentPlanTapPoisonTrigger (pay/planshape.go), joined with the destroy-self
// row by PaymentPlanTapConsequenceTrigger; the kernel's lowering is
// dirac/kgen/lower_trigger.go. Recorded at ffdee958b2, where the game
// diverged; the fix landed in 711e6ea61a before this ticket's round began.
func TestMBCKagemaroFirstToSufferReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kagemaro_cast_missing")
}
