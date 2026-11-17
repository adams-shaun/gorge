package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestGrantOnlySourceTwoManaOfAnyOneColourIsOneColour: a land with no mana
// ability of its own whose only mana is a granted "{T}: Add two mana of any one
// color" (Nature's Embrace on Maze of Ith) makes two mana of the ONE colour
// chosen (CR 605.1a), so {U}{R} is not payable from it alone while {U}{U} is.
func TestGrantOnlySourceTwoManaOfAnyOneColourIsOneColour(t *testing.T) {
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}}
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	lands := kop.FilterID(len(tbl.Filters) - 1)
	land := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Manaless Land", Types: kop.TLand})
	grant := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Two Of One Colour Grant", Types: kop.TEnchantment,
		Abilities: []kop.Ability{
			{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColAny, ManaN: 2},
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: lands, Grant: 1}},
		},
	})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.perm(grant, 0)
	src := sc.perm(land, 0)
	sl := sc.slot(src)
	if len(sc.k.gen.grantPC) == 0 || !sc.s.grantMay(sc.k, sl) || sc.s.ownManaUsable(sc.k, sl) {
		t.Fatal("precondition: the land is not a grant-only mana source")
	}
	var a avail
	sc.s.availFor(sc.k, 0, &a)
	if a.total != 2 {
		t.Fatalf("precondition: total = %d, want 2 (the grant's two mana)", a.total)
	}
	var ur, uu kop.ManaCost
	ur.Pips[kop.U], ur.Pips[kop.R] = 1, 1
	uu.Pips[kop.U] = 2
	if !a.affords(&uu, pipMaskOf(&uu), 0) {
		t.Fatalf("{U}{U} not affordable (total=%d nOne=%d)", a.total, a.nOne)
	}
	if a.affords(&ur, pipMaskOf(&ur), 0) {
		t.Fatalf("{U}{R} affordable from one grant-only tap (total=%d nOne=%d)", a.total, a.nOne)
	}
}
