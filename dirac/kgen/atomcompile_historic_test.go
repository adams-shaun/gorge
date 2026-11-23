package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAtomCompilerHistoricNeedsNoSaga: over a table with no Saga subtype
// Historic lowers to its matchup form (artifact or legendary, PredN 0), which
// the kernel answers, so the compiler reports nothing missing; Outlaw's five
// types still report a miss.
func TestAtomCompilerHistoricNeedsNoSaga(t *testing.T) {
	a := NewAtomCompiler()
	id, err := a.Filter("Card.Historic", kop.ZoneBattlefield)
	if err != nil {
		t.Fatal(err)
	}
	fs, _ := a.Tables()
	if fs[id].Pred != kop.PredHistoric || fs[id].PredN != 0 {
		t.Fatalf("Card.Historic over a table with no Saga lowered to %+v, want PredHistoric with PredN 0", fs[id])
	}
	if m := a.Missing(); len(m) != 0 {
		t.Errorf("Card.Historic reports missing subtypes %v over a table with no Saga", m)
	}
	if _, err := a.Filter("Card.Outlaw", kop.ZoneBattlefield); err != nil {
		t.Fatal(err)
	}
	if len(a.Missing()) == 0 {
		t.Errorf("Card.Outlaw reports nothing missing over a table with none of its types")
	}
}
