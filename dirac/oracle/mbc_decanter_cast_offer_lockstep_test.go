package oracle

import "testing"

// CR 601.2h with CR 605.3b: p1 holds Decanter of Endless Water ({3}) beside two
// untapped Islands and Hall of Heliod's Generosity ("{T}: Add {C}") enchanted
// by Blight, whose "when enchanted land becomes tapped" watcher makes gorge's
// planner defer the Hall (pay.PaymentPlanAbilityTier): its census never lists
// the land's {C}, so gorge offers only pass. The kernel modelled a deferred
// source only for a coloured production (tapsDeferColour), left a colourless
// one as an ordinary census unit and offered the cast (decision 627 of 627;
// tail-30-c011 pair 2, autopay, seed 1, land 7628ebcd21).
func TestMBCDecanterOfEndlessWaterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "decanter_priority_cast_offer")
}
