package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWindowPrintedManaLockedByCantAct: Petrify's CantBeActivated static
// locks a Powerstone token whose only mana ability is printed and restricted
// (flags gfRestr|gfRestrSrc, eachCol -1). gapMask filters such a source out
// of the window's restricted mask; windowGap must read the same lock
// (cast_cantactivate.go) or windowGapListed lists the token, tapGap taps it
// and eachFee indexes Abilities[-1] ("index out of range [-1]", tail-35 pair
// 253 decision 671).
func TestWindowPrintedManaLockedByCantAct(t *testing.T) {
	set := cardsettest.Set(t, "cantact")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append(append([]kop.Filter(nil), tbl0.Filters...), kop.Filter{Types: kop.TArtifact})
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the cantact pool", name)
		}
		return CardID(set.PoolID(i))
	}
	stone := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Powerstone", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, AFlags: kop.AfManaNonSpell, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColC, ManaN: 1, ManaRestrict: kop.FilterID(len(tbl.Filters) - 1)}}})
	sc := newScenTableLib(t, &tbl, id("Plains"), Config{NoElide: true})
	o := sc.perm(stone, 0)
	sl := sc.slot(o)
	if !sc.k.gen.has(stone, gfRestr) || sc.k.gen.eachCol(stone) != -1 || !sc.s.windowGap(sc.k, 0, sl) {
		t.Fatal("precondition: restricted artifact not a window gap")
	}
	sc.wear(id("Petrify"), o)
	if !sc.s.actForbidden(sc.k, sl, true) {
		t.Fatal("precondition: no CantBeActivated lock")
	}
	if sc.s.windowGap(sc.k, 0, sl) {
		t.Error("window lists a locked restricted source")
	}
}
