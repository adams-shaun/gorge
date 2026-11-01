package rules

import "testing"

// A layer-7 pump whose Affected$ names a colour reads the layer-5 colours
// (CR 613.1e applies before 613.4c): Heraldic Banner (+1/+0 to creatures of
// the chosen colour) pumps a creature Witness Protection made green and
// white by its green, not its printed black (dirac lockstep pairs 110,
// 1004). The scalar P/T walk matched the pump against the printed colours.
func TestLayer7ColourFilterReadsLayer5Colours(t *testing.T) {
	const painter = "Name:Green Painter\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Creature | SetColor$ Green\nOracle:x\n"
	const rat = "Name:Black Rat\nManaCost:B\nTypes:Creature Rat\nPT:1/1\nOracle:x\n"
	for _, tc := range []struct {
		lord string
		want int32
	}{
		{"Name:Green Lord\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Creature.Green | AddPower$ 1\nOracle:x\n", 2},
		{"Name:Black Lord\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Creature.Black | AddPower$ 1\nOracle:x\n", 1},
	} {
		e := mayPlayBase(t)
		r := onBoardGrant(t, e, 1, rat)
		onBoardGrant(t, e, 0, painter)
		onBoardGrant(t, e, 1, tc.lord)
		if got := e.Colors(r); got != "G" {
			t.Fatalf("Black Rat colours under the painter = %q, want G", got)
		}
		if got := e.Power(r); got != tc.want {
			t.Errorf("%s: recoloured Black Rat power = %d, want %d", tc.lord[5:15], got, tc.want)
		}
		if got := e.Derived(r).Power; got != tc.want {
			t.Errorf("%s: Derived power = %d, want %d", tc.lord[5:15], got, tc.want)
		}
	}
}
