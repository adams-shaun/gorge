package heuristic

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestAdaptersCarryAIHints pins the A3 hint facts to the same printed face on
// both adapter halves: the view half reads the projected CardView fields, the
// game half reads cards.Face directly. A hint one half filled and the other
// left zero would make forge-heuristic cast differently depending on who
// asked, exactly the divergence the whole-game parity tests exist to catch.
func TestAdaptersCarryAIHints(t *testing.T) {
	card := parseTestCard(t, "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\nAI:RemoveDeck:All\nSVar:PlayMain1:ALWAYS\n")

	// A real engine is the view.Chars the projection needs; empty decks keep
	// the position to just the card this test adds.
	e := rules.New(rules.Config{Seed: 0, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{{}, {}}})
	o := e.G.AddObject(card, 0)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{o.ID})

	game := botpolicy.BoardFromGame(e.G, e, 0)
	// Project the game rather than hand-building the CardView: this is the
	// engine/view hunk that fills the two facts on the wire, so a projection
	// that stopped carrying them must fail here.
	v := view.Project(e.G, e, 0, nil)
	if len(v.Players) < 1 || len(v.Players[0].Hand) != 1 {
		t.Fatalf("projected hand = %#v, want the one card", v.Players)
	}
	if cv := v.Players[0].Hand[0]; cv.PlayMain1 != cards.PlayMain1Always || !cv.RemoveDeck {
		t.Fatalf("projected hints = (%q, %v), want (%q, true)", cv.PlayMain1, cv.RemoveDeck, cards.PlayMain1Always)
	}
	fromView := BoardFromView(v)

	gc, vc := game.Cards.Get(o.ID), fromView.Cards.Get(o.ID)
	if gc.PlayMain1 != cards.PlayMain1Always || vc.PlayMain1 != cards.PlayMain1Always {
		t.Fatalf("PlayMain1 = game %q, view %q; want %q on both", gc.PlayMain1, vc.PlayMain1, cards.PlayMain1Always)
	}
	if !gc.RemoveDeck || !vc.RemoveDeck {
		t.Fatalf("RemoveDeck = game %v, view %v; want true on both", gc.RemoveDeck, vc.RemoveDeck)
	}
}
