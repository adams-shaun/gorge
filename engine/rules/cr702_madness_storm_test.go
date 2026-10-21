package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR702_35aMadnessExileIsMandatory: CR 702.35a, "If a player would
// discard this card, that player discards it, but exiles it instead of
// putting it into their graveyard." The replacement is not optional: a
// discarded Fiery Temper goes straight to exile with no replacement ask,
// and its madness trigger (the optional part, CR 702.35a's second ability)
// is created.
func TestCR702_35aMadnessExileIsMandatory(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 934, []string{"Fiery Temper"}, nil, nil)
	id := findCardObj(t, e, 0, "Fiery Temper", state.ZHand)
	e.pending = nil
	e.emit(events.Discard(id, 0))
	if d := e.Pending(); d != nil && d.Kind == decision.KReplacement {
		t.Fatalf("CR 702.35a seq %d: madness posed an optional replacement ask: %+v", len(e.L.Events), d)
	}
	if z := e.G.Obj(id).Zone; z != state.ZExile {
		t.Fatalf("CR 702.35a seq %d: discarded Fiery Temper is in %s, want exile", len(e.L.Events), z)
	}
	e.priorityRound()
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Source != id {
		t.Fatalf("CR 702.35a seq %d: want the madness trigger on the stack, stack %v", len(e.L.Events), e.G.Stack)
	}
	replayCheck(t, e, cfg)
}

// TestCR702_40aStormCountsSpellsCastBeforeIt: CR 702.40a, "When you cast
// this spell, copy it for each other spell that was cast before it this
// turn." The count is fixed by the spells cast BEFORE the storm spell: one
// Bolt, then the storm spell, then a second Bolt in response to the storm
// trigger -- one copy, not two.
func TestCR702_40aStormCountsSpellsCastBeforeIt(t *testing.T) {
	t.Parallel()
	tendrils := "Name:Tendrils\nManaCost:2 B B\nTypes:Sorcery\nK:Storm\nA:SP$ LoseLife | ValidTgts$ Player | LifeAmount$ 2 | SubAbility$ DBGainLife\nSVar:DBGainLife:DB$ GainLife | Defined$ You | LifeAmount$ 2\nOracle:x\n"
	e, cfg, td := newFixtureDeck(t, 91, tendrils, boltSrc, boltSrc)
	first := addToHand(t, e, 0, boltSrc)
	addMana(t, e, 0, "R")
	castObj(t, e, first)
	passUntilStackEmpty(t, e, 20)
	second := addToHand(t, e, 0, boltSrc)
	addMana(t, e, 0, "BBGG")
	// Cast the storm spell at seat 1; the storm trigger goes on the stack.
	submitChoices(t, e, castOptionFor(t, e, td).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("CR 702.40a seq %d: no target ask for the storm spell: %+v", len(e.L.Events), d)
	}
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			submitChoices(t, e, o.Index)
			break
		}
	}
	if len(e.G.Stack) != 2 {
		t.Fatalf("CR 702.40a seq %d: want the storm spell and its trigger on the stack, got %v", len(e.L.Events), e.G.Stack)
	}
	// In response to the storm trigger: the second Bolt.
	addMana(t, e, 0, "R")
	castObj(t, e, second)
	copies := 0
	for _, o := range e.G.Objs {
		if o.IsCopy {
			copies++
		}
	}
	if copies != 1 {
		t.Fatalf("CR 702.40a seq %d: storm made %d copies, want 1 (only one spell was cast before it)", len(e.L.Events), copies)
	}
	replayCheck(t, e, cfg)
}
