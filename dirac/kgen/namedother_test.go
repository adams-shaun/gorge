package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// a named<X> argument normalises as gorge's nameArg does,
// and a literal named clause builds Pred = PredNamed against the pool's
// printed names (-1 when no entry has the name; two entries refuse).
func TestNamedotherNameArg(t *testing.T) {
	f, err := parseFilter("Card.namedAether_Burst", func(string) bool { return false }, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Clauses) != 1 || f.Clauses[0].Name != "Aether Burst" {
		t.Fatalf("clauses %+v, want one named Aether Burst", f.Clauses)
	}
}

func TestNamedotherBuild(t *testing.T) {
	lw := &lowered{
		cards:  []*lCard{{Name: "Rufus Shinra"}, {Name: "Darkstar"}},
		tokens: []*lCard{{Name: "darkstar", Token: true, printed: "Darkstar"}},
	}
	cb := &cardBuilder{pt: newPoolTables(lw, nil), c: lw.cards[0]}
	c := lClause{Types: tCreature, Name: "Nobody Here"}
	kf, err := cb.clause(c, kop.ZoneBattlefield)
	if err != nil {
		t.Fatal(err)
	}
	if kf.Pred != kop.PredNamed || kf.PredN != -1 {
		t.Fatalf("absent name: Pred %d PredN %d, want PredNamed -1", kf.Pred, kf.PredN)
	}
	c.Name = "Rufus Shinra"
	if kf, err = cb.clause(c, kop.ZoneLibrary); err != nil || kf.Pred != kop.PredNamed || kf.PredN != 0 {
		t.Fatalf("one entry: Pred %d PredN %d err %v, want PredNamed 0", kf.Pred, kf.PredN, err)
	}
	c.Name = "Darkstar" // a card and a token share it
	if _, err = cb.clause(c, kop.ZoneBattlefield); err == nil || !strings.Contains(err.Error(), "names 2 pool entries") {
		t.Fatalf("two entries: err %v, want a refusal naming 2 pool entries", err)
	}
}
