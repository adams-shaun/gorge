package oracle

import "testing"

// CR 601.2a-h with CR 613: the payment-plan hold probe (castprobe.go
// paymentPlanHoldsOnStack) re-resolves the plan's steps with the cast's card
// on the stack, because the CR 601.2g window opens only after CR 601.2a moved
// the card out of the hand. Here that move empties the caster's hand, so
// Demon's Jester's hellbent static ("gets +2/+1 as long as you have no cards
// in hand") turns on and Alena, Kessig Trapper's {T}: "Add an amount of {R}
// equal to the greatest power among creatures you control that entered this
// turn" produces 4, not the witnessed 3: the one-step plan does not hold, and
// the plain cast is one neither side offers (the floating pool pays nothing
// and the autopay surface lists no potential plain cast).
//
// The staticEffects memo (engine_layercache.go) is stamped at events and the
// probe emits none, so the memo's recorded Condition$ Hellbent gate outcome
// (recorded at a one-card hand) served stale inside the probe: gorge resolved
// Alena at 3 R on the empty hand, priced a plan that "held", and offered the
// cast the kernel withheld (decision 720 of the tail-19-c000 focus game;
// options options, implicate Molten Echoes).
func TestMBCMoltenEchoesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "plan_hold_probe_static_gate_stale_hand")
}
