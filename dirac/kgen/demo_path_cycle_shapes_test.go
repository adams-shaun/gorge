package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoPathCycleShapes: the five Path of the X sorceries lower with no
// refusal. Four chain through Forge's DB$ BlankLine spacer (gorge
// effBlankLine is a silent no-op, effects/planechase.go) and Path of the
// Schemer puts a picked graveyard creature onto the battlefield with
// StaticEffect$ Animate (gorge applyStaticEffect), the pick's entry followed
// by the OvEnterWith override the kernel applies as the card enters.
func TestDemoPathCycleShapes(t *testing.T) {
	names := []string{"Path of the Animist", "Path of the Enigma", "Path of the Ghosthunter", "Path of the Pyromancer", "Path of the Schemer"}
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
	schemer := false
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		if b.def.Name != "Path of the Schemer" {
			continue
		}
		for _, ab := range b.def.Abilities {
			for i, in := range ab.Prog {
				if in.Op != kop.OpChooseMove || i+1 >= len(ab.Prog) {
					continue
				}
				nx := ab.Prog[i+1]
				if nx.Op == kop.OpOverride && nx.B.N == kop.OvEnterWith {
					schemer = true
				}
			}
		}
	}
	if !schemer {
		t.Error("Path of the Schemer: no graveyard pick followed by an OvEnterWith override")
	}
}
