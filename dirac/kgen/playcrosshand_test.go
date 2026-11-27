package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// playCrossHandForms: the Play populations over the controller's hand read
// through a cross-object referent. Twinning Glass's Valid$
// Card.nonLand+sharesNameWith ThisTurnCast (ValidZone$ Hand, ValidSA$ Spell,
// WithoutManaCost$ True, Optional$ True) lowers to an optional free CastFree
// over the hand (C=2) whose filter reads FwSharesNameThisTurnCast.
var playCrossHandForms = []formCase{
	{card: "Twinning Glass", form: "free cast from hand of a card sharing a name with a spell cast this turn", lir: "CastFree C=2 CS=Optional|One"},
}

func TestPlayCrossHandShapes(t *testing.T) {
	checkForms(t, playCrossHandForms)
	cb := &cardBuilder{pt: &poolTables{missing: map[string]bool{}}}
	no := func(string) bool { return false }
	f, err := parseFilter("Card.nonLand+sharesNameWith ThisTurnCast", no, no)
	if err != nil {
		t.Fatal(err)
	}
	kf, err := cb.clause(f.Clauses[0], kop.ZoneHand)
	if err != nil {
		t.Fatalf("sharesNameWith ThisTurnCast: %v", err)
	}
	if kf.Pred != kop.PredProg {
		t.Fatalf("Pred %d, want PredProg", kf.Pred)
	}
	found := false
	for _, l := range cb.pt.progs[kf.PredN].Lits {
		found = found || l.W == kop.FwSharesNameThisTurnCast && !l.Neg
	}
	if !found {
		t.Fatalf("program %+v has no positive FwSharesNameThisTurnCast", cb.pt.progs[kf.PredN].Lits)
	}
	// the negated spelling stays refused: gorge fails it closed on an empty
	// ledger, which a negated literal would invert
	if nf, err := parseFilter("Card.nonLand+!sharesNameWith ThisTurnCast", no, no); err == nil {
		if _, err := cb.clause(nf.Clauses[0], kop.ZoneHand); err == nil {
			t.Fatal("!sharesNameWith ThisTurnCast lowered; want a refusal")
		}
	}
}
