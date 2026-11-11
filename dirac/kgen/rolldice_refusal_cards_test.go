package kgen

import (
	"strings"
	"testing"
)

// TestRollDiceCardsRefusedByName pins that the three roll-dice commander
// cards stay refused by the kernel lowering (no dirac support for R:Event$
// RollDice, RolledDie triggers or the roll-modify keyword), each with the
// named reason: gorge supports them, the kernel does not.
func TestRollDiceCardsRefusedByName(t *testing.T) {
	want := map[string]string{
		"Barbarian Class":                "repl-hard: RollDice replacement",
		"Wyll, Blade of Frontiers":       "repl-hard: RollDice replacement",
		"Night Shift of the Living Dead": "After you roll a die",
	}
	names := make([]string, 0, len(want))
	for n := range want {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		frag, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		rs := lowerRefusals(c)
		if len(rs) == 0 {
			t.Errorf("%s lowered with no refusal", c.Name)
		}
		found := false
		for _, r := range rs {
			if strings.Contains(r.Reason, frag) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no refusal contains %q; got %v", c.Name, frag, rs)
		}
	}
	if seen != len(want) {
		t.Fatalf("lowered %d of %d cards", seen, len(want))
	}
}
