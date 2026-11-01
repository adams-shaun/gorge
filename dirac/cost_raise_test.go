package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRaiseCost: a generated StatIncreaseCost (gorge RaiseCost) adds its
// generic mana to the matching spells of the casters its Activator$ names
// (StRaiseOpp: the controller's opponents), before the reductions' floor;
// a table that also casts for free stalls loud (StallRaiseFree).
func TestRaiseCost(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tax, spell := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Tax", Types: kop.TEnchantment, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatIncreaseCost, A: 2, StFlags: kop.StRaiseOpp}}}},
		kop.CardDef{Name: "Test Spell", Types: kop.TSorcery, Cost: kop.ManaCost{Generic: 1}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyRaise || sc.k.gen.raiseFree {
		t.Fatalf("anyRaise %v raiseFree %v, want true false", sc.k.gen.anyRaise, sc.k.gen.raiseFree)
	}
	sc.perm(tax, 0)
	base := kop.ManaCost{Generic: 1}
	if g := sc.s.adjustCostGen(sc.k, 1, spell, NoObj, base).Generic; g != 3 {
		t.Fatalf("the opponent's spell costs %d generic, want 3", g)
	}
	if g := sc.s.adjustCostGen(sc.k, 0, spell, NoObj, base).Generic; g != 1 {
		t.Fatalf("the controller's own spell costs %d generic, want 1", g)
	}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Free", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: int16(kop.ZoneExile)}}}}})
	if _, free := tableRaises(tbl); !free {
		t.Fatal("a raise beside OpCastFree: want raiseFree")
	}
}
