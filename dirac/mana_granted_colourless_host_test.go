package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// colourlessGrantScen: a land printing "{T}: Add {C} x ownN" under a static
// that grants every land you control "{T}: Add two mana of any one color"
// (Nature's Embrace over Trenchpost or Treasure Vault).
func colourlessGrantScen(t *testing.T, ownN uint8) (*scen, Slot) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	lands := kop.FilterID(len(tbl.Filters) - 1)
	vault := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Colourless Vault", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: ownN},
	}})
	rite := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test One-Colour Rite", Types: kop.TEnchantment, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: lands, Grant: 2}},
		{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColAny, ManaN: 2},
	}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.perm(rite, 0)
	land := sc.perm(vault, 0)
	sl := sc.slot(land)
	if !sc.s.ownManaUsable(sc.k, sl) {
		t.Fatalf("precondition: the vault's printed {C} ability (x%d) is not usable", ownN)
	}
	if sc.s.grantIdx(sc.k, sl) < 0 {
		t.Fatal("precondition: the one-colour grant does not reach the vault")
	}
	return sc, sl
}

// TestGrantedOneColourOverColourlessHost: a source printing "{T}: Add {C}"
// and granted "{T}: Add two mana of any one color" can make either its
// colourless mana or two mana of ONE colour, never both and never two
// colours. The merged grant must keep that one-colour limit even though the
// source's own mask ({C}) joins it -- the merged mask is 0x3f, not
// kop.ColAny -- while the printed {C} ability stays a live alternative, so a
// lone {C} is still payable (CR 106.4, 106.12a). The equal-amount case
// (ownN 2) is the same class: neither face may be counted beside the other.
func TestGrantedOneColourOverColourlessHost(t *testing.T) {
	for _, tc := range []struct {
		name string
		ownN uint8
		cc   bool
	}{
		{"one colourless mana", 1, false},
		{"two colourless mana", 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, _ := colourlessGrantScen(t, tc.ownN)
			var a avail
			sc.s.availFor(sc.k, 0, &a)
			if a.total != 2 || a.nOne != 1 {
				t.Fatalf("availability total=%d nOne=%d, want the merged tap's 2 and one one-colour source", a.total, a.nOne)
			}
			ub := cost(0, kop.U, kop.B)
			if a.affords(&ub, pipMaskOf(&ub), 0) {
				t.Fatalf("{U}{B} affordable from one {C} source plus a two-mana one-colour grant (total=%d nOne=%d): the grant makes two mana of one colour", a.total, a.nOne)
			}
			uu := cost(0, kop.U, kop.U)
			if !a.affords(&uu, pipMaskOf(&uu), 0) {
				t.Fatalf("{U}{U} not affordable from the two-mana one-colour grant (total=%d nOne=%d)", a.total, a.nOne)
			}
			c := cost(0, kop.C)
			if !a.affords(&c, pipMaskOf(&c), 0) {
				t.Fatalf("{C} not affordable through the vault's printed ability (total=%d nOne=%d)", a.total, a.nOne)
			}
			cu := cost(0, kop.C, kop.U)
			if a.affords(&cu, pipMaskOf(&cu), 0) {
				t.Fatalf("{C}{U} affordable: the source taps once, so the printed {C} and the grant are alternatives (total=%d nOne=%d)", a.total, a.nOne)
			}
			cc := cost(0, kop.C, kop.C)
			if got := a.affords(&cc, pipMaskOf(&cc), 0); got != tc.cc {
				t.Fatalf("{C}{C} affordable=%v, want %v (total=%d nOne=%d)", got, tc.cc, a.total, a.nOne)
			}
		})
	}
}
