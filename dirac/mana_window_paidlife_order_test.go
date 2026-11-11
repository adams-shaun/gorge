package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// paidLifeScen: the town pool (Starting Town) with two test cards appended,
// a Test Confluence land ("{T}, Pay 1 life: Add one mana of any color", its
// only mana ability) and a Test Druid creature ("{T}, Pay 1 life: Add {G}",
// its only mana ability). p0 is at life 2, in an unless / ward window's
// affordability (availWin), where gorge's paid-cost layer
// (appendPaidManaUnits) walks the battlefield in zone order and counts a
// paid-life alternative only while life - committed > its cost.
func paidLifeScen(t *testing.T) (sc *scen, id func(string) CardID, conf, druid CardID) {
	t.Helper()
	set := cardsettest.Set(t, "town")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	id = func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the town pool", name)
		}
		return CardID(set.PoolID(i))
	}
	life := kop.Cost{Flags: kop.CostTap | kop.CostPayLife, N: 1}
	conf = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Confluence", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: life, ManaColors: kop.ColAny, ManaN: 1},
	}})
	druid = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Druid", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: life, ManaColors: kop.ColG, ManaN: 1},
	}})
	sc = newScenTableLib(t, &tbl, id("Forest"), Config{NoElide: true})
	if !sc.k.painMana {
		t.Fatal("precondition: the pool's painful abilities are not modelled (k.painMana)")
	}
	if pc := &sc.k.cards[conf]; !pc.painOnly || !pc.painLife || pc.painN != 1 {
		t.Fatalf("precondition: Test Confluence is not a paid-life pain-only source: %+v", *pc)
	}
	sc.s.pl[0].life = 2
	return sc, id, conf, druid
}

// windowMask is painAvailMaskAt under the unless / ward window's mode.
func (sc *scen) windowMask(o Obj) uint8 {
	s := sc.s
	x := s.g.availX
	s.g.availX |= availWin
	defer func() { s.g.availX = x }()
	sl := sc.slot(o)
	return s.painAvailMaskAt(sc.k, sl, s.manaPCAt(sc.k, sl), 0)
}

// TestWindowPaidLifeStartingTownCommitsLife: at life 2, Starting Town (in
// play first) then Mana Confluence. Gorge's paid-cost layer counts Starting
// Town's "{T}, Pay 1 life" alternative (2 - 0 > 1) and commits its 1 life, so
// Confluence (2 - 1 > 1 fails) is left out: an unless {2} is unaffordable.
// Skipping Starting Town's life counted Confluence.
func TestWindowPaidLifeStartingTownCommitsLife(t *testing.T) {
	sc, id, conf, _ := paidLifeScen(t)
	town := sc.perm(id("Starting Town"), 0)
	cf := sc.perm(conf, 0)
	if pc := sc.s.manaPCAt(sc.k, sc.slot(town)); !pc.painLife || pc.painOnly || pc.painN != 1 {
		t.Fatalf("precondition: Starting Town is not a paid-life painland: %+v", *pc)
	}
	if sc.s.perms[sc.slot(town)].ts >= sc.s.perms[sc.slot(cf)].ts {
		t.Fatal("precondition: Starting Town did not enter first")
	}
	if m := sc.windowMask(cf); m != 0 {
		t.Fatalf("Confluence counts as %#x in the window; gorge leaves it out (Starting Town committed 1 of 2 life)", m)
	}
}

// TestWindowPaidLifeZoneOrderNotSlotOrder: at life 2, Test Druid ({G} for 1
// life) enters, then Test Confluence takes a lower, freed slot. Gorge walks
// zone order: the Druid counts ({G}, commits 1) and Confluence is left out,
// so an unless {U} is unaffordable. A slot-order walk counted Confluence (any
// colour) and left the Druid out.
func TestWindowPaidLifeZoneOrderNotSlotOrder(t *testing.T) {
	sc, id, conf, druid := paidLifeScen(t)
	filler := sc.perm(id("Forest"), 0)
	dr := sc.perm(druid, 0)
	sc.s.destroy(sc.k, sc.slot(filler))
	sc.s.npend = 0
	if sc.onBF(filler) {
		t.Fatal("precondition: the filler is still on the battlefield")
	}
	cf := sc.perm(conf, 0)
	if sc.slot(cf) >= sc.slot(dr) {
		t.Fatalf("precondition: Confluence slot %d not below the Druid's %d", sc.slot(cf), sc.slot(dr))
	}
	if sc.s.perms[sc.slot(dr)].ts >= sc.s.perms[sc.slot(cf)].ts {
		t.Fatal("precondition: the Druid did not enter first")
	}
	if m := sc.windowMask(dr); m != 1<<kop.G {
		t.Fatalf("the Druid counts as %#x in the window, want {G} (%#x)", m, uint8(1<<kop.G))
	}
	if m := sc.windowMask(cf); m != 0 {
		t.Fatalf("Confluence counts as %#x in the window; gorge leaves it out (the Druid committed 1 of 2 life)", m)
	}
}

// TestPaidLifeSourceUnpayableBelowItsLife: CR 119.4, a player can pay life
// only if their life total is at least the payment. At life 1 Test Confluence
// can pay its 1 life; at life 0 it cannot be activated, so an unless window
// that took one Confluence to 0 life does not tap a second one (gorge leaves
// it out of the window's options; the kernel tapped it to -1: manaconfluence
// seed 2769).
func TestPaidLifeSourceUnpayableBelowItsLife(t *testing.T) {
	sc, _, conf, _ := paidLifeScen(t)
	cf := sc.perm(conf, 0)
	sc.s.pl[0].life = 1
	if !sc.s.canTapForMana(sc.k, sc.slot(cf)) {
		t.Fatal("precondition: at life 1 Test Confluence cannot be tapped")
	}
	sc.s.pl[0].life = 0
	if sc.s.canTapForMana(sc.k, sc.slot(cf)) {
		t.Fatal("at life 0 Test Confluence can still be tapped for its 1 life (CR 119.4)")
	}
}
