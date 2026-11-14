package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// deferredScen is a scenario over a two-card-type deck holding the deferred
// source src and the spell cast.
func deferredScen(t *testing.T, set, src, spell, land string) (*scen, func(string) CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: src, Count: 1}, {Name: spell, Count: 1}, {Name: land, Count: 20}}
	tbl, da, _, err := cardsettest.Set(t, set).Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]CardID{}
	for _, c := range da {
		ids[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, ids[land], Config{NoElide: true})
	return sc, func(n string) CardID { return ids[n] }
}

// TestDeferredSpentSurvivesOffColourPool: Goggles' {R} pays Shock while a {W}
// floats beside it. The payment empties {R}, artSync forgets the deferred
// source, and deferredSpent must still attribute the spend (it reads the
// colour deferredPool took before the payment, not the cleared record).
func TestDeferredSpentSurvivesOffColourPool(t *testing.T) {
	sc, id := deferredScen(t, "goggles", "Pyromancer's Goggles", "Shock", "Mountain")
	gog, shock := id("Pyromancer's Goggles"), id("Shock")
	g := sc.perm(gog, 0)
	sc.s.floatDeferred(sc.k, 0, sc.slot(g))
	sc.s.pl[0].pool[kop.W]++
	if sc.s.g.defSrc[0] == 0 || sc.s.pl[0].pool[kop.R] != 1 {
		t.Fatalf("precondition: defSrc %d, pool R %d", sc.s.g.defSrc[0], sc.s.pl[0].pool[kop.R])
	}
	o := sc.obj(shock, 0)
	sc.kcastBuild(o, shock, 0)
	cost := sc.k.t.Cards[shock].Cost
	sc.s.payRider(sc.k, 0, &cost, 0, o, false)
	if sc.s.pl[0].pool[kop.R] != 0 || sc.s.pl[0].pool[kop.W] != 1 {
		t.Fatalf("pool after the payment: R %d W %d, want 0 and 1", sc.s.pl[0].pool[kop.R], sc.s.pl[0].pool[kop.W])
	}
	if got := sc.s.bld().obsSl; got != sc.slot(g)+1 {
		t.Fatalf("obsSl %d, want the Goggles' slot + 1 = %d", got, sc.slot(g)+1)
	}
}

// TestDeferredSourceForgottenOnOtherPayment: Gilanra's {G} is spent on a
// payment that is no cast's (payFromPool: a tax, an ability cost). A creature
// source sets no poolArt bit, so only the pool spend itself can forget it; a
// Forest's {G} that floats afterwards and pays a cast is not Gilanra's.
func TestDeferredSourceForgottenOnOtherPayment(t *testing.T) {
	sc, id := deferredScen(t, "gilanra", "Gilanra, Caller of Wirewood", "Grizzly Bears", "Forest")
	gil, bears := id("Gilanra, Caller of Wirewood"), id("Grizzly Bears")
	g := sc.perm(gil, 0)
	sc.s.floatDeferred(sc.k, 0, sc.slot(g))
	if sc.s.g.defSrc[0] == 0 || sc.s.pl[0].pool[kop.G] != 1 || sc.s.g.poolArt[0] != 0 {
		t.Fatalf("precondition: defSrc %d, pool G %d, poolArt %d", sc.s.g.defSrc[0], sc.s.pl[0].pool[kop.G], sc.s.g.poolArt[0])
	}
	tax := kop.ManaCost{Generic: 1}
	sc.s.payFromPool(0, &tax, 0)
	if sc.s.g.defSrc[0] != 0 {
		t.Fatalf("defSrc %d after the source's {G} was spent: want forgotten", sc.s.g.defSrc[0])
	}
	sc.s.pl[0].pool[kop.G] += 2 // Forests' mana, floated in the window
	o := sc.obj(bears, 0)
	sc.kcastBuild(o, bears, 0)
	cost := sc.k.t.Cards[bears].Cost
	sc.s.payRider(sc.k, 0, &cost, 0, o, false)
	if got := sc.s.bld().obsSl; got != 0 {
		t.Fatalf("obsSl %d: a Forest's mana paid the cast, not Gilanra's", got)
	}
}
