package heuristic

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestAuraLogicAdaptersAgree pins Card.AuraLogic to the same printed face on
// both adapter halves: the game half reads effects.CastAuraLogic, the view
// half the projected CardView.AuraLogic.
func TestAuraLogicAdaptersAgree(t *testing.T) {
	card := parseTestCard(t, "Name:Test Pump Aura\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\nSVar:AttachAILogic:Pump\nOracle:x\n")
	e := rules.New(rules.Config{Seed: 0, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{{}, {}}})
	o := e.G.AddObject(card, 0)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{o.ID})

	game := botpolicy.BoardFromGame(e.G, e, 0)
	fromView := BoardFromView(view.Project(e.G, e, 0, nil))
	gc, vc := game.Cards.Get(o.ID), fromView.Cards.Get(o.ID)
	if gc.AuraLogic != "Pump" || vc.AuraLogic != "Pump" {
		t.Fatalf("AuraLogic = game %q, view %q; want Pump on both", gc.AuraLogic, vc.AuraLogic)
	}
}
