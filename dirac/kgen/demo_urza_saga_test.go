package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoUrzaSagaShapes: Urza's Saga lowers with no refusal. Its Chapter III
// search filter "Artifact.ManaCost0,Artifact.ManaCost1" is two clauses, each
// a FwManaCostEQ literal over its mana value (0 and 1).
func TestDemoUrzaSagaShapes(t *testing.T) {
	names := []string{"Urza's Saga"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
	}
	seen := map[int16]bool{}
	for _, p := range pt.progs {
		for _, l := range p.Lits {
			if l.W == kop.FwManaCostEQ {
				seen[l.Arg] = true
			}
		}
	}
	if !seen[0] || !seen[1] || len(seen) != 2 {
		t.Errorf("FwManaCostEQ values in the filter programs = %v, want exactly 0 and 1", seen)
	}
}
