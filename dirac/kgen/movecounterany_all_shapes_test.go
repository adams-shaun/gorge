package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMoveCounterAnyAllShapes lowers the two MoveCounter CounterType$ shapes
// the counter-kind work leaves: Nesting Grounds' "move a counter" activation
// (CounterType$ Any CounterNum$ 1, the two-target form -- Source$ ParentTarget
// is the chosen origin, the node's own target the destination; gorge asks the
// kind only when the origin holds more than one) and The Ozolith's beginning-
// of-combat move (Source$ Self, CounterType$ All CounterNum$ All onto a
// target creature). Both lower with no refusal.
func TestMoveCounterAnyAllShapes(t *testing.T) {
	names := []string{"Nesting Grounds", "The Ozolith"}
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
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
	}
	var kindMove, allMove bool
	for _, b := range bs {
		for j := range b.def.Abilities {
			for _, in := range b.def.Abilities[j].Prog {
				switch in.Op {
				case kop.OpMoveCounterKind:
					if b.def.Name == "Nesting Grounds" {
						kindMove = true
					}
				case kop.OpMoveCounterAll:
					if b.def.Name == "The Ozolith" {
						allMove = true
					}
				}
			}
		}
	}
	if !kindMove {
		t.Error("Nesting Grounds lowered no MoveCounterKind")
	}
	if !allMove {
		t.Error("The Ozolith lowered no MoveCounterAll")
	}
}
