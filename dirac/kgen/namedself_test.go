package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// a named<X> whose X is the source's own printed name lowers
// as a literal PredNamed like every other name (gorge wordNamed ->
// sharesName reads the name, not the source), so it still counts after the
// source has left the battlefield.
func TestNamedselfLiteral(t *testing.T) {
	face := &cards.Face{Name: "Hare Apparent", SVars: map[string]string{}}
	lc := &lowerCtx{face: face, p: &pool{}, known: func(string) bool { return false }, out: &lCard{}}
	f, err := lc.filter("Creature.namedHare Apparent+YouCtrl+Other")
	if err != nil {
		t.Fatal(err)
	}
	var named *lClause
	for i := range f.Clauses {
		if f.Clauses[i].Name != "" {
			named = &f.Clauses[i]
		}
	}
	if named == nil || named.Name != "Hare Apparent" {
		t.Fatalf("clauses %+v, want one named Hare Apparent", f.Clauses)
	}
	if named.Flags&fNamedAsSource != 0 {
		t.Fatalf("clause keeps fNamedAsSource: %+v", *named)
	}
	lw := &lowered{cards: []*lCard{{Name: "Hare Apparent"}}}
	cb := &cardBuilder{pt: newPoolTables(lw, nil), c: lw.cards[0]}
	for _, zone := range []kop.Zone{kop.ZoneBattlefield, kop.ZoneHand} {
		kf, err := cb.clause(*named, zone)
		if err != nil {
			t.Fatal(err)
		}
		if kf.Pred != kop.PredNamed || kf.PredN != 0 || kf.Flags2&kop.F2SameNameAsSource != 0 {
			t.Fatalf("zone %d: Pred %d PredN %d Flags2 %#x, want PredNamed 0 and no SameNameAsSource", zone, kf.Pred, kf.PredN, kf.Flags2)
		}
	}
}

// TestNamedselfFDNTable: the FDN table holds Hare Apparent's token count as the
// literal PredNamed on its own matchup id.
func TestNamedselfFDNTable(t *testing.T) {
	deck := []cardset.Entry{{Name: "Hare Apparent", Count: 1}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	hare := -1
	for _, c := range da {
		if tbl.Cards[c].Name == "Hare Apparent" {
			hare = int(c)
		}
	}
	if hare < 0 {
		t.Fatal("Hare Apparent not in the matchup deck")
	}
	found := false
	for _, ab := range tbl.Cards[hare].Abilities {
		if ab.Kind != kop.AbTriggered || ab.Trig.Event != kop.TrigETB {
			continue
		}
		for _, in := range ab.Prog {
			if in.Op != kop.OpCreateToken {
				continue
			}
			found = true
			f := tbl.Filter(kop.FilterID(in.A.N))
			if f.Pred != kop.PredNamed || int(f.PredN) != hare || f.Flags2&kop.F2SameNameAsSource != 0 {
				t.Fatalf("token count filter: Pred %d PredN %d Flags2 %#x, want PredNamed %d and no SameNameAsSource", f.Pred, f.PredN, f.Flags2, hare)
			}
		}
	}
	if !found {
		t.Fatal("no ETB OpCreateToken on Hare Apparent")
	}
}
