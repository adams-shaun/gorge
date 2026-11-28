package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMutateAbilityUnionWalkers: the statM walkers that read the top card
// (s.cardAt) rather than the pile walk see an under card's replacement, cost
// static and mana ability too (CR 702.140d; gorge engine/state/pile.go reads
// the union). The mutating card carries all three and mutates UNDER the
// vanilla bird, so every ability below belongs to the under card.
func TestMutateAbilityUnionWalkers(t *testing.T) {
	tbl, mutator, bird, _ := unionTable()
	walker := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Walker", Types: kop.TCreature, Power: 1, Toughness: 1,
		Cost:      kop.ManaCost{Generic: 3, Pips: [kop.NumManaKinds]uint8{kop.G: 1}},
		HasMutate: true, MutateCost: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.G: 1}},
		MutateTgt: tbl.Cards[mutator].MutateTgt,
		Abilities: []kop.Ability{
			{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplLifeDouble}},
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatReduceCost, A: 1}},
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1},
		},
	})
	tax := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Tax", Types: kop.TInstant,
		Cost: kop.ManaCost{Generic: 1}, Timing: kop.TimingInstant, Colors: kop.ColC,
		Abilities: []kop.Ability{{Kind: kop.AbSpell}}})

	// pile mutates the walker under a bird, with the given forests on the
	// battlefield: the bird stays the top card and the walker every ability.
	pile := func(t *testing.T, forests int) (*scen, Obj) {
		t.Helper()
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.lands(0, v.Forest, forests)
		b := sc.perm(bird, 0)
		mutateCast(t, sc, sc.hand(walker, 0), b, false)
		resolveStack(t, sc)
		sl := sc.slot(b)
		if c := sc.s.cardAt(sl); c != bird {
			t.Fatalf("top card %d, want the bird %d", c, bird)
		}
		if rec := sc.s.mergeRecOf(b); rec == nil || rec.n != 1 || sc.s.objs[underObj(rec, 0)].card != walker {
			t.Fatalf("the walker is not the pile's under card")
		}
		return sc, b
	}

	t.Run("replacement", func(t *testing.T) {
		sc, b := pile(t, 2)
		if p, _ := sc.s.pt(sc.k, sc.slot(b)); p != 1 {
			t.Fatalf("precondition: the pile is %d power, want the bird's 1", p)
		}
		life := sc.s.pl[0].life
		sc.s.gainLife(sc.k, 0, 3)
		if got := sc.s.pl[0].life - life; got != 6 {
			t.Fatalf("life gained %d, want 6: the under card's life-gain doubler did not apply", got)
		}
	})

	t.Run("cost static", func(t *testing.T) {
		sc, _ := pile(t, 1)
		o := sc.hand(tax, 0)
		got := sc.s.adjustCostGen(sc.k, 0, tax, o, sc.k.cards[tax].cost)
		if got.Generic != 0 {
			t.Fatalf("test spell generic cost %d, want 0: the under card's reduction did not apply", got.Generic)
		}
		if acts := sc.castOptions(o); len(acts) == 0 {
			t.Fatalf("the reduced free cast is not offered")
		}
	})

	t.Run("mana ability", func(t *testing.T) {
		// the one Forest pays the mutate; the {G} spell must then be paid by
		// the pile's under-card mana ability (the bird prints none)
		sc, b := pile(t, 1)
		sl := sc.slot(b)
		if sc.s.perms[sl].flags&pfTapped != 0 {
			t.Fatalf("precondition: the pile is tapped before the cast")
		}
		o := sc.hand(v.GiantGrowth, 0)
		if acts := sc.castOptions(o); len(acts) == 0 {
			t.Fatalf("the {G} spell is not offered: the under card's mana ability was not counted")
		}
		sc.do(t, AkCast, TCode(o), scAny)
		sc.do(t, AkTarget, scAny, TCode(b))
		if sc.s.perms[sl].flags&pfTapped == 0 {
			t.Fatalf("the pile was not tapped to pay: the under card's mana ability was not used")
		}
		resolveStack(t, sc)
		if z := sc.s.objs[o].zone; z != kop.ZoneGraveyard {
			t.Fatalf("the {G} spell zone %d after resolution, want the graveyard", z)
		}
	})
}
