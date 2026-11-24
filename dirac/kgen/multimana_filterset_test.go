package kgen

import (
	"strings"
	"testing"
)

// TestMultiManaFilterSet (T18e4): a plain {T}: Add {C} first ability beside
// two or more paid filter abilities ({1},{T}: Add <colour>, or {2},{T}:
// Add {B} or {G}) stays refused until the kernel's filter walk holds one
// filter ability per colour and models a fixed-colour or choice fee (it
// records one filter ability per card and needs ColAny), and the refusal
// names that reason; the other shapes of the nine T18e cards keep their
// refusal, naming their actual reason.
func TestMultiManaFilterSet(t *testing.T) {
	refuse := map[string]string{
		"Koskun Keep":           "the kernel's filter walk records one filter ability per card",
		"Castle Sengir":         "the kernel's filter walk records one filter ability per card",
		"An-Havva Township":     "the kernel's filter walk records one filter ability per card",
		"Aysen Abbey":           "the kernel's filter walk records one filter ability per card",
		"Wizards' School":       "the kernel's filter walk records one filter ability per card",
		"Murmuring Bosk":        "a pain rider beside a coloured plain {T} ability",
		"Cadaverous Bloom":      "gorge's census is unrelaxable, so no prefix search runs",
		"Glittering Stockpile":  "mana ability with a conditional or unbounded rider program",
		"The Eternity Elevator": "granted mana ability with a dynamic amount",
	}
	var names []string
	for n := range refuse {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		seen++
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
