package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// An opponent's Overgrowth on a land adds {G}{G} when it taps (CR 106.12a,
// 605.1b), so the enchanted land plus a Mountain and a Swamp afford a {4}{R}
// spell: the cast must be offered, whether the enchanted land is a basic or a
// Reflecting Pool (which makes one of two types per tap).
func TestOvergrowthEnchantedLandAffordsFiveMana(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		pool func(t *testing.T, e *Engine) state.ObjID
	}{
		{"mountain", func(t *testing.T, e *Engine) state.ObjID { return onBoard(t, e, 0, ppMountain) }},
		{"reflecting pool", func(t *testing.T, e *Engine) state.ObjID { return lrCorpus(t, e, "Reflecting Pool") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, _, spell := newFixtureDeck(t, 9921, lrSpell("4 R"))
			onBoard(t, e, 0, ppMountain)
			onBoard(t, e, 0, "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n")
			land := tc.pool(t, e)
			aura := onBoardCard(t, e, 1, corpusCard(t, "Overgrowth"))
			e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{land}})
			toMain1(t, e)
			e.pending = nil
			e.askPriority(0)
			if got := e.G.Obj(aura).AttachedTo; got != land {
				t.Fatalf("precondition: Overgrowth attached to %d, want %d", got, land)
			}
			offered := false
			for _, pp := range e.potentialPlans(0, 0) {
				if pp.Action.Kind == "cast" && pp.Action.Obj == spell {
					offered = true
					if pp.Reason != "" {
						t.Fatalf("the {4}{R} cast is judged %q although the enchanted land, a Mountain and a Swamp make five mana", pp.Reason)
					}
				}
			}
			if !offered {
				t.Fatal("precondition: no potential cast of the spell at all")
			}
		})
	}
}
