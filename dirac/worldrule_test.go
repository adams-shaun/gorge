package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWorldRuleKeepsNewest: two Concordant Crossroads, one under each seat;
// the CR 704.5k world rule puts the older one into its owner's graveyard and
// keeps the newer one, posing no ask (gorge rules/sba.go worldRule).
func TestWorldRuleKeepsNewest(t *testing.T) {
	set := cardsettest.Set(t, "worldrule")
	deck := []cardset.Entry{{Name: "Concordant Crossroads", Count: 2}, {Name: "Forest", Count: 8}}
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the worldrule table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cc := id["Concordant Crossroads"]
	if !tbl.Cards[cc].World {
		t.Fatal("precondition: Concordant Crossroads has no World flag")
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	if !sc.k.gen.anyWorld {
		t.Fatal("precondition: anyWorld not set")
	}
	old := sc.perm(cc, 0)
	young := sc.perm(cc, 1)
	if !sc.onBF(old) || !sc.onBF(young) {
		t.Fatal("precondition: both Crossroads on the battlefield")
	}
	if a, b := sc.s.layerTs(sc.slot(old)), sc.s.layerTs(sc.slot(young)); a >= b {
		t.Fatalf("precondition: entry stamps %d, %d: want the second newer", a, b)
	}
	if sc.s.sba(sc.k) {
		t.Fatalf("the world rule posed an ask (ctl %d)", sc.s.ctl)
	}
	if z := sc.s.objs[old].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the older Crossroads is in zone %d, want the graveyard", z)
	}
	if !sc.onBF(young) {
		t.Fatal("the newer Crossroads left the battlefield")
	}
}
