package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kdrawnvpLower lowers a synthetic enchantment carrying one Drawn trigger
// line with the given params.
func kdrawnvpLower(t *testing.T, params map[string]string) *lCard {
	t.Helper()
	svars := map[string]string{"Body": "DB$ GainLife | LifeAmount$ 1"}
	params["Execute"] = "Body"
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "1 W",
		Types:    []string{"Enchantment"},
		SVars:    svars,
		Triggers: []cards.Trigger{{
			Mode:   "Drawn",
			Params: params,
			Effect: cards.ResolveSVar(svars, "Body"),
		}},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}

func kdrawnvpTrigger(t *testing.T, lc *lCard) *lTrigger {
	t.Helper()
	if len(lc.Refusals) != 0 {
		t.Fatalf("want a lowering, got refusals %v", lc.Refusals)
	}
	for _, a := range lc.Abilities {
		if a.Trigger != nil && a.Trigger.Event == "Drawn" {
			return a.Trigger
		}
	}
	t.Fatal("no Drawn trigger lowered")
	return nil
}

// TestDrawnvpLowers: a Drawn trigger scoped by ValidPlayer$ (no ValidCard$)
// lowers the drawing player relative to the controller (gorge
// drawnMatches); any other ValidPlayer spelling, or none, still refuses.
func TestDrawnvpLowers(t *testing.T) {
	tr := kdrawnvpTrigger(t, kdrawnvpLower(t, map[string]string{"ValidPlayer": "Opponent", "Number": "2", "TriggerZones": "Battlefield"}))
	if tr.Whose != whoOpp || tr.NthDraw != 2 {
		t.Errorf("ValidPlayer$ Opponent | Number$ 2: Whose %d NthDraw %d, want %d and 2", tr.Whose, tr.NthDraw, whoOpp)
	}
	tr = kdrawnvpTrigger(t, kdrawnvpLower(t, map[string]string{"ValidPlayer": "Player", "TriggerZones": "Battlefield"}))
	if tr.Whose != whoAny || tr.NthDraw != 0 {
		t.Errorf("ValidPlayer$ Player: Whose %d NthDraw %d, want %d and 0", tr.Whose, tr.NthDraw, whoAny)
	}
	for _, c := range []struct {
		name   string
		params map[string]string
		want   string
	}{
		{"qualified player", map[string]string{"ValidPlayer": "Player.Active", "TriggerZones": "Battlefield"}, `Drawn ValidCard "" ValidPlayer "Player.Active"`},
		{"opponent qualified", map[string]string{"ValidPlayer": "Player.Opponent+NonActive", "TriggerZones": "Battlefield"}, `ValidPlayer "Player.Opponent+NonActive"`},
		{"neither param", map[string]string{"TriggerZones": "Battlefield"}, `Drawn ValidCard "" ValidPlayer ""`},
	} {
		t.Run(c.name, func(t *testing.T) {
			enterstappedRefuseWant(t, kdrawnvpLower(t, c.params).Refusals, c.want)
		})
	}
}
