package kgen

import (
	"strings"
	"testing"
)

// TestDefilerCycleLowers: the five Defiler cards' board OptionalCost lowers to
// one Defiler static each (kop.StatDefiler: life 2, the card's own colour) and
// carries no refusal.
func TestDefilerCycleLowers(t *testing.T) {
	for _, tc := range []struct {
		name string
		col  uint8
	}{
		{"Defiler of Faith", 1}, {"Defiler of Dreams", 2}, {"Defiler of Flesh", 3}, {"Defiler of Instinct", 4}, {"Defiler of Vigor", 5},
	} {
		c := lowerNamedCard(t, tc.name)
		n := 0
		for _, st := range c.Statics {
			if st.Kind == "Defiler" {
				n++
				if st.Amount.N != 2 || st.Defiler == 0 || st.F == nil {
					t.Errorf("%s: static life %d colour %d filter %v", tc.name, st.Amount.N, st.Defiler, st.F)
				}
			}
		}
		if n != 1 {
			t.Errorf("%s lowers to %d Defiler statics, want one", tc.name, n)
		}
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", tc.name, c.Refusals)
		}
	}
}

// TestDefilerShapes: a board OptionalCost beside the corpus shape refuses by name.
func TestDefilerShapes(t *testing.T) {
	const head = "Name:Defiler Probe\nManaCost:3 G G\nTypes:Creature Wurm\nPT:6/6\nS:Mode$ OptionalCost | ValidCard$ Permanent.Green | ValidSA$ Spell | "
	for _, tc := range []struct{ name, rest, want string }{
		{"Defiler Probe Cost", "Activator$ You | Cost$ Discard<1/Card> | ReduceColor$ G", `Cost "Discard<1/Card>"`},
		{"Defiler Probe Opp", "Activator$ Opponent | Cost$ PayLife<2> | ReduceColor$ G", `Activator "Opponent"`},
		{"Defiler Probe Col", "Activator$ You | Cost$ PayLife<2> | ReduceColor$ C", `ReduceColor "C"`},
		{"Defiler Probe Two", "Activator$ You | Cost$ PayLife<2> | ReduceColor$ GR", `ReduceColor "GR"`},
		{"Defiler Probe NoCost", "Activator$ You | ReduceColor$ G", "Cost"},
	} {
		c := lowerScriptCard(t, tc.name, head+tc.rest+"\nOracle:probe\n")
		hit := false
		for _, rf := range c.Refusals {
			hit = hit || strings.Contains(rf.Reason, tc.want)
		}
		if !hit {
			t.Errorf("%s refusals = %v, want one containing %q", tc.name, c.Refusals, tc.want)
		}
	}
}
