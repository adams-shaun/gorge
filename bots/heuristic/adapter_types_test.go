package heuristic

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// typeChars is a minimal Chars whose layer-4 line is scripted: it stands in
// for rules.Engine.ViewTypes when a type-changing effect has made a printed
// creature into a land.
type typeChars struct {
	types map[state.ObjID][]string
}

func (c typeChars) Power(state.ObjID) int32     { return 2 }
func (c typeChars) Toughness(state.ObjID) int32 { return 2 }
func (c typeChars) Keywords(state.ObjID) []string {
	return nil
}
func (c typeChars) ViewTypes(id state.ObjID) ([]string, bool) {
	types, ok := c.types[id]
	return types, ok
}

// TestAdaptersUseDerivedTypeLine pins the two Board adapters to the same
// source for Creature/Land/Basic/Instant: the view half reads the projected
// CardView.Types (the engine's layer-4 line), so the game half must read
// rules.Engine.ViewTypes rather than the printed face. Without it a printed
// creature turned into a land by Imprisoned in the Moon is a creature on one
// half and a land on the other, and the A2 land rule sees a different board
// depending on who asked.
func TestAdaptersUseDerivedTypeLine(t *testing.T) {
	card := parseTestCard(t, "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	face := card.Faces[0]

	g := state.NewGame([]string{"a", "b"})
	o := g.AddObject(card, 0)
	o.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})

	game := botpolicy.BoardFromGame(g, typeChars{types: map[state.ObjID][]string{o.ID: {"Land"}}}, 0)
	v := view.View{
		Viewer: 0,
		Phase:  "main1",
		Step:   state.StepMain1.String(),
		Active: 0,
		Players: []view.PlayerView{
			{ID: 0, Life: 20, Battlefield: []view.CardView{{
				ID:             o.ID,
				Name:           face.Name,
				Types:          "Land",
				ManaCost:       face.ManaCost,
				ColourIdentity: cards.ColourMask(face.ColourIdentity()),
				Power:          2,
				Toughness:      2,
				Controller:     0,
				Owner:          0,
			}}},
			{ID: 1, Life: 20},
		},
	}
	fromView := BoardFromView(v)

	gameCard, viewCard := game.Cards.Get(o.ID), fromView.Cards.Get(o.ID)
	if !gameCard.Land || !viewCard.Land {
		t.Fatalf("derived Land = game %v, view %v; want true on both", gameCard.Land, viewCard.Land)
	}
	if gameCard.Creature || viewCard.Creature {
		t.Fatalf("derived Creature = game %v, view %v; want false on both", gameCard.Creature, viewCard.Creature)
	}
	if gameCard.Basic || viewCard.Basic {
		t.Fatalf("derived Basic = game %v, view %v; want false on both", gameCard.Basic, viewCard.Basic)
	}
	if gameCard.TypeSig != viewCard.TypeSig {
		t.Fatalf("TypeSig = game %#x, view %#x; want the same derived line", gameCard.TypeSig, viewCard.TypeSig)
	}
	if game.Creatures.Has(o.ID) || fromView.Creatures.Has(o.ID) {
		t.Fatalf("derived land remained in the creature census: game %v, view %v", game.Creatures.Has(o.ID), fromView.Creatures.Has(o.ID))
	}
}
