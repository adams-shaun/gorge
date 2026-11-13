package dirac

import (
	"errors"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A block tax refuses the matchup beside each part its source walk does not
// model (combat_blocktax.go blkTaxGap), and plays beside a source of a shape it
// prices.
func TestBlockTaxRefusesUnmodelledPartners(t *testing.T) {
	tap := kop.Cost{Flags: kop.CostTap}
	tax := kop.CardDef{Name: "Half Block Tax", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatBlockTax, A: 2}}}}
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
			if got := blkTaxGap(&c.partner); got != c.refused {
				t.Fatalf("blkTaxGap(%s) = %v, want %v", c.partner.Name, got, c.refused)
			}
			set := stallPool("blktax", []kop.CardDef{tax, c.partner}, nil, nil, nil)
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
			if r.Reason != StallName(StallBlkTax) || !slices.Contains(r.Cards, tax.Name) || !slices.Contains(r.Cards, c.partner.Name) || r.Why == "" {
				t.Errorf("refusal %+v, want %s naming %q and %q", r, StallName(StallBlkTax), tax.Name, c.partner.Name)
			}
			for _, alone := range []string{tax.Name, c.partner.Name} {
				if _, _, _, err := set.Matchup(deck(alone), deck("Vanilla B")); err != nil {
					t.Errorf("%s alone: %v", alone, err)
				}
			}
		})
	}
}

// blkTaxScene is an FDN Plains table with a Taxer (a creature whose block tax
// charges every creature {2} to block it), and a Bear.
func blkTaxScene(t *testing.T, lands int) (sc *scen, atk, blk Obj, ls []Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TCreature})
	fCreature := kop.FilterID(len(tbl.Filters) - 1)
	taxer := CardID(len(tbl.Cards))
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		kop.CardDef{Name: "Test Taxer", Types: kop.TCreature, Power: 2, Toughness: 2,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatBlockTax, Filter: fCreature, A: 2}}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc = newScenTableLib(t, tbl, 0, Config{NoElide: true}) // Plains is the matchup table's card 0
	if !blkTaxOn(sc.k) {
		t.Fatal("precondition: the prepared table carries a block tax")
	}
	atk, blk = sc.perm(taxer, 0), sc.perm(taxer+1, 1)
	for i := 0; i < lands; i++ {
		ls = append(ls, sc.perm(0, 1))
	}
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, uint8(atk), 0xff)
	sc.do(t, AkDone, 0xff, 0xff)
	sc.passUntil(t, kop.StepBlockers)
	return sc, atk, blk, ls
}

// TestBlockTaxBudgetAndPayment: the defender's mana bounds the block (a {2}
// charge needs two lands: the pick is neither offered nor does the decision
// open with one land) and Done pays the charge by tapping the lands.
func TestBlockTaxBudgetAndPayment(t *testing.T) {
	blockAndDone := func(atk, blk Obj) Decision {
		return Decision{Kind: DkBlockers, N: 2, Options: [MaxOptions]Action{MakeAction(AkBlock, TCode(blk), TCode(atk), 0), MakeAction(AkDone, 0, 0, 0)}}
	}
	t.Run("one land", func(t *testing.T) {
		sc, atk, blk, _ := blkTaxScene(t, 1)
		s, k := sc.s, sc.k
		// PRECONDITION: the attacker is attacking, the charge is {2}, the budget {1}.
		if n, ok := s.blkTaxCharge(k, sc.slot(blk), sc.slot(atk)); !ok || n != 2 {
			t.Fatalf("charge = %d, %v, want 2", n, ok)
		}
		if b := s.manaBudget(k, 1); b != 1 || s.perms[sc.slot(atk)].flags&pfAttacking == 0 {
			t.Fatalf("budget %d attacking %v, want 1 and true", b, s.perms[sc.slot(atk)].flags&pfAttacking != 0)
		}
		if s.ctl == ctlBlockers {
			t.Fatal("a blockers decision opened though every pair is priced out (anyBlock)")
		}
		d := blockAndDone(atk, blk)
		s.blkTaxFilter(k, &d)
		if d.N != 1 || d.Options[0].Kind() != AkDone {
			t.Fatalf("filtered options %v, want only Done", d.Options[:d.N])
		}
		// a bypassing declaration is declined: taps standing, nothing declared
		s.applyBlock(k, MakeAction(AkBlock, TCode(blk), TCode(atk), 0))
		if s.perms[sc.slot(atk)].nblock != 1 {
			t.Fatalf("precondition: nblock = %d, want 1", s.perms[sc.slot(atk)].nblock)
		}
		if s.blkTaxPay(k) {
			t.Fatal("blkTaxPay paid {2} from one land")
		}
		s.dropBlockDeclaration(k)
		if bp := &s.perms[sc.slot(blk)]; bp.flags&pfBlocking != 0 || bp.blocking != NoSlot || s.perms[sc.slot(atk)].nblock != 0 {
			t.Fatalf("declaration not dropped: blocker %+v nblock %d", bp, s.perms[sc.slot(atk)].nblock)
		}
	})
	t.Run("two lands", func(t *testing.T) {
		sc, atk, blk, ls := blkTaxScene(t, 2)
		s, k := sc.s, sc.k
		if s.ctl != ctlBlockers || s.manaBudget(k, 1) != 2 {
			t.Fatalf("precondition: ctl %d budget %d, want the blockers decision and 2", s.ctl, s.manaBudget(k, 1))
		}
		d := blockAndDone(atk, blk)
		s.blkTaxFilter(k, &d)
		if d.N != 2 {
			t.Fatalf("filtered options %v, want the pick and Done", d.Options[:d.N])
		}
		for _, l := range ls {
			if s.perms[sc.slot(l)].flags&pfTapped != 0 {
				t.Fatal("precondition: a land starts tapped")
			}
		}
		sc.do(t, AkBlock, uint8(blk), uint8(atk))
		if s.perms[sc.slot(atk)].nblock != 1 || s.perms[sc.slot(blk)].blocking != sc.slot(atk) {
			t.Fatalf("precondition: nblock %d blocking %d", s.perms[sc.slot(atk)].nblock, s.perms[sc.slot(blk)].blocking)
		}
		sc.do(t, AkDone, 0xff, 0xff)
		for _, l := range ls {
			if s.perms[sc.slot(l)].flags&pfTapped == 0 {
				t.Errorf("land %d not tapped by the payment", l)
			}
		}
		var pool int
		for _, n := range s.pl[1].pool {
			pool += int(n)
		}
		if pool != 0 {
			t.Errorf("pool = %d after the payment, want 0", pool)
		}
		if s.perms[sc.slot(blk)].flags&pfBlocking == 0 {
			t.Error("the paid block was dropped")
		}
	})
}
