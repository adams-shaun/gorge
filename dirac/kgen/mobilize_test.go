package kgen

import "testing"

// TestMobilizeLowers: Mobilize 1 and 2 lower as the Attacks trigger's
// remembering Token plus the RememberChain$ False group DelayedTrigger whose
// latent body is one SacrificeDelayed; Mobilize 3 and X stay refused (a
// delayed program carries two objects).
func TestMobilizeLowers(t *testing.T) {
	checkCarrier(t,
		[]string{"Dragonback Lancer", "Bone-Cairn Butcher"},
		map[string]string{
			"Dalkovan Packbeasts":   "Mobilize group of 3",
			"Avenger of the Fallen": "RememberChain$ False delayed trigger shape",
		},
		[]string{
			"DelayedTrigger",
			"SacrificeDelayed obj=",
		})
}
