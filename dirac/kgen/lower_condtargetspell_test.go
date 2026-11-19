package kgen

import "testing"

// The targeted-spell gate (lower_condtargetspell.go): a counter's own
// "if it's <colour>" gate lowers; a later node's read of the countered spell
// (Jace's Defeat's scry) keeps the Spell-head refusal.
func TestTargetedSpellGateLowers(t *testing.T) {
	names := []string{"Pyroblast", "Hydroblast", "Burnout", "Ertai's Trickery", "Jace's Defeat"}
	kp := kfhBuild(t, names, nil)
	for _, n := range names[:4] {
		kp.accepted(t, n)
	}
	kp.refusedWith(t, "Jace's Defeat", "a Spell head outside a ReduceCost ValidTarget$")
}
