package kgen

import "testing"

// TestGainControlVariantsForms: GainControl NewController$ resolved through
// definedPlayer (a triggered player), and the LoseControl$ durations that
// map onto the kernel's control-record expiries.
func TestGainControlVariantsForms(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Clambassadors", form: "NewController$ TriggeredTarget (the damaged player)", lir: "GainControl obj="},
		{card: "Contested War Zone", form: "NewController$ TriggeredSourceController", lir: "GainControl obj="},
		{card: "Drooling Ogre", form: "NewController$ TriggeredActivator (the caster)", lir: "GainControl obj="},
		{card: "Evil's Thrall", form: "LoseControl$ EOT, then UntilTheEndOfYourNextTurn", lir: "CS=Untap|kw=Haste|dur=YourNextTurn"},
	})
}
