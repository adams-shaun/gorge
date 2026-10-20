package botpolicy_test

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestDrawTokenAdaptersAgree pins the A6c facts both halves must fill: the
// game-shaped BoardFromGame and the view-shaped BoardFromView classify the
// same printed spell as a Draw or Token (or neither). The view half is
// built through view.Project -- the REAL projection a client receives --
// so a reverted projection line fails here, not only in the whole-game
// parity tests. A missing fill on either half makes the rule read a
// different board depending on the host.
func TestDrawTokenAdaptersAgree(t *testing.T) {
	for _, tc := range []struct {
		name  string
		src   string
		draw  bool
		token bool
	}{
		{
			name: "draw",
			src:  "Name:Divination\nManaCost:2 U\nTypes:Sorcery\nOracle:Draw two cards.\nA:SP$ Draw | NumCards$ 2 | SpellDescription$ Draw two cards.\n",
			draw: true,
		},
		{
			name:  "token",
			src:   "Name:Raise the Alarm\nManaCost:1 W\nTypes:Instant\nOracle:Create two 1/1 white Soldier creature tokens.\nA:SP$ Token | TokenAmount$ 2 | TokenScript$ w_1_1_soldier | SpellDescription$ Create two 1/1 white Soldier creature tokens.\n",
			token: true,
		},
		{
			name: "neither",
			src:  "Name:Lightning Bolt\nManaCost:R\nTypes:Instant\nOracle:Lightning Bolt deals 3 damage to any target.\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3 | SpellDescription$ Lightning Bolt deals 3 damage to any target.\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			card, diags := cards.ParseBytes("timing_drawtoken_adapter_test.go", []byte(tc.src))
			if len(diags) != 0 {
				t.Fatalf("parse: %v", diags)
			}
			card.Link()
			g := state.NewGame([]string{"a", "b"})
			o := g.AddObject(card, 0)
			g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})

			v := view.Project(g, nil, 0, nil)
			var cv *view.CardView
			for i := range v.Players[0].Hand {
				if v.Players[0].Hand[i].ID == o.ID {
					cv = &v.Players[0].Hand[i]
				}
			}
			if cv == nil {
				t.Fatalf("projection: hand card %d missing", o.ID)
			}
			if got := cv.SpellAPI == "Draw"; got != tc.draw {
				t.Fatalf("projected SpellAPI=%q, Draw=%v, want %v", cv.SpellAPI, got, tc.draw)
			}
			if got := cv.SpellAPI == "Token"; got != tc.token {
				t.Fatalf("projected SpellAPI=%q, Token=%v, want %v", cv.SpellAPI, got, tc.token)
			}

			gameCard := botpolicy.BoardFromGame(g, printedChars{}, 0).Cards.Get(o.ID)
			viewCard := heuristic.BoardFromView(v).Cards.Get(o.ID)
			if gameCard.Draw != tc.draw || gameCard.Token != tc.token {
				t.Fatalf("game facts Draw=%v Token=%v, want %v %v", gameCard.Draw, gameCard.Token, tc.draw, tc.token)
			}
			if viewCard.Draw != tc.draw || viewCard.Token != tc.token {
				t.Fatalf("view facts Draw=%v Token=%v, want %v %v", viewCard.Draw, viewCard.Token, tc.draw, tc.token)
			}
		})
	}
}
