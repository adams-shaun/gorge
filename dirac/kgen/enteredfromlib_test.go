package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEnteredFromLibraryWordLowers: ThisTurnEnteredFrom_Library (The Weaver
// King, Captain N'ghathrod) lowers to the PredWord7 sub-kind
// kop.W7EnteredFromLib on a card off the battlefield, distinct from the
// battlefield word, and a permanent read of it refuses by name.
func TestEnteredFromLibraryWordLowers(t *testing.T) {
	known := func(string) bool { return false }
	f, err := parseFilter("Creature.ThisTurnEnteredFrom_Library", known, known)
	if err != nil || len(f.Clauses) != 1 || f.Clauses[0].CmpStat != cmpWord7 || f.Clauses[0].CmpN != int(kop.W7EnteredFromLib) {
		t.Fatalf("library origin: got %s, %v", f, err)
	}
	if kop.W7EnteredFromLib == kop.W7EnteredFromBF {
		t.Fatal("the library word must not share the battlefield word's sub-kind")
	}
	cb := &cardBuilder{pt: &poolTables{filterKey: map[string]kop.FilterID{}, exprKey: map[string]int16{}}}
	kf, err := cb.clause(f.Clauses[0], kop.ZoneGraveyard)
	if err != nil || kf.Pred != kop.PredWord7 || kf.PredN&0xff != kop.W7EnteredFromLib {
		t.Errorf("graveyard read: got %+v, %v", kf, err)
	}
	if _, err := cb.clause(f.Clauses[0], kop.ZoneBattlefield); err == nil || !strings.Contains(err.Error(), "ThisTurnEnteredFrom_Library on the battlefield") {
		t.Errorf("battlefield read: err %v, want the named refusal", err)
	}
}

// TestZoneMoves5GraveMayPlayRefusals: the two graveyard MayPlay$ shapes gorge
// reads differently from Forge stay refused by name: a later Permanent
// alternative (Kagha, Shadow Archdruid) and a typed grant over lands you may
// not own (Coram, the Undertaker). An own-land typed grant still lowers.
func TestZoneMoves5GraveMayPlayRefusals(t *testing.T) {
	if err := laterPermanentAlternative("Land.YouOwn+ThisTurnEnteredFrom_Library,Permanent.YouOwn+ThisTurnEnteredFrom_Library"); err == nil || !strings.Contains(err.Error(), "later Permanent alternative") {
		t.Errorf("Kagha's Affected$: err %v, want the later-Permanent refusal", err)
	}
	if err := laterPermanentAlternative("Permanent.YouOwn,Land.YouOwn"); err != nil {
		t.Errorf("a leading Permanent is rewritten by gorge too: %v", err)
	}
	known := func(string) bool { return false }
	anyLand, err := parseFilter("Land.ThisTurnEnteredFrom_Library", known, known)
	if err != nil {
		t.Fatal(err)
	}
	own, err := parseFilter("Land.YouOwn", known, known)
	if err != nil {
		t.Fatal(err)
	}
	typed := newParams(map[string]string{"MayPlayText": "Land", "AffectedZone": "Graveyard"})
	if err := ownGraveLandsOnly(typed, anyLand); err == nil || !strings.Contains(err.Error(), "land you may not own") {
		t.Errorf("Coram's land grant: err %v, want the own-graveyard refusal", err)
	}
	if err := ownGraveLandsOnly(typed, own); err != nil {
		t.Errorf("a YouOwn land grant: %v", err)
	}
	if err := ownGraveLandsOnly(newParams(map[string]string{"AffectedZone": "Graveyard"}), anyLand); err != nil {
		t.Errorf("an untyped grant registers a gorge may-play effect, whose walk reads every graveyard: %v", err)
	}
}
