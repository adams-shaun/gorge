package oracle

import "testing"

// Urza, Lord High Artificer's tap-an-artifact mana beside an untapped Jeweled
// Lotus (commander-only mana) and the Construct token: gorge and dirac must
// agree whether the {1}{U} Ordeal of Thassa in hand is castable (tail-64-c000
// pair 4, decision 459).
func TestMBCOrdealOfThassaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "urza_tap_artifact_mana_beside_lotus")
}
