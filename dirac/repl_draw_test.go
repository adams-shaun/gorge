package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDrawMore: a controller's "draw N instead" replacement (gorge
// applyReplacements' Draw event) turns one draw into N plain draws; the
// opponent's draw is untouched; NotFirstCardInDrawStep$ exempts the first
// card of the draw step; a replacement is not met again by its own draws
// (CR 614.5) but another is (the second one of two draws here); two matching
// the same draw apply both (the composition is commutative: gorge settles
// the CR 616.1 order in scan order, unasked), and a lineage of more than
// drawChainMax sources is the declared stall.
func TestDrawMore(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	mk := func(repls ...kop.Repl) (*scen, []CardID) {
		tbl, _, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		var ids []CardID
		for _, r := range repls {
			ids = append(ids, CardID(len(tbl.Cards)))
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Doubler", Types: kop.TEnchantment, Abilities: []kop.Ability{
				{Kind: kop.AbReplacement, Repl: r}}})
		}
		return newScenTable(t, tbl, Config{NoElide: true}), ids
	}
	two := kop.Repl{Kind: kop.ReplDrawMore, A: 2}
	notFirst := kop.Repl{Kind: kop.ReplDrawMore, A: 2, C: kop.ReplDrawNotFirst}
	drew := func(s *State, p uint8, f func()) int {
		h := int(s.pl[p].handN)
		f()
		return int(s.pl[p].handN) - h
	}

	sc, ids := mk(two)
	s := sc.s
	sc.perm(ids[0], 0)
	if n := drew(s, 0, func() { s.draw(0) }); n != 2 {
		t.Fatalf("one draw under a doubler drew %d, want 2", n)
	}
	if n := drew(s, 1, func() { s.draw(1) }); n != 1 {
		t.Fatalf("the opponent drew %d, want 1", n)
	}
	if s.pl[0].drawnTurn != 2 {
		t.Fatalf("drawnTurn %d, want 2 (the replaced draw is not a draw)", s.pl[0].drawnTurn)
	}

	sc, ids = mk(notFirst)
	s = sc.s
	sc.perm(ids[0], 0)
	s.step, s.active, s.g.drawFirst = kop.StepDraw, 0, true
	if n := drew(s, 0, func() { s.draw(0) }); n != 1 {
		t.Fatalf("the draw step's first card drew %d, want 1", n)
	}
	if n := drew(s, 0, func() { s.draw(0) }); n != 2 {
		t.Fatalf("a second draw in the draw step drew %d, want 2", n)
	}
	s.step = kop.StepMain1
	s.g.drawFirst = false
	if n := drew(s, 0, func() { s.draw(0) }); n != 2 {
		t.Fatalf("a draw outside the draw step drew %d, want 2", n)
	}

	// an unconditional and a first-card-exempt one: the draw step's first
	// card is replaced by the first, its two draws meet the second, whose
	// first is exempt and whose second is doubled: three cards
	sc, ids = mk(two, notFirst)
	s = sc.s
	sc.perm(ids[0], 0)
	sc.perm(ids[1], 0)
	s.step, s.active, s.g.drawFirst = kop.StepDraw, 0, true
	if n := drew(s, 0, func() { s.draw(0) }); n != 3 {
		t.Fatalf("draw-step draw under both drew %d, want 3", n)
	}
	if s.g.stalled != 0 {
		t.Fatalf("stalled %d, want none", s.g.stalled)
	}

	// two replacements match one draw: the composition is commutative, so
	// gorge settles the CR 616.1 order in scan order, unasked, and both
	// doublers apply (2 x 2 = 4 cards)
	sc, ids = mk(two, two)
	s = sc.s
	sc.perm(ids[0], 0)
	sc.perm(ids[1], 0)
	if n := drew(s, 0, func() { s.draw(0) }); n != 4 {
		t.Fatalf("two doublers drew %d, want 4", n)
	}
	if s.g.stalled != 0 {
		t.Fatalf("two doubers: stalled %d, want none", s.g.stalled)
	}

	// four replacements in one lineage outrun the draw chain's three slots:
	// the declared capacity stall
	sc, ids = mk(two, two, two, two)
	s = sc.s
	for _, id := range ids {
		sc.perm(id, 0)
	}
	s.draw(0)
	if s.g.stalled != StallDrawRepl {
		t.Fatalf("four doublers: stalled %d, want StallDrawRepl", s.g.stalled)
	}
}
