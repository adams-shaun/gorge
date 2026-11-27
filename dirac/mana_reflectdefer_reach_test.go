package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestReflectDeferExtraReach: a ProduceMana replacement's extra (Virtue of
// Strength, StTapReflectDefer over basic lands) floats at a window tap
// (addGatedExtra) only for a source its filter reaches. A basic Forest gains
// the static's B more green; a nonbasic land making {G} gains nothing.
func TestReflectDeferExtraReach(t *testing.T) {
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TBasic, Ctrl: kop.CtrlYou})
	basic := kop.FilterID(len(tbl.Filters) - 1)
	grove := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Grove", Types: kop.TLand, Abilities: []kop.Ability{{
		Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1,
	}}})
	virtue := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Virtue", Types: kop.TEnchantment, Abilities: []kop.Ability{{
		Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 2, StFlags: kop.StTapReflectDefer, Filter: basic},
	}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	forest := sc.slot(sc.perm(v.Forest, 0))
	nonbasic := sc.slot(sc.perm(grove, 0))
	sc.perm(virtue, 0)
	if !sc.k.gen.anyTapExtra {
		t.Fatal("precondition: the table has no tap-extra carrier")
	}
	g := int(kop.G)
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{}
	sc.s.addGatedExtra(sc.k, 0, forest, 0)
	if got := sc.s.pl[0].pool[g]; got != 2 {
		t.Fatalf("a basic Forest's window tap floated %d green extra, want 2", got)
	}
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{}
	sc.s.addGatedExtra(sc.k, 0, nonbasic, 0)
	if pool := sc.s.pl[0].pool; pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("a nonbasic land's window tap floated extra %v, want none", pool)
	}
}
