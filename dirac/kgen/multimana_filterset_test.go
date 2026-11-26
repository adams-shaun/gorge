package kgen

import (
	"strings"
	"testing"
)

// TestMultiManaFilterSet (T18e4): a plain {T}: Add {C} first ability beside
// one or more paid filter abilities ({1},{T}: Add <colour>, or {2},{T}:
// Add {B} or {G}) lowers: the kernel's filter walk holds one filter ability
// per colour and models a fixed-colour or two-colour-choice fee. The other
// shapes of the nine T18e cards keep their refusal, naming their actual
// reason (Glittering Stockpile's is the counter-rider-beside-another-mana-
// ability reason).
func TestMultiManaFilterSet(t *testing.T) {
	accept := map[string]bool{
		"Koskun Keep":       true,
		"Castle Sengir":     true,
		"An-Havva Township": true,
		"Aysen Abbey":       true,
		"Wizards' School":   true,
		"Murmuring Bosk":    true, // the pain-first coloured pair (kernel prepPainFirst)
	}
	refuse := map[string]string{
		"Cadaverous Bloom":      "gorge's census is unrelaxable, so no prefix search runs",
		"Glittering Stockpile":  "mana ability with a counter rider beside another mana ability",
		"The Eternity Elevator": "granted mana ability with a dynamic amount",
	}
	var names []string
	for n := range accept {
		names = append(names, n)
	}
	for n := range refuse {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		seen++
		if accept[c.Name] {
			if len(c.Refusals) > 0 {
				t.Errorf("%s: refused: %v", c.Name, c.Refusals)
			}
			continue
		}
		want, ok := refuse[c.Name]
		if !ok {
			t.Fatalf("lowered unexpected card %s", c.Name)
		}
		found := false
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: no refusal naming %q (got %v)", c.Name, want, c.Refusals)
		}
	}
	if seen != len(names) {
		t.Errorf("lowered %d of %d cards", seen, len(names))
	}
}
