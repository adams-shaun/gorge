package kgen

import "testing"

// The T17b static-mode ports: each mode's census example cards lower.
// These are the sole-blocker carriers of the four classes — a card listed
// here unlocks exactly when the named mode lowers beside everything else it
// prints, so the precondition (every listed card refuses only at its mode
// today) is what the RED run pins.
//
// TestStatModePortsLower: MustBlock, UnspentMana, CanBlockIfShadow and
// IgnoreLegendRule lower through the shared static machinery — the mode body
// consumes the mode's params, readCond reads the shared gates, and the build
// emits the manifest kind. gorge registers all four (engine/rules/statics.go,
// block_restrict.go, statics_echo.go, sba.go's IgnoreLegendRule walk).
func TestStatModePortsLower(t *testing.T) {
	for _, c := range []struct {
		mode  string
		names []string
	}{
		{"MustBlock", []string{"Iron Golem", "Razorgrass Screen", "Relentless Raptor", "Spirespine", "Watchdog", "Grand Melee"}},
		{"UnspentMana", []string{"Ashling, Flame Dancer", "Electro, Assaulting Battery", "Fangorn, Tree Shepherd", "Upwelling"}},
		{"CanBlockIfShadow", []string{"Aether Web", "Aetherflame Wall", "Heartwood Dryad", "Wall of Diffusion"}},
		{"IgnoreLegendRule", []string{"Brothers Yamazaki", "Cadric, Soul Kindler", "Council of Reeds", "Mirror Gallery", "Try-My-Deck Elemental"}},
	} {
		t.Run(c.mode, func(t *testing.T) {
			for _, c := range lowerNamed(t, c.names) {
				for _, r := range c.Refusals {
					t.Errorf("%s: refused: %s", c.Name, r.Reason)
				}
			}
		})
	}
}
