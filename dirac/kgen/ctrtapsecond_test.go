package kgen

import (
	"slices"
	"testing"
)

// TestCounterTapSecondManaAbility: "{T}, remove N counters from this: Add
// <fixed colour>" beside a plain {T} first ability lowers (Weather Maker's
// {C}{C}: the kernel's counter gap kind reads a second ability,
// mana_countercost.go), and so does one mana of any colour (the Vivid lands,
// Tendo Ice Bridge: the activation asks the colour); the same cost shape with
// a restricted production stays refused (Mirrodin's Core, Throne of Makindi).
func TestCounterTapSecondManaAbility(t *testing.T) {
	lowered := []string{"Weather Maker", "Vivid Grove", "Vivid Creek", "Vivid Meadow", "Vivid Crag", "Vivid Marsh", "Tendo Ice Bridge"}
	refused := []string{"Mirrodin's Core", "Throne of Makindi"}
	lw := lowerNamedPool(t, append(slices.Clone(lowered), refused...))
	got := map[string]int{}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Logf("%s: %s", c.Name, r.Reason)
		}
		got[c.Name] += len(c.Refusals)
	}
	for _, name := range lowered {
		if n, ok := got[name]; !ok || n != 0 {
			t.Errorf("%s: %d refusals (lowered=%v), want a clean lowering", name, n, ok)
		}
	}
	for _, n := range refused {
		if got[n] == 0 {
			t.Errorf("%s lowered clean (or is not in the pool), want a refusal: its production is restricted", n)
		}
	}
}
