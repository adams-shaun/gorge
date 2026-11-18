package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// plainRaiseRestrScen: p0 has two Temur Devotees ("{1}: Add {G}, {U} or {R}",
// once each turn: a first-only filter the planner cannot chain, the CR 601.2g
// window can), two {C} floating for their fees, and a Test Heirloom ("{T}: Add
// {U}. Spend this mana only to cast a creature spell") whose {U} floats
// restricted (State.rb). The cast being built is a Test Raiser, a creature spell
// {G}{U} with "costs {U} more for each target beyond the first" and two targets,
// so it costs {G}{U}{U}: the floating {U} pays one pip and the two Devotees the
// other two.
func plainRaiseRestrScen(t *testing.T) (sc *scen, ent *stackEntry) {
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
	raiser := CardID(len(tbl.Cards))
	guu := kop.ManaCost{}
	guu.Pips[kop.G], guu.Pips[kop.U] = 1, 2
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Raiser", Types: kop.TCreature, Cost: guu})
	sc = newScenTableLib(t, &tbl, id("Plains"), Config{NoElide: true})
	s := sc.s
	sc.perm(id("Temur Devotee"), 0)
	sc.perm(id("Temur Devotee"), 0)
	first := sc.perm(heirloom, 0)
	o := sc.hand(raiser, 0)
	c := s.cardAt(sc.slot(first))
	rab := &sc.k.t.Cards[c].Abilities[sc.k.gen.restr(c)]
	if !s.floatRestrAb(sc.k, 0, sc.slot(first), rab, int(kop.U), 1) {
		t.Fatal("precondition: the Heirloom's mana did not float restricted")
	}
	*s.bld() = build{e: stackEntry{kind: seSpell, card: raiser, ctrl: 0, obj: o}}
	s.ctl = ctlBuild
	e := &s.bld().e
	e.tgt = noTargets
	e.nt, e.tgt[0], e.tgt[1] = 2, TCode(PlayerTarget0), TCode(PlayerTarget0+1)
	if s.raisePerTargetCharge(raiser, 0) != 1 { // two targets: the plain arm tries the planner, then the window
		t.Fatalf("precondition: raise charges %d, want 1", s.raisePerTargetCharge(raiser, 0))
	}
	if s.restrPoolAdmits(sc.k, 0) == 0 || s.restrOf(0, int(kop.U)) != 1 {
		t.Fatalf("precondition: restricted %v not admitted for the Raiser", s.rb[0])
	}
	s.pl[0].pool[kop.C] = 2 // the filters' fees
	return sc, e
}

// TestTapForSpellPlainWindowFallbackCountsRestrictedOnce: the plain cast's mana
// step with two targets chosen (the per-target raise's guard, raisePerTargetCharge)
// and no planner witness falls to the CR 601.2g window, whose settle discounts the
// admitted floating restricted mana itself; the step hands it the unshortened cost
// so that mana is counted once (CR 106.6). The cost {G}{U}{U} owes the Devotees'
// {G} and {U} beside the floating {U}: counting the float twice (the shortened
// cost {G}{U} discounted again) settles after one Devotee and leaves the other
// untapped.
func TestTapForSpellPlainWindowFallbackCountsRestrictedOnce(t *testing.T) {
	sc, e := plainRaiseRestrScen(t)
	s := sc.s
	cost := s.buildCost(sc.k)
	if cost.Pips[kop.G] != 1 || cost.Pips[kop.U] != 2 || cost.Generic != 0 {
		t.Fatalf("precondition: the build costs %+v, want {G}{U}{U}", cost)
	}
	short := s.restrAdmShortCost(sc.k, e, cost)
	if short == cost {
		t.Fatal("precondition: the admitted float does not shorten the cost")
	}
	f := s.fork()
	if f.tapForTry(sc.k, 0, &short, int(e.x), e.obj, false) {
		t.Fatal("precondition: the planner has a witness for the taxed cost, the fallback is not reached")
	}
	if s.bld().bfl&bflLowered != 0 {
		t.Fatal("precondition: the cast is lowered, the plain arm is not reached")
	}
	if s.manaStep(sc.k) {
		t.Fatal("the cast was reversed: the window funds the payment")
	}
	if got := s.pl[0].pool; got[kop.G] != 1 || got[kop.U] != 1 {
		t.Fatalf("pool %v after the window, want the Devotees' {G} and {U}: the floating {U} was counted twice", got)
	}
}
