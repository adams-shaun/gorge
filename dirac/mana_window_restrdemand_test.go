package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// restrDemandScen: p0 has Temur Devotee (a first-only filter, its Combo printed
// G U R) and a Test Heirloom ("{T}: Add {U}. Spend this mana only to cast a
// creature spell") on its battlefield, the Heirloom's mana floats restricted
// (State.rb), and a spell build of a creature card is live (s.ctl ctlBuild), so
// the payment may spend the batch (CR 106.6). The returned sc has the pool
// {C:1} (the fee the filter is about to take) and the batch floating. The
// Heirloom is appended to the manaabmanacost table: the pool holds the Devotees
// but no restricted mana source.
func restrDemandScen(t *testing.T) (sc *scen, dev, heirl Obj) {
	t.Helper()
	set := cardsettest.Set(t, "manaabmanacost")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	spendOn := kop.FilterID(len(tbl.Filters) - 1)
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the manaabmanacost pool", name)
		}
		return CardID(set.PoolID(i))
	}
	heirloom := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Heirloom", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColU, ManaN: 1, ManaRestrict: spendOn},
	}})
	sc = newScenTableLib(t, &tbl, id("Plains"), Config{NoElide: true})
	s := sc.s
	dev = sc.perm(id("Temur Devotee"), 0)
	heirl = sc.perm(heirloom, 0)
	devID := id("Temur Devotee")
	if !sc.k.gen.has(devID, gfFilt) || !s.filtFirstOnly(sc.k, sc.slot(dev)) || !s.filtUsable(sc.k, sc.slot(dev)) {
		t.Fatal("precondition: the Devotee is not a usable first-only filter")
	}
	if s.filtFee(sc.k, sc.slot(dev)) != 1 {
		t.Fatal("precondition: the Devotee's fee is not {1}")
	}
	ab := &tbl.Cards[devID].Abilities[sc.k.gen.filt(devID)]
	if ab.ManaColOrd != 204 { // G, U, R in printed order
		t.Fatalf("precondition: Temur Devotee's Combo order %d, want G U R (204)", ab.ManaColOrd)
	}
	if !s.windowRestr(sc.k, 0).has(sc.slot(heirl)) {
		t.Fatal("precondition: the Heirloom is not a restricted window gap")
	}
	// The Heirloom's mana floats restricted as {U} (State.rb).
	c := s.cardAt(sc.slot(heirl))
	rab := &sc.k.t.Cards[c].Abilities[sc.k.gen.restr(c)]
	if !s.floatRestrAb(sc.k, 0, sc.slot(heirl), rab, int(kop.U), 1) {
		t.Fatal("precondition: the Heirloom's mana did not float restricted")
	}
	if s.restrN(0) != 1 || s.restrOf(0, int(kop.U)) != 1 {
		t.Fatalf("precondition: restricted %v, want one {U}", s.rb[0])
	}
	// A spell build of a creature card, which the Heirloom's restriction
	// admits: the payment may spend the batch.
	*s.bld() = build{e: stackEntry{kind: seSpell, card: id("Grizzly Bears"), ctrl: 0, obj: heirl}}
	s.ctl = ctlBuild
	if s.restrPoolAdmits(sc.k, 0) == 0 {
		t.Fatal("precondition: the batch is not admitted for the build's payment")
	}
	s.pl[0].pool[kop.C] = 1
	return sc, dev, heirl
}

// TestWindowOwedColourReadsTheRestrictedPool: a window's owed colour ask prices
// the pool the payment may spend (gorge paymentUsefulColours reads
// pay.AvailableFor, CR 106.6). The payment cost {U}{R} sees the Heirloom's
// restricted {U} beside the raw pool's {C}, so the {U} pip is covered and only
// {R} is owed; Temur Devotee's Combo asks G, U, R in printed order and the
// window activates it for {R}. A raw pool read (which cannot see the batch)
// answers the {U} first and strands the cast.
func TestWindowOwedColourReadsTheRestrictedPool(t *testing.T) {
	sc, dev, _ := restrDemandScen(t)
	s := sc.s
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.U: 1, kop.R: 1}}
	s.windowFiltFirst(sc.k, 0, sc.slot(dev), &cost, false)
	if s.perms[sc.slot(dev)].actUsed&(1<<uint(sc.k.gen.filt(s.cardAt(sc.slot(dev))))) == 0 {
		t.Fatal("the Devotee's once-each-turn use was not spent")
	}
	if s.restrN(0) != 1 {
		t.Fatal("the filter's fee spent the restricted mana")
	}
	if s.pl[0].pool != ([kop.NumManaKinds]uint8{kop.R: 1}) {
		t.Fatalf("pool %v after the filter, want the fee off {C} and {R} made", s.pl[0].pool)
	}
}

// TestWindowDryFilterOwedColourReadsTheRestrictedPool: the same payment on the
// dry window walk (gorge castWindowReachable): the fee comes off the {C}, the
// batch stays, and the colour the filter makes is the first owed one of the
// printed order, {R} -- not the raw pool's shortfall {U}.
func TestWindowDryFilterOwedColourReadsTheRestrictedPool(t *testing.T) {
	sc, dev, _ := restrDemandScen(t)
	s := sc.s
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.U: 1, kop.R: 1}}
	s.windowDryFilter(sc.k, 0, &cost, sc.slot(dev))
	if s.pl[0].pool != ([kop.NumManaKinds]uint8{kop.R: 1}) {
		t.Fatalf("pool %v after the dry filter, want the fee off {C} and {R} made", s.pl[0].pool)
	}
}
