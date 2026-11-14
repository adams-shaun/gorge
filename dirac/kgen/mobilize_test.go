package kgen

import "testing"

// TestMobilizeLowers: Mobilize 1 and 2 lower as the Attacks trigger's
// remembering Token plus the RememberChain$ False group DelayedTrigger whose
// latent body is one SacrificeDelayed; Mobilize 3 lowers on the many-object
// program (SacrificeDelayed with CS Many).
func TestMobilizeLowers(t *testing.T) {
	checkCarrier(t,
		[]string{"Dragonback Lancer", "Bone-Cairn Butcher", "Dalkovan Packbeasts"},
		map[string]string{
			"Avenger of the Fallen": "RememberChain$ False delayed trigger shape",
		},
		[]string{
			"DelayedTrigger",
			"SacrificeDelayed obj=",
		})
}
