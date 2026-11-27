package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoTokenattackingShapes: the three demo TokenAttacking$ shapes lower
// with no refusal and carry the attacking bits the kernel reads:
// Flamerush Rider's Attacks-copy (CopyPermanent TokenAttacking$ True beside
// ValidTgts$), Shaun, Father of Synths' AttackersDeclared copy and Sigiled
// Sword of Valeron's Token in an equipped-creature Attacks trigger.
func TestDemoTokenattackingShapes(t *testing.T) {
	names := []string{"Flamerush Rider", "Shaun, Father of Synths", "Sigiled Sword of Valeron"}
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
	copies, tokens := 0, 0
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		for _, ab := range b.def.Abilities {
			for _, in := range ab.Prog {
				switch {
				case in.Op == kop.OpCopyAsTokenEx && uint16(in.B.N)&kop.CopyAttacking != 0:
					copies++
				case in.Op == kop.OpCreateTokenEx &&
					in.B.N&(kop.TokAttacking|kop.TokAttackAsk) == kop.TokAttacking|kop.TokAttackAsk:
					tokens++
				}
			}
		}
	}
	if copies != 2 || tokens != 1 {
		t.Errorf("%d attacking copies and %d attacking tokens, want 2 and 1", copies, tokens)
	}
}
