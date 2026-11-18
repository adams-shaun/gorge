package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// reflStallScen: seat 0 holds five wide-any restricted sources ({T}: Add two
// mana in any combination of colours, spend it only on your instant casts —
// the anyR gap, kop.AfManaChoice with a kop.ManaRestrict), each naming a
// different restriction filter (a different restricted-mana batch kind), a
// Plaza-shaped card (its plain {T}: Add {C} and a reflected read over lands,
// which make no colour: the board is a reflected one, reflBoard), and a
// five-{G} instant in hand. Three of the five sources' mana is already
// floating (earlier taps left it), so a payment past every line needs two
// more restricted floats of two more kinds and State.rb holds one batch
// fewer than that: the reflected search's second float fails, which latches
// StallRestrX in the fork (reflDo).
func reflStallScen(t *testing.T) (*scen, Obj, [5]Obj) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	var srcs [5]CardID
	for i := range srcs {
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TInstant, Ctrl: kop.CtrlYou})
		fid := kop.FilterID(len(tbl.Filters) - 1)
		srcs[i] = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Courtyard " + string(rune('A'+i)), Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, AFlags: kop.AfManaChoice, Cost: kop.Cost{Flags: kop.CostTap},
				ManaColors: kop.ColAny, ManaN: 2, ManaRestrict: fid},
		}})
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Ctrl: kop.CtrlYou})
	among := kop.FilterID(len(tbl.Filters) - 1)
	// The Plaza shape: a plain {T}: Add {C} first (its planner unit, manaAb 0),
	// then a reflected any-colour read over lands — lands make no colour, so
	// the reflected read adds nothing here; it makes the board a reflected one.
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Plaza", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
		{Kind: kop.AbMana, AFlags: kop.AfManaReflect, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColAny, ManaN: 1, ManaAmong: among},
	}})
	ember := CardID(len(tbl.Cards))
	emberCost := kop.ManaCost{}
	emberCost.Pips[kop.G] = 6
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Ember", Types: kop.TInstant, Cost: emberCost})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	var os [5]Obj
	for i, c := range srcs {
		os[i] = sc.perm(c, 0)
	}
	sc.perm(CardID(len(tbl.Cards)-2), 0) // the Plaza
	emberO := sc.hand(ember, 0)
	return sc, emberO, os
}

// TestReflScriptStallPropagates: on the reflStallScen board every payment line
// of the reflected search is past the six-{G} cast, and the search's lines
// float their second restricted unit past the batches State.rb already holds —
// reflDo latches StallRestrX in the fork. reflScript declares the stall on the
// real state instead of dropping the play as "no payment": the game draws as
// the unmodelled stall instead of silently losing the cast gorge offers
// (TestFiltScriptStallPropagates's shape).
func TestReflScriptStallPropagates(t *testing.T) {
	sc, emberO, os := reflStallScen(t)
	s, k := sc.s, sc.k
	a := MakeAction(AkCast, TCode(emberO), 0, 0)
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	restr, anyR := s.scriptRestr(k, 0), s.scriptAnyR(k, 0)
	if anyR.count() < 5 {
		t.Fatalf("precondition: the board lists %d restricted any-colour sources, want 5", anyR.count())
	}
	if !s.reflBoard(k, 0, restr, anyR) {
		t.Fatal("precondition: the board is no reflected one (reflBoard false)")
	}
	// Three taps left their mana floating restricted, one batch per kind.
	for i := 0; i < 3; i++ {
		sl := sc.slot(os[i])
		c := s.cardAt(sl)
		rab := &k.t.Cards[c].Abilities[k.gen.anyR(c)]
		if !s.floatRestrAb(k, 0, sl, rab, int(kop.G), 1) {
			t.Fatalf("precondition: %s's mana did not float restricted", k.t.Cards[c].Name)
		}
		s.perms[sl].flags |= pfTapped // the tap that produced it
	}
	if s.restrN(0) != 3 {
		t.Fatalf("precondition: restricted %v, want three units floating", s.rb[0])
	}
	if anyR := s.scriptAnyR(k, 0); anyR.count() != 2 {
		t.Fatalf("precondition: the search walks %d any-colour sources, want the two untapped", anyR.count())
	}
	found, dead := s.reflScript(k, a, 0, false)
	if found {
		t.Fatalf("the search paid the cast (dead %v), want its only lines to stall", dead)
	}
	if !s.Stalled() || s.StallReason() != StallRestrX {
		t.Fatalf("the search dropped the play silently: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallRestrX))
	}
}
