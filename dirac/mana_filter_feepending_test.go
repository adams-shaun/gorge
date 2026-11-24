package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// feependingBoard is the fee-carrying combination filter beside a row of
// basics: five green sources, one black, a Baxter Building shape ({T}: Add
// {C}; {4},{T}: Add four mana in any combination of colors), and a hand spell
// costing {2}{B}{B}{G} (The Coming of Galactus). Only the filter's fee-funded
// activation makes the second {B}, so the plain census cannot price the cast.
func feependingBoard(t *testing.T) (*scen, CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}}
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	forest := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Forest", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1},
	}})
	swamp := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Swamp", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColB, ManaN: 1},
	}})
	baxter := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Baxter", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 4}}, ManaColors: kop.ColAny, ManaN: 4, AFlags: kop.AfManaChoice},
	}})
	cost := kop.ManaCost{Generic: 2}
	cost.Pips[kop.B] = 2
	cost.Pips[kop.G] = 1
	spell := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Saga", Types: kop.TInstant, Cost: cost})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	for i := 0; i < 5; i++ {
		sc.perm(forest, 0)
	}
	sc.perm(swamp, 0)
	sc.perm(baxter, 0)
	sc.hand(spell, 0)
	return sc, baxter
}

// TestFeePendingComboFilterFeeBranchReached: a fee-carrying combination filter beside a
// row of identical basics otherwise spends the search's node budget ordering
// the basics — the fee-carrying source's dead {C} answer alone costs a full
// funding subtree at every node, none of whose states dedup (the key reads
// the tapped set) — and its fee branch is never reached: the cast is dropped
// while gorge offers it (The Coming of Galactus, pair 129). The fee branch
// must be reached within the budget, as gorge's search reaches it.
func TestFeePendingComboFilterFeeBranchReached(t *testing.T) {
	sc, baxter := feependingBoard(t)
	s, k := sc.s, sc.k
	var bsl Slot
	for b := s.ctrlM[0]; b.any(); b = b.rest() {
		if s.cardAt(b.first()) == baxter {
			bsl = b.first()
		}
	}
	if !k.gen.has(baxter, gfFilt) {
		t.Fatal("precondition: the {4} combination is not the modelled filter")
	}
	if !s.scriptFilt(k, 0).has(bsl) {
		t.Fatal("precondition: the filter gap mask does not hold the source")
	}
	var spellO Obj
	for o := Obj(1); int(o) < len(s.objs); o++ {
		if s.objs[o].zone == kop.ZoneHand {
			spellO = o
		}
	}
	a := MakeAction(AkCast, TCode(spellO), 0, 0)
	var gaps Decision
	s.plainGapsOptions(k, &gaps, s.scriptFilt(k, 0))
	if !hasOption(&gaps, a) {
		t.Fatal("precondition: the gaps walk does not list the cast")
	}
	var d, capSpent Decision
	s.scriptOffers(k, &d, &capSpent, 0)
	if !hasOption(&d, a) {
		t.Fatalf("the offer walk dropped the cast whose fee branch funds it (stalled=%v reason=%s)",
			s.Stalled(), StallName(s.StallReason()))
	}
}
