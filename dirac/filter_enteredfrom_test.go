package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestEnteredFromBattlefieldWord pins kop.W7EnteredFromBF: a card matches
// when its latest arrival into its current zone came from the battlefield
// this turn, and only then.
func TestEnteredFromBattlefieldWord(t *testing.T) {
	tbl := *v.Table
	tbl.Filters = append(append([]kop.Filter(nil), v.Table.Filters...),
		kop.Filter{Pred: kop.PredWord7, PredN: kop.W7EnteredFromBF})
	fl := tbl.Filters[len(tbl.Filters)-1]
	sc := newScenTable(t, &tbl, Config{})
	s := sc.s
	if !s.g.enteredOn {
		t.Fatal("a table whose filter carries the word must record the entered ledger")
	}
	match := func(o Obj) bool { return s.wordCard(sc.k, &fl, s.objs[o].card, o, false, 0) }

	died := sc.perm(v.GrizzlyBears, 1)
	s.destroy(sc.k, sc.slot(died))
	if s.objs[died].zone != kop.ZoneGraveyard {
		t.Fatalf("precondition: died creature is in zone %d, want the graveyard", s.objs[died].zone)
	}
	if !match(died) {
		t.Fatal("a creature that died this turn must match")
	}

	disc := sc.hand(v.GrizzlyBears, 1)
	s.discardObj(sc.k, disc)
	if s.objs[disc].zone != kop.ZoneGraveyard || !s.g.zoneTurn.has(disc) {
		t.Fatalf("precondition: discarded card zone %d, entered this turn %v", s.objs[disc].zone, s.g.zoneTurn.has(disc))
	}
	if match(disc) {
		t.Fatal("a discarded card must not match")
	}

	milled := sc.obj(v.GrizzlyBears, 1)
	s.placeFrom(milled, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
	if match(milled) {
		t.Fatal("a milled card must not match")
	}

	// died, then graveyard -> exile -> graveyard: the latest origin is exile
	s.placeFrom(died, kop.ZoneExile, false, kop.ZoneGraveyard, false)
	if match(died) {
		t.Fatal("in exile after leaving the graveyard: latest origin is the graveyard")
	}
	s.placeFrom(died, kop.ZoneGraveyard, false, kop.ZoneExile, false)
	if match(died) {
		t.Fatal("back in the graveyard from exile must not match")
	}

	// a permanent put into exile straight from the battlefield matches there
	exiled := sc.perm(v.GrizzlyBears, 1)
	s.placeFrom(exiled, kop.ZoneExile, false, kop.ZoneBattlefield, true)
	if !match(exiled) {
		t.Fatal("exiled from the battlefield this turn must match")
	}

	// no object, no match; and nothing matches next turn
	if s.wordCard(sc.k, &fl, kop.NoCard, NoObj, false, 0) {
		t.Fatal("no object must not match")
	}
	s.newTurnGen()
	if match(exiled) {
		t.Fatal("nothing matches on the next turn")
	}
}
