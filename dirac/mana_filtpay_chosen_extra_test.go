package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// chosenExtraScen: a colourless tap source carries a Produced$ Chosen
// TapsForMana extra chosen red (a synthetic StatTapExtra static), beside a
// first-only filter ("{1}, {T}: Add one mana of any color"). The source's own
// {C} pays the filter's {1}; the filter makes one {R}; the chosen extra is the
// only other red, so an unless {R}{R} is payable only when the extra is
// counted.
func chosenExtraScen(t *testing.T) (*scen, Slot, Slot) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	mine := kop.FilterID(len(tbl.Filters) - 1)
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Extra Source", Types: kop.TLand, Abilities: []kop.Ability{{
		Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1,
	}}})
	static := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Chosen Extra", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{{
		Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, A: int16(kop.R), B: 1, StFlags: kop.StTapChosen, Filter: mine},
	}}})
	filter := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Filter", Types: kop.TArtifact, Abilities: []kop.Ability{{
		Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 1}}, ManaColors: kop.ColAny,
	}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sl := sc.slot(sc.perm(src, 0))
	st := sc.slot(sc.perm(static, 0))
	sc.s.perms[st].chosen = objWord(kop.R)
	fsl := sc.slot(sc.perm(filter, 0))
	if !sc.k.gen.anyTapExtra {
		t.Fatal("precondition: the table has no tap-extra carrier")
	}
	if !sc.k.gen.anyFiltFirst {
		t.Fatal("precondition: the table lists no first-only filter")
	}
	if !sc.s.scriptFilt(sc.k, 0).has(fsl) {
		t.Fatal("precondition: the Test Filter is no live census gap")
	}
	return sc, sl, fsl
}

// TestFiltPaySkipsChosenTapExtra: the unless census prices a source's own
// mana, never a Produced$ Chosen TapsForMana trigger's extra (addGatedExtra
// skips it under pool.census), so the filter payment probe must not count the
// chosen red. A second red source is the census control.
func TestFiltPaySkipsChosenTapExtra(t *testing.T) {
	sc, _, _ := chosenExtraScen(t)
	cost := manaOf(0, 2<<(3*kop.R)) // {R}{R}
	if ok, _ := sc.s.filtPay(sc.k, 0, &cost, true); ok {
		t.Fatal("the payment probe counted the Produced$ Chosen tap extra")
	}
	// The census control: a Mountain makes the second red.
	sc.perm(v.Mountain, 0)
	if ok, _ := sc.s.filtPay(sc.k, 0, &cost, true); !ok {
		t.Fatal("the payment probe did not pay {R}{R} beside a Mountain")
	}
}
