package kgen

import (
	"strings"
	"testing"
)

// TestMultiManaShapes (kmana2): a card with two or more mana abilities
// is accepted only as a set of plain fixed-colour {T} abilities (the
// kernel's folded unit) or a plain {T} ability beside one gap the kernel
// models; every other shape is refused.
func TestMultiManaShapes(t *testing.T) {
	accept := map[string]bool{"Badlands": true, "Ketria Triome": true, "Alpine Meadow": true, "Secluded Courtyard": true,
		"Hidden Grotto": true, "Lupinflower Village": true, "Three Tree City": true,
		"Ifnir Deadlands": true, // {T}, Pay 1 life beside {C} (kernel painLife)
		"Baxter Building": true, // {4},{T}: four mana in any combination (kernel filtCombo)
		"Haunted Screen":  true, // {W} or {B}, then {G}, {U} or {R} for 1 life (kernel painLife)
		"Vivid Crag":      true} // {T}, remove a charge counter: one mana of any colour (kernel counter gap)
	refuse := map[string]bool{"Nightshade Dryad": true, "Abandoned Outpost": true, "An-Havva Township": true,
		"Eye of Ramos": true, "Crystal Vein": true,
		"Cascade Bluffs": true, "Cabal Stronghold": true, "Tournament Grounds": true}
	var names []string
	for n := range accept {
		names = append(names, n)
	}
	for n := range refuse {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		multi := false
		for _, r := range c.Refusals {
			multi = multi || strings.Contains(r.Reason, "mana abilities:")
		}
		switch {
		case accept[c.Name]:
			seen++
			if len(c.Refusals) > 0 {
				t.Errorf("%s refused: %v", c.Name, c.Refusals)
			}
		case refuse[c.Name]:
			seen++
			if !multi {
				t.Errorf("%s: not refused as a multi-mana shape (%v)", c.Name, c.Refusals)
			}
		}
	}
	if seen != len(names) {
		t.Errorf("lowered %d of %d cards", seen, len(names))
	}
}
