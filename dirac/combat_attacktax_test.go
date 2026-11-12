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
// shape it prices (atkTaxManaAb).
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
		{land("Half Treasure", kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostSacSelf}, ManaColors: kop.ColAny, ManaN: 1}), true},
		{land("Half Painland", kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColW, ManaN: 1,
			Prog: []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefController, A: kop.Const(1)}}}), true},
		{land("Half Restricted", kop.Ability{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1, ManaRestrict: 1}), true},
		{kop.CardDef{Name: "Half Brute", Types: kop.TCreature, Power: 2, Toughness: 2,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatMustAttack}}}}, true},
		{kop.CardDef{Name: "Half Goader", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Prog: []kop.Instr{{Op: kop.OpGoad, Obj: kop.RefNone}}}}}, true},
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
