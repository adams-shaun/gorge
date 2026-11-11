package kgen

import (
	"strings"
	"testing"
)

// TestOnceAbilityOrdinals: Draconautics Engineer's two Exhaust abilities and
// Loot, the Pathfinder's three lower with their once-only ordinals in ability
// order, so the kernel can keep one bit per ability (pfOnceGame and the
// mechOnce2/3 arena masks).
func TestOnceAbilityOrdinals(t *testing.T) {
	want := map[string]int{"Draconautics Engineer": 2, "Loot, the Pathfinder": 3}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		n, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		if len(c.Refusals) > 0 {
			t.Errorf("%s refused: %v", c.Name, c.Refusals)
			continue
		}
		var got []int
		for _, a := range c.Abilities {
			if a.Limit == 1 {
				got = append(got, a.Once)
			}
		}
		if len(got) != n {
			t.Errorf("%s: %d once-only abilities, want %d", c.Name, len(got), n)
		}
		for i, once := range got {
			if once != i+1 {
				t.Errorf("%s: once-only ability %d has ordinal %d, want %d", c.Name, i, once, i+1)
			}
		}
	}
	if seen != len(want) {
		t.Fatalf("lowered %d of %d cards", seen, len(want))
	}
}

// TestOnceAbilityFourthRefused: a fourth activate-only-once ability has no
// kernel carrier (pfOnceGame plus two arena masks) and is refused by name.
func TestOnceAbilityFourthRefused(t *testing.T) {
	lc := &lowerCtx{name: "Test Four Once", out: &lCard{}}
	for i := 0; i < 4; i++ {
		lc.out.Abilities = append(lc.out.Abilities, &lAbility{Kind: akActivated, Limit: 1})
	}
	lc.numberOnceAbilities()
	if len(lc.out.Refusals) != 1 || !strings.Contains(lc.out.Refusals[0].Reason, "four activate-only-once") {
		t.Fatalf("refusals %v, want the four-once refusal", lc.out.Refusals)
	}
	for i := 0; i < 3; i++ {
		if got := lc.out.Abilities[i].Once; got != i+1 {
			t.Errorf("ability %d ordinal %d, want %d", i, got, i+1)
		}
	}
}
