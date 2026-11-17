package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// powerstoneUnlessTable is vanilla's table with a Powerstone-shaped card
// appended: an untapped artifact whose only ability is "{T}: Add {C}. This
// mana can't be spent to cast a nonartifact spell." Vanilla's table needs no
// gen prep, so the card keeps its census ability. (The real table's game is
// the oracle's lockstep repro powerstone_unless_payable: archtest keeps raw
// card tables to the lockstep oracle and kgen.)
func powerstoneUnlessTable() *kop.Table {
	tbl := *v.Table
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TArtifact})
	fid := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Powerstone",
		Types: kop.TArtifact,
		Abilities: []kop.Ability{{
			Kind:         kop.AbMana,
			Cost:         kop.Cost{Flags: kop.CostTap},
			ManaColors:   kop.ColC,
			ManaN:        1,
			ManaRestrict: fid,
			AFlags:       kop.AfManaNonSpell,
		}},
	})
	return &tbl
}

// TestUnlessPowerstoneWindow pins the plain census: a Powerstone-shaped
// source funds a non-spell payment (gorge prices an Energy Flux-style
// "{2} or sacrifice" unless payable from two untapped Powerstones, {C}{C}).
func TestUnlessPowerstoneWindow(t *testing.T) {
	tbl := powerstoneUnlessTable()
	sc := newScenTable(t, tbl, Config{NoElide: true})
	pst := kop.CardID(len(tbl.Cards) - 1)
	if name := sc.k.t.Cards[pst].Name; name != "Test Powerstone" {
		t.Fatalf("precondition: appended card is %q", name)
	}
	sc.perm(pst, 0)
	sc.perm(pst, 0)
	if !sc.s.canPayGen(sc.k, 0, kop.PayMana, 2, 0, 0) {
		t.Fatal("the kernel prices the {2} unless unpayable beside two untapped {C} sources")
	}
}
