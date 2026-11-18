package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// delveXFilterChoices casts Logic Knot ({X}{U}{U}, delve) from p0's hand with
// {U}{U} floating, an untapped Island and Izzet Signet ({1},{T}: {U}{R}, a
// filter whose fee the Island pays: the two together make two generic beside the pool) and gy
// cards in the graveyard, after mutate has edited the spell's printed cost, and
// returns the X values the ask offers (CR 601.2b, 702.66a, 605.3b). Editing the
// cost prices a two-{X} or fixed-generic delve cost in the same scenario.
func delveXFilterChoices(t *testing.T, gy int, mutate func(c *kop.ManaCost)) string {
	t.Helper()
	deck := []cardset.Entry{{Name: "Logic Knot", Count: 1}, {Name: "Island", Count: 8}, {Name: "Izzet Signet", Count: 1}}
	tbl, da, _, err := cardsettest.Synth(t, "tail-47").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tb := *tbl
	tb.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	mutate(&tb.Cards[id["Logic Knot"]].Cost)
	sc := newScenTableLib(t, &tb, id["Island"], Config{NoElide: true})
	sc.perm(id["Island"], 0)
	sc.perm(id["Izzet Signet"], 0)
	sc.grave(id["Island"], 0, gy)
	knot := sc.hand(id["Logic Knot"], 0)
	sc.crimeStackSpell(id["Izzet Signet"], 1) // Logic Knot's target: a spell on the stack
	sc.s.pl[0].pool[kop.U] = 2
	// preconditions: the delve credit exists and the filter is on the battlefield
	if g := sc.k.gen; int(sc.s.pl[0].graveN) != gy || g == nil || !g.anyDelve || !g.anyFilt {
		t.Fatalf("graveN %d, gen %+v: want a graveyard, a delve card and a filter", sc.s.pl[0].graveN, g != nil)
	}
	sc.castOption(t, knot, AkCast, 0)
	for i := 0; i < 8; i++ {
		d := sc.s.Step()
		switch {
		case d.Kind == DkChoose && d.Sub == ChX:
			var xs []int
			for j := 0; j < int(d.N); j++ {
				xs = append(xs, int(d.Options[j].Mode()))
			}
			return fmt.Sprint(xs)
		case d.Kind == DkChoose && d.Sub == ChGenDelve:
			for j := 0; j < int(d.N); j++ {
				if d.Options[j].Kind() == AkDone {
					sc.s.Apply(d.Options[j])
				}
			}
		default:
			t.Fatalf("decision %v/%d before the X ask", d.Kind, d.Sub)
		}
	}
	t.Fatal("no X ask")
	return ""
}

// TestDelveXFilterCredit: the X ask prices each value in generic mana through
// the filter window, the delve credit spent first. One {X}: the window pays two
// generic, so X = 2 bare, 5 with three cards delved.
func TestDelveXFilterCredit(t *testing.T) {
	if got := delveXFilterChoices(t, 3, func(*kop.ManaCost) {}); got != "[0 1 2 3 4 5]" {
		t.Fatalf("one {X}: offered %s, want [0 1 2 3 4 5]", got)
	}
}

// TestDelveXFilterTwoXCounts: with {X}{X} each X is two generic, so the credit
// of three covers X = 1 outright and X = 2 leaves one for the window's two;
// X = 3 leaves three and is not payable. The credit is generic and X counts in
// twos: comparing X with the filter's uncredited largest X (1) would offer 3.
func TestDelveXFilterTwoXCounts(t *testing.T) {
	got := delveXFilterChoices(t, 3, func(c *kop.ManaCost) { c.X = 2 })
	if got != "[0 1 2]" {
		t.Fatalf("two {X}: offered %s, want [0 1 2]", got)
	}
}

// TestDelveXFilterFixedGeneric: {4}{X}{U}{U} cannot be paid at X = 0 without the
// credit, so the window's largest uncredited X is -1: a sentinel, not a count of
// spare mana (the window's two generic are short of the {4} even at X = 0, and
// reading -1 as a count would offer X = 2). Three delved leave 1 + X generic for
// the window's two, so X = 1 is the largest.
func TestDelveXFilterFixedGeneric(t *testing.T) {
	got := delveXFilterChoices(t, 3, func(c *kop.ManaCost) { c.Generic = 4 })
	if got != "[0 1]" {
		t.Fatalf("fixed generic: offered %s, want [0 1]", got)
	}
}
