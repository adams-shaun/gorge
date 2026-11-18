package kgen

import "testing"

// TestLowerHybridFilterLands: the Shadowmoor/Eventide filter lands, "{T}: Add
// {C}" beside "{W/B}, {T}: Add {W}{W}, {W}{B}, or {B}{B}", lower with no
// refusal.
func TestLowerHybridFilterLands(t *testing.T) {
	names := []string{"Fetid Heath", "Rugged Prairie", "Flooded Grove", "Cascade Bluffs", "Twilight Mire",
		"Graven Cairns", "Sunken Ruins", "Mystic Gate", "Fire-Lit Thicket", "Wooded Bastion"}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		seen++
		if len(c.Refusals) > 0 {
			t.Errorf("%s refused: %v", c.Name, c.Refusals)
		}
	}
	if seen != len(names) {
		t.Errorf("lowered %d of %d cards", seen, len(names))
	}
}
