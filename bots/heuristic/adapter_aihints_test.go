package heuristic

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
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
	face := card.Faces[0]

	g := state.NewGame([]string{"a", "b"})
	o := g.AddObject(card, 0)
	o.Zone = state.ZHand
	g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})

	game := botpolicy.BoardFromGame(g, typeChars{}, 0)
	v := view.View{
		Viewer: 0,
		Phase:  "main1",
		Step:   state.StepMain1.String(),
		Active: 0,
		Players: []view.PlayerView{
			{ID: 0, Life: 20, Hand: []view.CardView{{
				ID:         o.ID,
				Name:       face.Name,
				Types:      "Creature Bear",
				ManaCost:   face.ManaCost,
				Power:      2,
				Toughness:  2,
				Controller: 0,
				Owner:      0,
				PlayMain1:  cards.PlayMain1Always,
				RemoveDeck: true,
			}}},
			{ID: 1, Life: 20},
		},
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
