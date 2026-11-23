package oracle

import "testing"

// CR 602.2b with CR 605.3a: Titanoth Rex's cycling ({1}{G}, from the hand) is
// offered beside two untapped Prism tokens and a tapped Diamond Kaleidoscope,
// whose "Sacrifice a Prism token: Add one mana of any color" is repeatable
// (the sacrifice is not the source), so two sacrifices pay the cost. Gorge
// offered the activation and the kernel did not (tail-12-c000 pair 93,
// decision 531).
func TestMBCTitanothRexReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cycling_pays_with_sacrificed_prism_tokens")
}
