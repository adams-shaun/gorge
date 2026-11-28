package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestZeroYieldRestrictedSourceLockedByCantAct: an admitted restricted mana
// ability that yields nothing leaves the window's census incomplete and is an
// untapped restricted source, but a CantBeActivated lock (Petrify) takes it
// off both reads: an ability that can't be activated is no source (CR 602.5).
func TestZeroYieldRestrictedSourceLockedByCantAct(t *testing.T) {
	set := cardsettest.Set(t, "cantact")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	yours := kop.FilterID(len(tbl.Filters) - 1)
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the cantact pool", name)
		}
		return CardID(set.PoolID(i))
	}
	idle := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Idle Spring", Types: kop.TArtifact, Abilities: []kop.Ability{
		// a dynamic amount that reads zero
		{Kind: kop.AbMana, AFlags: kop.AfManaDynamic, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColU,
			ManaRestrict: yours, ManaRestrictAb: yours},
	}})
	engine := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Engine", Types: kop.TCreature, Abilities: []kop.Ability{
		{Kind: kop.AbActivated, Cost: kop.Cost{Mana: kop.ManaCost{Generic: 1}}},
	}})
	sc := newScenTableLib(t, &tbl, id("Plains"), Config{NoElide: true})
	s := sc.s

	o := sc.perm(idle, 0)
	sl := sc.slot(o)
	eng := sc.perm(engine, 0)
	b := s.bld()
	b.e = stackEntry{kind: seAbility, card: engine, ab: 0, ctrl: 0, obj: eng, srcVer: s.objs[eng].ver, tgt: noTargets}
	s.ctl = ctlBuild
	s.pushBuild(sc.k)

	a := &sc.k.t.Cards[idle].Abilities[0]
	d := s.payDescNow(NoObj)
	if !s.restrSrcAdmits(sc.k, 0, sl, a, &d) || s.windowPricesRestr(sc.k, sl, a) {
		t.Fatalf("precondition: want an admitted, unpriced restricted ability (payment %+v)", d)
	}
	if !s.restrManaUntapped(sc.k, sl) || s.windowCensusComplete(sc.k, &b.e) {
		t.Fatal("precondition: the unlocked source is not open and unpriced")
	}
	sc.wear(id("Petrify"), o)
	if !s.actForbidden(sc.k, sl, true) {
		t.Fatal("precondition: no CantBeActivated lock")
	}
	if s.restrManaUntapped(sc.k, sl) {
		t.Error("a locked source still reads as an untapped restricted source")
	}
	if !s.windowCensusComplete(sc.k, &b.e) {
		t.Error("a locked source still leaves the census incomplete")
	}
}
