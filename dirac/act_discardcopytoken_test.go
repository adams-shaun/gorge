package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// discardScen is a scenario whose stack holds a test sorcery whose one
// instruction makes its controller (seat 0) discard one chosen card.
func discardScen(t *testing.T) *scen {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Discard One", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpDiscard, Obj: kop.RefController, A: kop.Const(1), C: kop.ChooseSelf}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	o := sc.obj(src, 0)
	sc.s.objs[o].zone = kop.ZoneStack
	sc.s.push(stackEntry{kind: seSpell, obj: o, ctrl: 0, card: src, trigObj: NoObj})
	sc.s.res.e = stackEntry{kind: seSpell, obj: o, ctrl: 0, card: src, trigObj: NoObj}
	sc.s.res.pc = 0
	return sc
}

// copyTokenInHand puts a token copy of a permanent into p's hand, where a
// bounced token waits for the state-based actions to cease it.
func copyTokenInHand(sc *scen, p uint8) Obj {
	o := sc.hand(v.Plains, p)
	sc.s.objs[o].of |= ofToken | ofCopy
	return o
}

// TestDiscardOffersNoCopyToken: a token copy bounced to the hand is no card in
// it to Forge, and gorge's matcher rejects it (CR 707.10h), so a discard of one
// card does not offer it (Calamitous Tide returning an Offspring token).
func TestDiscardOffersNoCopyToken(t *testing.T) {
	sc := discardScen(t)
	a, b := sc.hand(v.Plains, 0), sc.hand(v.Plains, 0)
	tok := copyTokenInHand(sc, 0)
	sc.s.runProg(sc.k)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChDiscard {
		t.Fatalf("ask kind %d sub %d, want DkChoose ChDiscard", d.Kind, d.Sub)
	}
	for i := 0; i < int(d.N); i++ {
		if got := Obj(d.Options[i].Source()); got == tok {
			t.Fatalf("the token copy is offered for discard: %v", d.Options[:d.N])
		}
	}
	if d.N != 2 {
		t.Fatalf("%d discard options, want 2 (%d and %d)", d.N, a, b)
	}
}

// TestDiscardAllCardsAsksNothingPastCopyToken: one card in the hand besides a
// token copy is discarded with no ask, the token left where it is.
func TestDiscardAllCardsAsksNothingPastCopyToken(t *testing.T) {
	sc := discardScen(t)
	card := sc.hand(v.Plains, 0)
	tok := copyTokenInHand(sc, 0)
	sc.s.runProg(sc.k)
	if sc.s.objs[card].zone != kop.ZoneGraveyard {
		t.Fatalf("the lone card is in zone %d, want the graveyard", sc.s.objs[card].zone)
	}
	if sc.s.objs[tok].zone != kop.ZoneHand {
		t.Fatalf("the token copy is in zone %d, want the hand", sc.s.objs[tok].zone)
	}
}
