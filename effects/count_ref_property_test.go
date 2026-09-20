package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// The <Ref>$<Property> count family the DamageSource$ work rides on: before
// the evaluator these bodies evaluated to zero (Kiku's Shadow dealt no
// damage at all), so every branch here pins a formerly-dead head answering
// its object's real property.
func TestRefPropertyCounts(t *testing.T) {
	h := newHost(t, 2)
	power := mkCard(t, "Name:Ox\nTypes:Creature Ox\nPT:3/4\nOracle:x\n")
	cheap := mkCard(t, "Name:Dork\nTypes:Creature Dork\nPT:1/1\nManaCost:2\nOracle:x\n")
	ao := h.g.AddObject(power, 0)
	bo := h.g.AddObject(cheap, 0)
	a, b := ao.ID, bo.ID
	h.g.Obj(a).AddCounter("P1P1", 2)
	c := &Ctx{Source: a, Controller: 0, Targets: []state.Target{{Obj: a}, {Obj: b}},
		Remembered: []state.Target{{Obj: b}}}
	for _, tc := range []struct {
		expr string
		want int32
	}{
		// Targeted$CardPower sums over the targets (3+2 counters, 1): Vein
		// Drinker's second half; ParentTargeted$CardPower reads the same
		// list; TriggeredCard$CardPower reads the Remembered objects.
		{"Targeted$CardPower", 3 + 2 + 1},
		{"ParentTargeted$CardPower", 3 + 2 + 1},
		{"TriggeredCard$CardPower", 1},
		{"Targeted$CardToughness", 4 + 2 + 1},
		{"Targeted$CardManaCost", 2},
		{"TriggeredCard$CardManaCost", 2},
		// Remembered$Amount stays the remembered-entry count (the pre-existing
		// head, unbroken); Remembered$CardPower is the new family on the same
		// prefix.
		{"Remembered$Amount", 1},
		{"Remembered$CardPower", 1},
		// The Valid head counts referenced objects matching a card spec;
		// unknown predicates inside the spec fail closed (never match).
		{"Remembered$Valid Creature", 1},
		{"Remembered$Valid NonexistentType", 0},
		// AllTargeted$ binds this engine's Ctx.Targets (the root SA's chosen
		// targets) -- the faithful-as-available reading of Forge's whole-chain
		// union, whose sub-ability targets this engine defers to resolution
		// (AGENTS.md's Known approximations). Rows share Targeted's fixture.
		{"AllTargeted$CardPower", 3 + 2 + 1},
		{"AllTargeted$CardManaCost", 2},
		{"AllTargeted$Valid Creature.powerLE3", 1}, // Dork (1) matches, Ox (5) does not
		{"AllTargeted$CardPower/Twice", (3 + 2 + 1) * 2},
		// An unknown ref or property stays zero (the conservative no-op), and
		// a /Op suffix applies through the ordinary arithmetic.
		{"Remembered$ChromaSource", 0},
		{"TriggeredTarget$LifeTotal", 0},
		{"UnknownRef$CardPower", 0},
		{"Targeted$CardPower/Twice", (3 + 2 + 1) * 2},
	} {
		if got := EvalCount(h, c, tc.expr); got != tc.want {
			t.Errorf("%s = %d, want %d", tc.expr, got, tc.want)
		}
	}
}
