package oracle

import "testing"

// CR 107.1, 601.2b and gorge's own pain pricing (pay.PaymentPlanDamageRider:
// a mana ability that damages its owner as it taps is last resort): when the
// payment script the prefix search writes for an activation funds a filter's
// fee, it must try the source's pain-free mana ability before its painful
// one. Raging Ravine's animate ({2}{R}{G}) was paid by activating Scabland's
// {R}/{W} ability (1 damage to its owner) to fund Shadowblood Ridge's {1}
// fee, although the same Scabland could add {C} for free and the Ridge's own
// {B} covers the {R} pip (decision 652, tail-73 pair 1326065240514747139).
func TestMBScriptPainColourFirstReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "script_pain_colour_first")
}
