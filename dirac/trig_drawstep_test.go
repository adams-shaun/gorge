package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDrawnFirstCardInDrawStep: a Drawn trigger's FirstCardInDrawStep$
// qualifier fires exactly on the first card its player draws in that player's
// own draw step (CR 504.1; gorge trigmatch.firstCardInDrawStep), on the
// second card, and for a non-active seat's draw during another player's draw
// step -- that is an extra draw, not a turn-based one.
func TestDrawnFirstCardInDrawStep(t *testing.T) {
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(
		[]cardset.Entry{{Name: "Plains", Count: 20}}, []cardset.Entry{{Name: "Plains", Count: 20}})
	if err != nil {
		t.Fatal(err)
	}
	table := tbl
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	firstID := CardID(len(table.Cards))
	table.Cards = append(table.Cards, kop.CardDef{Name: "Test First Draw", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigDrawn, Whose: kop.WhoseAny, Flags5: kop.Tf5FirstCardInDrawStep}, Prog: gain}}})
	notFirstID := CardID(len(table.Cards))
	table.Cards = append(table.Cards, kop.CardDef{Name: "Test Not First Draw", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigDrawn, Whose: kop.WhoseAny, Flags5: kop.Tf5NotFirstCardInDrawStep}, Prog: gain}}})

	sc := newScenTable(t, table, Config{NoElide: true})
	sc.perm(firstID, 0)
	sc.perm(notFirstID, 0)
	s := sc.s

	drawn := func(p uint8) (n int, card CardID) {
		s.npend = 0
		s.draw(p)
		if s.npend == 0 {
			return 0, 0
		}
		return int(s.npend), s.pend[0].card
	}

	// Seat 1's own draw step: its first card fires First, its second fires
	// NotFirst.
	s.step, s.active, s.g.drawStepN = kop.StepDraw, 1, 0
	if n, c := drawn(1); n != 1 || c != firstID {
		t.Fatalf("seat 1's first draw: %d triggers of card %d, want First", n, c)
	}
	if n, c := drawn(1); n != 1 || c != notFirstID {
		t.Fatalf("seat 1's second draw: %d triggers of card %d, want NotFirst", n, c)
	}
	// During seat 0's draw step a seat-1 draw is not seat 1's draw step, so
	// it is not "first" even though no card was drawn in this step yet.
	s.active, s.g.drawStepN = 0, 0
	if n, c := drawn(1); n != 1 || c != notFirstID {
		t.Fatalf("non-active seat 1's draw: %d triggers of card %d, want NotFirst", n, c)
	}
	// Seat 0's own first draw fires First.
	if n, c := drawn(0); n != 1 || c != firstID {
		t.Fatalf("seat 0's first draw: %d triggers of card %d, want First", n, c)
	}
}
