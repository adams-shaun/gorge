package oracle

import "testing"

// TestMBCPriceOfGloryTriggerOrderReprosLockstep: under Price of Glory a land
// tapped on the opponent's turn is a tap gorge's planner never makes
// (pay.PaymentPlanAbilityTier -> TierDeferred), so every land of the payer's is a
// census gap the prefix search taps before the activation, a Taps trigger
// each, and CR 603.3 puts those triggers on the stack ahead of the ability.
// Cliffgate ("Add {R} or one mana of the chosen color") was left out of the
// kernel's deferred gaps, so the kernel paid with it inside the activation and
// its trigger went on the stack above Lord of the Accursed's ability.
//
// The recording is the first game the pair diverged in, played to the end with
// both engines in lockstep.
func TestMBCPriceOfGloryTriggerOrderReprosLockstep(t *testing.T) {
	for _, name := range []string{
		"price_of_glory_trigger_above_activation",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name)
		})
	}
}
