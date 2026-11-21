package dirac

import (
	"errors"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// An attack tax refuses the matchup beside each part its walk does not
// model (combat_attacktax.go atkTaxGap), and plays beside a source of a
// shape it prices (atkTaxManaAb, atkTaxPaidAb) or never lists (atkTaxUnlisted).
func TestAttackTaxRefusesUnmodelledPartners(t *testing.T) {
	tap := kop.Cost{Flags: kop.CostTap}
	tax := kop.CardDef{Name: "Half Tax", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatAttackTax, A: 2, StFlags: kop.StAtkTaxPlayer}}}}
	land := func(name string, a kop.Ability) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TLand, Abilities: []kop.Ability{a}}
	}
	for _, c := range []struct {
		partner kop.CardDef
		refused bool
	}{
		{land("Half Fixed", kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColG, ManaN: 1}), false},
		{land("Half Choice", kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColW | kop.ColU, ManaN: 1, AFlags: kop.AfManaChoice}), false},
		{land("Half Treasure", kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostSacSelf}, ManaColors: kop.ColAny, ManaN: 1}), false},
		{land("Half Spawn", kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostSacSelf}, ManaColors: kop.ColC, ManaN: 1}), false},
		{land("Half Treasure Rider", kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostSacSelf}, ManaColors: kop.ColAny, ManaN: 1, ManaRider: kop.ManaRiderOppGainLife}), true},
		{land("Half Signet", kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 1}}, ManaColors: kop.ColW | kop.ColU, ManaN: 1, ManaEach: true}), false},
		{land("Half Bounce", kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColW | kop.ColU, ManaN: 1, ManaEach: true}), false},
		{land("Half Bounce Choice", kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColW | kop.ColU, ManaN: 1, ManaEach: true, AFlags: kop.AfManaChoice}), true},
		{kop.CardDef{Name: "Half Fee First", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 1}}, ManaColors: kop.ColW | kop.ColU, ManaN: 1, ManaEach: true},
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1}}}, true},
		{kop.CardDef{Name: "Half Fee Second", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 1}}, ManaColors: kop.ColW | kop.ColU, ManaN: 1, ManaEach: true}}}, false},
		{land("Half Painland", kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColW, ManaN: 1,
			Prog: []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefController, A: kop.Const(1)}}}), true},
		{kop.CardDef{Name: "Half Painland Pair", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColW | kop.ColU, ManaN: 1, AFlags: kop.AfManaChoice,
				Prog: []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefController, A: kop.Const(1)}}}}}, false},
		{kop.CardDef{Name: "Half Pain Choice First", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColG | kop.ColB, ManaN: 1, AFlags: kop.AfManaChoice},
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColW | kop.ColU, ManaN: 1, AFlags: kop.AfManaChoice,
				Prog: []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefController, A: kop.Const(1)}}}}}, true},
		{kop.CardDef{Name: "Half Spire", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostPayLife, N: 1}, ManaColors: kop.ColAny, ManaN: 1}}}, false},
		{kop.CardDef{Name: "Half Spire Big", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostPayLife, N: 1}, ManaColors: kop.ColAny, ManaN: 2}}}, true},
		{kop.CardDef{Name: "Half Tainted", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColW | kop.ColB, ManaN: 1, AFlags: kop.AfManaChoice}}}, false},
		{land("Half Dynamic", kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColG, ManaN: 1, AFlags: kop.AfManaDynamic, ManaV: kop.ValueRef{K: kop.VX}}), false},
		{land("Half EachColor", kop.Ability{Kind: kop.AbMana, Cost: tap, ManaN: 1, AFlags: kop.AfManaEachColor}), false},
		{land("Half Dynamic Const", kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColG, ManaN: 1, AFlags: kop.AfManaDynamic, ManaV: kop.Const(2)}), true},
		{land("Half Restricted", kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1, ManaRestrict: 1}), false},
		{kop.CardDef{Name: "Half Restricted First", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1, ManaRestrict: 1},
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColW, ManaN: 1}}}, true},
		{kop.CardDef{Name: "Half Restricted Second", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColAny, ManaN: 1, ManaRestrict: 1}}}, false},
		{kop.CardDef{Name: "Half Brute", Types: kop.TCreature, Power: 2, Toughness: 2,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatMustAttack}}}}, false},
		{kop.CardDef{Name: "Half Brute Other", Types: kop.TCreature, Power: 2, Toughness: 2,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatMustAttack, B: 1}}}}, true},
		{kop.CardDef{Name: "Half Goader", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Prog: []kop.Instr{{Op: kop.OpGoad, Obj: kop.RefNone}}}}}, false},
		{kop.CardDef{Name: "Half Millikin", Types: kop.TCreature, Power: 0, Toughness: 1, Abilities: []kop.Ability{{Kind: kop.AbActivated, Cost: tap,
			Prog: []kop.Instr{{Op: kop.OpAddMana}}}}}, false},
		{kop.CardDef{Name: "Half Lonely", Types: kop.TCreature, Power: 2, Toughness: 2, AloneAtk: true}, true},
		{kop.CardDef{Name: "Half Watcher", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigTaps}}}}, true},
	} {
		t.Run(c.partner.Name, func(t *testing.T) {
			// PRECONDITION: the partner alone is the gap the walk reads.
			if got := atkTaxGap(&c.partner); got != c.refused {
				t.Fatalf("atkTaxGap(%s) = %v, want %v", c.partner.Name, got, c.refused)
			}
			set := stallPool("atktax", []kop.CardDef{tax, c.partner}, nil, nil, nil)
			deck := func(n string) []cardset.Entry {
				return []cardset.Entry{{Name: n, Count: 1}, {Name: "Vanilla A", Count: 1}}
			}
			_, _, _, err := set.Matchup(deck(tax.Name), deck(c.partner.Name))
			var r *cardset.InteractionRefusal
			if !c.refused {
				if err != nil {
					t.Fatalf("Matchup beside a priced source: %v", err)
				}
				return
			}
			if !errors.As(err, &r) {
				t.Fatalf("Matchup: %v, want an InteractionRefusal", err)
			}
			if r.Reason != StallName(StallAtkTax) || !slices.Contains(r.Cards, tax.Name) || !slices.Contains(r.Cards, c.partner.Name) || r.Why == "" {
				t.Errorf("refusal %+v, want %s naming %q and %q", r, StallName(StallAtkTax), tax.Name, c.partner.Name)
			}
			for _, alone := range []string{tax.Name, c.partner.Name} {
				if _, _, _, err := set.Matchup(deck(alone), deck("Vanilla B")); err != nil {
					t.Errorf("%s alone: %v", alone, err)
				}
			}
		})
	}
}

// The tax walks refuse a table over an emblem or an override only when a card
// of the table can make it: Table.Emblems and Table.Overrides are the pool's
// (tableReach), so a pod without the creating card is not refused over a
// record another deck interned.
func TestAttackTaxReachIsCardPrecise(t *testing.T) {
	tax := kop.CardDef{Name: "Half Tax", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatAttackTax, A: 2, StFlags: kop.StAtkTaxPlayer}}}}
	maker := kop.CardDef{Name: "Half Maker", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
		Prog: []kop.Instr{{Op: kop.OpCreateEmblem, C: 1}}}}}
	overrider := kop.CardDef{Name: "Half Overrider", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
		Prog: []kop.Instr{{Op: kop.OpOverride, C: 1}}}}}
	// an emblem's requirement naming a defender (B) is no broad duty (atkTaxMustOK)
	emblems := []kop.Emblem{{}, {Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatMustAttack, B: 1}}}}}
	basic := kop.SubMask{}.With(0)
	overrides := []kop.Override{{}, {AddSub: basic}}
	for _, c := range []struct {
		name    string
		extra   []kop.CardDef
		refused bool
	}{
		{"neither card", nil, false},
		{"emblem maker", []kop.CardDef{maker}, true},
		{"override maker", []kop.CardDef{overrider}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			tb := &kop.Table{Cards: append([]kop.CardDef{tax}, c.extra...), Emblems: emblems, Overrides: overrides, Subtypes: []string{"Forest"}}
			g := &genPrep{blgOn: true}
			for i := range g.ix {
				g.ix[i] = arenaNone
			}
			for i := range g.blgSub {
				g.blgSub[i] = subOutside
			}
			g.blgSub[4] = 0
			prepAtkTax(tb, g)
			if g.atkTaxStall != c.refused {
				t.Errorf("atkTaxStall = %v, want %v", g.atkTaxStall, c.refused)
			}
			prepBlkTax(tb, g)
		})
	}
}

// A broad emblem requirement (Bident of Thassa's "creatures your opponents
// control attack this turn if able") is read beside an attack tax as a card's
// is (staticFlag reads both), and still refuses a block tax table.
func TestAttackTaxReadsABroadEmblemDuty(t *testing.T) {
	tax := kop.CardDef{Name: "Half Tax", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatAttackTax, A: 2, StFlags: kop.StAtkTaxPlayer}}}}
	blk := kop.CardDef{Name: "Half Block Tax", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatBlockTax, A: 1}}}}
	maker := kop.CardDef{Name: "Half Maker", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
		Prog: []kop.Instr{{Op: kop.OpCreateEmblem, C: 1}}}}}
	tb := &kop.Table{Cards: []kop.CardDef{tax, blk, maker}, Overrides: []kop.Override{{}},
		Emblems: []kop.Emblem{{}, {Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatMustAttack}}}}}}
	g := &genPrep{}
	for i := range g.ix {
		g.ix[i] = arenaNone
	}
	prepAtkTax(tb, g)
	prepBlkTax(tb, g)
	if g.atkTaxStall {
		t.Error("a broad emblem duty refuses the attack tax table")
	}
	if !g.blkTaxStall {
		t.Error("a broad emblem duty does not refuse the block tax table")
	}
}
