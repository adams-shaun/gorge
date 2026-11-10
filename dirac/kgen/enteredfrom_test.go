package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEnteredFromWordLowers: ThisTurnEnteredFrom_Battlefield lowers to the
// PredWord7 sub-kind kop.W7EnteredFromBF on a card off the battlefield; the
// same word names any other origin zone and refuses, and a permanent read of
// it refuses (the word reads a card's latest arrival).
func TestEnteredFromWordLowers(t *testing.T) {
	known := func(string) bool { return false }
	f, err := parseFilter("Creature.ThisTurnEnteredFrom_Battlefield", known, known)
	if err != nil || len(f.Clauses) != 1 || f.Clauses[0].CmpStat != cmpWord7 || f.Clauses[0].CmpN != int(kop.W7EnteredFromBF) {
		t.Fatalf("battlefield origin: got %s, %v", f, err)
	}
	for _, z := range []string{"Hand", "Library", "Graveyard", "Exile"} {
		_, err := parseFilter("Creature.ThisTurnEnteredFrom_"+z, known, known)
		if err == nil || !strings.Contains(err.Error(), "ThisTurnEnteredFrom_"+z) {
			t.Errorf("origin %s: err %v, want a refusal naming the word", z, err)
		}
	}
	cb := &cardBuilder{pt: &poolTables{filterKey: map[string]kop.FilterID{}, exprKey: map[string]int16{}}}
	kf, err := cb.clause(f.Clauses[0], kop.ZoneGraveyard)
	if err != nil || kf.Pred != kop.PredWord7 || kf.PredN&0xff != kop.W7EnteredFromBF {
		t.Errorf("graveyard read: got %+v, %v", kf, err)
	}
	if _, err := cb.clause(f.Clauses[0], kop.ZoneBattlefield); err == nil || !strings.Contains(err.Error(), "ThisTurnEnteredFrom_Battlefield on the battlefield") {
		t.Errorf("battlefield read: err %v, want the named refusal", err)
	}
}
