package view

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCardViewProjectsAuraLogic pins the CardView.AuraLogic projection: a
// creature Aura carrying an AttachAILogic SVar projects it, a Bear does not.
func TestCardViewProjectsAuraLogic(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	parse := func(text string) *cards.Card {
		c, d := cards.ParseBytes("a.txt", []byte(text))
		if len(d) != 0 {
			t.Fatalf("diags: %v", d)
		}
		c.Link()
		return c
	}
	aura := parse("Name:Test Pump Aura\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\nSVar:AttachAILogic:Pump\nOracle:x\n")
	bear := parse("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	if sa := aura.Faces[0].SpellAbility(); sa == nil || sa.API != "Attach" {
		t.Fatalf("fixture precondition: Aura spell ability = %+v, want Attach", sa)
	}
	ao := g.AddObject(aura, 0)
	ao.Zone = state.ZHand
	aid := ao.ID
	bo := g.AddObject(bear, 0)
	bo.Zone = state.ZHand
	bid := bo.ID
	g.SetZone(state.ZHand, 0, []state.ObjID{aid, bid})

	hand := Project(g, nil, 0, nil).Players[0].Hand
	if len(hand) != 2 {
		t.Fatalf("hand = %d cards, want 2", len(hand))
	}
	if got := hand[0].AuraLogic; got != "Pump" {
		t.Errorf("Aura AuraLogic = %q, want Pump", got)
	}
	if got := hand[1].AuraLogic; got != "" {
		t.Errorf("Bear AuraLogic = %q, want empty", got)
	}
}
