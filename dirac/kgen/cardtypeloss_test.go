package kgen

import (
	"slices"
	"testing"
)

// TestCardTypeRemovalKeepsSubtypes: RemoveCardTypes$ keeps the subtypes of a
// card type the effect adds, unless it also adds one of that set (CR 205.1a).
func TestCardTypeRemovalKeepsSubtypes(t *testing.T) {
	want := map[string]struct{ keep, gone []string }{
		"Imprisoned in the Moon": {[]string{"Forest", "Desert"}, []string{"Vehicle", "Aura", "Elf"}},
		"Darksteel Mutation":     {[]string{"Vehicle", "Equipment"}, []string{"Forest", "Aura", "Elf"}},
		"Enchantmentize":         {[]string{"Aura", "Saga"}, []string{"Forest", "Vehicle", "Elf"}},
		"Sugar Coat":             {nil, []string{"Forest", "Treasure", "Aura", "Elf"}},
		"Honor the Past":         {[]string{"Vehicle"}, []string{"Forest", "Aura", "Elf"}},
		"Frogify":                {nil, []string{"Forest", "Vehicle", "Aura", "Frog"}},
	}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	slices.Sort(names)
	seen := 0
	for _, c := range lowerNamed(t, names) {
		w, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		if len(c.overrides) == 0 {
			t.Errorf("%s: no override lowered (refusals %v)", c.Name, c.Refusals)
			continue
		}
		o := c.overrides[0]
		for _, s := range w.keep {
			if !o.Kept.Has(s) {
				t.Errorf("%s: drops %s", c.Name, s)
			}
		}
		for _, s := range w.gone {
			if o.Kept.Has(s) {
				t.Errorf("%s: keeps %s", c.Name, s)
			}
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d carriers", seen, len(want))
	}
}
