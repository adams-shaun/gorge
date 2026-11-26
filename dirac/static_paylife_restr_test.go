package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPayLifeGrantRestrictedPayers: the grant's life and the planner's discount
// for it are one unit on the restricted-spend payers too (payRestrSpend,
// payRestrX). A floating {W} restricted to creature spells pays the {1} of a
// {1}{B} creature spell; no source can pay the {B}, the plan leaves it off the
// taps (tapNeed's grant discount) and the grant's 2 life pays it. The payers
// once paid the unreduced cost from a pool with no {B} and panicked.
func TestPayLifeGrantRestrictedPayers(t *testing.T) {
	set := cardsettest.Set(t, "staticzones")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	spendOn := kop.FilterID(len(tbl.Filters) - 1)
	krrik := CardID(0)
	found := false
	for c := range tbl.Cards {
		if tbl.Cards[c].Name == "K'rrik, Son of Yawgmoth" {
			krrik, found = CardID(c), true
			break
		}
	}
	if !found {
		t.Fatal("K'rrik not in the staticzones table")
	}
	heirloom := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Heirloom", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColW, ManaN: 1, ManaRestrict: spendOn},
	}})
	payers := []struct {
		name string
		pay  func(sc *scen, m uint8, cost *kop.ManaCost)
	}{
		{"payRestrSpend", func(sc *scen, m uint8, cost *kop.ManaCost) {
			sc.s.payRestrSpend(sc.k, 0, m, cost, 0, NoObj, false)
		}},
		{"payRestrX", func(sc *scen, m uint8, cost *kop.ManaCost) {
			sc.s.payRestrX(sc.k, 0, m, cost, 0, NoObj, false)
		}},
	}
	for _, py := range payers {
		t.Run(py.name, func(t *testing.T) {
			sc := newScenTable(t, &tbl, Config{NoElide: true})
			sc.perm(krrik, 0)
			first := sc.perm(heirloom, 0)
			c := sc.s.cardAt(sc.slot(first))
			rab := &sc.k.t.Cards[c].Abilities[sc.k.gen.restr(c)]
			if !sc.s.floatRestrAb(sc.k, 0, sc.slot(first), rab, int(kop.W), 1) {
				t.Fatal("precondition: the Heirloom's mana did not float restricted")
			}
			m := sc.s.rbAll(0)
			if m == 0 {
				t.Fatal("precondition: no restricted batch floats")
			}
			var cost kop.ManaCost
			cost.Generic, cost.Pips[kop.B] = 1, 1
			if !sc.s.payLifeGrantApplies(sc.k, 0, &cost) {
				t.Fatal("precondition: the grant does not apply to the payer")
			}
			before := sc.s.pl[0].life
			py.pay(sc, m, &cost)
			if got := before - sc.s.pl[0].life; got != 2 {
				t.Errorf("life paid %d, want 2 (the {B} pip's life)", got)
			}
			if sc.s.rbAny(0) {
				t.Errorf("restricted mana %v left floating, want it spent", sc.s.rb[0])
			}
		})
	}
}
