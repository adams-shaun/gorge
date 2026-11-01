package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFlycardMatch pins matchCardOne's withFlying / withoutFlying read on
// a card off the battlefield (cardFlyingOK): gorge's matchesSpec binds the
// object's Derived keywords, the printed ones plus every battlefield
// Continuous grant whose Affected$ spec matches the card where it is
// (FeatStaticPT). Before, the card matcher refused withFlying outright, so
// Momo, Friendly Flier's Count$ThisTurnCast_Creature.nonLemur+withFlying
// could not count a flyer still on the stack.
func TestFlycardMatch(t *testing.T) {
	set := cardsettest.Set(t, "thisturncast")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
	filter := func(f kop.Filter) kop.FilterID {
		tbl.Filters = append(tbl.Filters, f)
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	with := filter(kop.Filter{Types: kop.TCreature, Flags2: kop.F2WithFlying})
	without := filter(kop.Filter{Types: kop.TCreature, Flags2: kop.F2WithoutFlying})
	youWith := filter(kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Flags2: kop.F2WithFlying})
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the pool", name)
		}
		return CardID(set.PoolID(i))
	}
	sphinx, bears, crab := id("Stoic Sphinx"), id("Grizzly Bears"), id("Canyon Crab")
	// Canyon Crab, for the test: "Creatures you control have flying"
	cd := &tbl.Cards[crab]
	cd.Abilities = append(append([]kop.Ability(nil), cd.Abilities...), kop.Ability{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatContinuous, Filter: filter(kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou}), Kw: kop.KwFlying}})
	sc := newScenTable(t, &tbl, Config{})
	s, k := sc.s, sc.k
	if k.cards[sphinx].kw&kop.KwFlying == 0 || k.cards[bears].kw&kop.KwFlying != 0 {
		t.Fatal("Stoic Sphinx must print flying and Grizzly Bears must not")
	}
	m := func(f kop.FilterID, o Obj, ctrl uint8) bool {
		ob := &s.objs[o]
		return s.matchCardGen(k, f, ob.card, ob.owner(), ctrl, o)
	}
	sph, bea, bea1 := sc.hand(sphinx, 0), sc.hand(bears, 0), sc.hand(bears, 1)
	for _, c := range []struct {
		name string
		f    kop.FilterID
		o    Obj
		want bool
	}{
		{"printed flyer withFlying", with, sph, true},
		{"printed flyer withoutFlying", without, sph, false},
		{"ground creature withFlying", with, bea, false},
		{"ground creature withoutFlying", without, bea, true},
	} {
		if got := m(c.f, c.o, 0); got != c.want {
			t.Fatalf("%s (no grant): got %v, want %v", c.name, got, c.want)
		}
	}
	if !s.matchCardGen(k, with, sphinx, 0, 0, NoObj) || s.matchCardGen(k, with, bears, 0, 0, NoObj) {
		t.Fatal("an unknown object reads the printed keywords")
	}
	sc.perm(crab, 0)
	if !m(with, bea, 0) || m(without, bea, 0) {
		t.Fatal("a hand creature under a battlefield flying grant must read as a flyer (gorge Derived)")
	}
	if m(with, bea1, 1) || !m(without, bea1, 1) {
		t.Fatal("the opponent's hand creature is outside the YouCtrl grant")
	}
	if !m(youWith, bea, 0) || m(youWith, bea, 1) {
		t.Fatal("YouCtrl+withFlying reads both the controller and the grant")
	}
}
