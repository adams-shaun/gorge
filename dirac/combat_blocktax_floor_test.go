package dirac

import (
	"errors"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A block tax refuses the matchup beside a block floor or a block duty
// (blkTaxFloorPart): the option builders decide Done without the defender's
// mana, so a budget covering one blocker of a menace attacker strands the
// declaration.
func TestBlockTaxRefusesFloorsAndDuties(t *testing.T) {
	tax := kop.CardDef{Name: "Floor Block Tax", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatBlockTax, A: 1}}}}
	bear := func(name string) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TCreature, Power: 2, Toughness: 2}
	}
	menace, floor, alone, lure, mustBe := bear("Floor Menace"), bear("Floor Three"), bear("Floor Alone"), bear("Floor Lure"), bear("Floor Must")
	menace.Keywords = kop.KwMenace
	floor.MinBlockers = 3
	alone.AloneBlk = true
	lure.Lure = true
	mustBe.MustBeBlocked = true
	anthem := kop.CardDef{Name: "Floor Anthem", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatContinuous, Kw: kop.KwMenace}}}}
	grant := kop.CardDef{Name: "Floor Grant", Types: kop.TInstant, Abilities: []kop.Ability{{Kind: kop.AbSpell,
		Prog: []kop.Instr{{Op: kop.OpGrantKeyword, Obj: kop.RefSelf, C: int16(kop.KwMenace)}}}}}
	for _, partner := range []kop.CardDef{menace, floor, alone, lure, mustBe, anthem, grant} {
		t.Run(partner.Name, func(t *testing.T) {
			// PRECONDITION: the partner is a floor or duty part, and not an
			// attack-window gap (the refusal is the new rule's).
			if !blkTaxFloorPart(&partner) || atkTaxGap(&partner) {
				t.Fatalf("blkTaxFloorPart %v atkTaxGap %v, want true and false", blkTaxFloorPart(&partner), atkTaxGap(&partner))
			}
			set := stallPool("blktaxfloor", []kop.CardDef{tax, partner}, nil, nil, nil)
			deck := func(n string) []cardset.Entry {
				return []cardset.Entry{{Name: n, Count: 1}, {Name: "Vanilla A", Count: 1}}
			}
			_, _, _, err := set.Matchup(deck(tax.Name), deck(partner.Name))
			var r *cardset.InteractionRefusal
			if !errors.As(err, &r) {
				t.Fatalf("Matchup: %v, want an InteractionRefusal", err)
			}
			if r.Reason != StallName(StallBlkTax) || !slices.Contains(r.Cards, tax.Name) || !slices.Contains(r.Cards, partner.Name) || r.Why == "" {
				t.Errorf("refusal %+v, want %s naming %q and %q", r, StallName(StallBlkTax), tax.Name, partner.Name)
			}
			if _, _, _, err := set.Matchup(deck(partner.Name), deck("Vanilla B")); err != nil {
				t.Errorf("%s without the tax: %v", partner.Name, err)
			}
		})
	}
}

// TestBlockTaxMenaceTaxerStalls: a 2/2 menace taxer charging {1} per blocker,
// the defender holding two bears and one Plains. The budget covers the first
// blocker but not menace's second: the kernel stalls the table at its start
// rather than offering a pick it can never complete (a blockers decision with
// no options).
func TestBlockTaxMenaceTaxerStalls(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TCreature})
	fCreature := kop.FilterID(len(tbl.Filters) - 1)
	taxer := CardID(len(tbl.Cards))
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		kop.CardDef{Name: "Test Taxer", Types: kop.TCreature, Power: 2, Toughness: 2, Keywords: kop.KwMenace,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatBlockTax, Filter: fCreature, A: 1}}}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
	// PRECONDITION: the table carries the tax and the attacker has menace.
	if !blkTaxOn(sc.k) || tbl.Cards[taxer].Keywords&kop.KwMenace == 0 {
		t.Fatal("precondition: a block tax on a menace creature")
	}
	if !sc.k.gen.blkTaxStall || sc.s.g.stalled != StallBlkTax {
		t.Fatalf("blkTaxStall %v stalled %s, want true and %s", sc.k.gen.blkTaxStall, StallName(sc.s.g.stalled), StallName(StallBlkTax))
	}
}

// TestBlockTaxEmptyDecisionStalls: a blockers decision the filter empties (Done
// withheld, every pick priced out) stalls the game; it never reaches StepInto
// as a decision with no options.
func TestBlockTaxEmptyDecisionStalls(t *testing.T) {
	sc, atk, blk, _ := blkTaxScene(t, 1)
	s, k := sc.s, sc.k
	// PRECONDITION: the pick is priced out ({2} against a budget of 1) and the
	// game has not stalled.
	if n, _ := s.blkTaxCharge(k, sc.slot(blk), sc.slot(atk)); n <= s.manaBudget(k, 1) || s.g.stalled != 0 {
		t.Fatalf("charge %d budget %d stalled %d, want charge > budget, no stall", n, s.manaBudget(k, 1), s.g.stalled)
	}
	d := Decision{Kind: DkBlockers, N: 1, Options: [MaxOptions]Action{MakeAction(AkBlock, TCode(blk), TCode(atk), 0)}}
	s.blkTaxFilter(k, &d)
	if d.N != 0 || d.Kind != DkNone || s.g.stalled != StallBlkTax || !s.over {
		t.Fatalf("N %d kind %d stalled %s over %v, want an empty DkNone and a %s stall", d.N, d.Kind, StallName(s.g.stalled), s.over, StallName(StallBlkTax))
	}
}
