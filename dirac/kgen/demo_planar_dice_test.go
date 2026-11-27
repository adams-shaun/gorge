package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoPlanarDiceShapes: the planar die's two corpus carriers lower with
// no refusal (the demo decks' RollPlanarDice shape). Fractured Powerstone's
// {T} ability is ExtRollPlanarDice with the default one die, and Ichor
// Elixir's R: line is a ReplRollPlanarDice replacement that adds one die and
// ignores one, the exact rewrite its ReplaceEffect SVar chain spells.
func TestDemoPlanarDiceShapes(t *testing.T) {
	names := []string{"Fractured Powerstone", "Ichor Elixir"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	rolls, replacements := 0, 0
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		for _, ab := range b.def.Abilities {
			if ab.Kind == kop.AbReplacement && ab.Repl.Kind == kop.ReplRollPlanarDice {
				if ab.Repl.A != 1 || ab.Repl.C != 1 {
					t.Errorf("%s: ReplRollPlanarDice plus %d ignore %d, want 1/1", b.def.Name, ab.Repl.A, ab.Repl.C)
				}
				replacements++
			}
			for _, in := range ab.Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtRollPlanarDice {
					if in.B != kop.Const(1) {
						t.Errorf("%s: ExtRollPlanarDice amount %v, want the default 1", b.def.Name, in.B)
					}
					rolls++
				}
			}
		}
	}
	if rolls != 1 {
		t.Errorf("lowered %d ExtRollPlanarDice instructions, want 1 (Fractured Powerstone)", rolls)
	}
	if replacements != 1 {
		t.Errorf("lowered %d ReplRollPlanarDice abilities, want 1 (Ichor Elixir)", replacements)
	}
}
