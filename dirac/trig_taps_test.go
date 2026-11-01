package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsTrigger: "whenever this becomes tapped" (gorge Mode$ Taps) fires
// when it attacks (after the attack triggers) and not for a vigilant or
// already-tapped one; a table that also grants a mana ability stalls at
// the start (StallTapsMana: a mana tap would not fire it).
func TestTapsTrigger(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	tapper := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Tapper", Types: kop.TCreature, Power: 2, Toughness: 2,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigTaps, Flags: kop.TfSelf}, Prog: gain}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	a1, a2 := sc.perm(tapper, 0), sc.perm(tapper, 0)
	sc.s.perms[sc.slot(a2)].flags |= pfTapped
	sc.combat(t, []Obj{a1}, nil)
	if l0, l1 := sc.s.pl[0].life, sc.s.pl[1].life; l0 != 21 || l1 != 18 {
		t.Fatalf("life %d / %d, want 21 / 18 (one Taps trigger)", l0, l1)
	}

	tbl2, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl2.Cards = append(tbl2.Cards, tbl.Cards[tapper],
		kop.CardDef{Name: "Test Rite", Types: kop.TEnchantment, Abilities: []kop.Ability{
			{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1}}})
	var s State
	lib := make([]CardID, 20)
	s.Init(tbl2, lib, lib, 1, Config{})
	if s.StallReason() != StallTapsMana {
		t.Fatalf("stall %d, want StallTapsMana", s.StallReason())
	}
}
