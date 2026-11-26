package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSpellCopyHasNoAltCostMark: a copy of a spell cast for its own
// alternative cost is not cast (CR 707.10), so Count$AltCost reads unpaid on
// it: mintSpellCopy clears the trigPlAltCost mark the original carries.
func TestSpellCopyHasNoAltCostMark(t *testing.T) {
	tb, ids := kbargainTestTable(kop.CardDef{Name: "Test Altcost Copy", Types: kop.TSorcery})
	sc := newScenTable(t, tb, Config{})
	s := sc.s
	o := sc.hand(ids[0], 0)
	s.stack[0] = stackEntry{kind: seSpell, obj: o, card: ids[0], ctrl: 0, trigPl: trigPlAltCost}
	s.sp = 1
	if !s.altCostCast(evalCtx{e: &s.stack[0]}) {
		t.Fatal("precondition: the original does not read as alt-cost cast")
	}
	if !s.mintSpellCopy(sc.k, 0, 0) {
		t.Fatal("mintSpellCopy failed")
	}
	if s.sp != 2 {
		t.Fatalf("precondition: stack has %d entries, want the original and its copy", s.sp)
	}
	if s.stack[1].trigPl != trigPlAltCost {
		t.Fatal("precondition: the original lost its mark")
	}
	if s.altCostCast(evalCtx{e: &s.stack[0]}) {
		t.Error("the copy reads as cast for the alternative cost")
	}
}
