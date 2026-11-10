package kgen

import "testing"

// TestCounterTapSecondManaAbility: "{T}, remove N counters from this: Add
// <fixed colour>" beside a plain {T} first ability lowers (Weather Maker's
// {C}{C}: the kernel's counter gap kind reads a second ability,
// mana_countercost.go), and the same cost shape with an any-colour or a
// restricted production stays refused (Mirrodin's Core, Throne of Makindi,
// Vivid Crag).
func TestCounterTapSecondManaAbility(t *testing.T) {
	refused := []string{"Mirrodin's Core", "Throne of Makindi", "Vivid Crag"}
	lw := lowerNamedPool(t, append([]string{"Weather Maker"}, refused...))
	got := map[string]int{}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Logf("%s: %s", c.Name, r.Reason)
		}
		got[c.Name] += len(c.Refusals)
	}
	if n, ok := got["Weather Maker"]; !ok || n != 0 {
		t.Errorf("Weather Maker: %d refusals (lowered=%v), want a clean lowering", n, ok)
	}
	for _, n := range refused {
		if got[n] == 0 {
			t.Errorf("%s lowered clean (or is not in the pool), want a refusal: its production is not one fixed unrestricted colour", n)
		}
	}
}
