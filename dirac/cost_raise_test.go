package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRaiseCost: a generated StatIncreaseCost (gorge RaiseCost) adds its
// generic mana to the matching spells of the casters its Activator$ names
// (StRaiseOpp: the controller's opponents), before the reductions' floor.
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
	if !sc.k.gen.anyRaise {
		t.Fatal("anyRaise false, want true")
	}
	sc.perm(tax, 0)
	base := kop.ManaCost{Generic: 1}
	if g := sc.s.adjustCostGen(sc.k, 1, spell, NoObj, base).Generic; g != 3 {
		t.Fatalf("the opponent's spell costs %d generic, want 3", g)
	}
	if g := sc.s.adjustCostGen(sc.k, 0, spell, NoObj, base).Generic; g != 1 {
		t.Fatalf("the controller's own spell costs %d generic, want 1", g)
	}
	// a spell cast without paying its mana cost owes the raise, a generic on
	// the empty cost (CR 601.2f, 118.9d), and the offer withholds it when the
	// floating pool cannot pay that
	o := sc.hand(spell, 1)
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: spell, ctrl: 1, srcVer: sc.s.objs[o].ver, flags: sfFree}}
	sc.s.bld().e.tgt = noTargets
	if c := sc.s.buildCost0(sc.k); c != (kop.ManaCost{Generic: 2}) {
		t.Fatalf("the opponent's free spell owes %+v, want {2}", c)
	}
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: spell, ctrl: 0, srcVer: sc.s.objs[o].ver, flags: sfFree}}
	sc.s.bld().e.tgt = noTargets
	if c := sc.s.buildCost0(sc.k); c != (kop.ManaCost{}) {
		t.Fatalf("the controller's free spell owes %+v, want nothing", c)
	}
	*sc.s.bld() = build{}
}
