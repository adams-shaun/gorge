package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestAdventureInstantSpellFaceOfferedAtInstantTiming pins CR 714.3a: the
// Adventure face gets its own timing check before the creature front can
// suppress the card's hand offers.
func TestAdventureInstantSpellFaceOfferedAtInstantTiming(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg, id, oppBear := adventureCorpusEngine(t, reg)
	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepBeginCombat)
	e.pending = nil
	e.priorityRound()
	for _, r := range "UU" {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: string(r), Amount: 1})
	}
	e.priorityRound()

	if e.G.Step != state.StepBeginCombat {
		t.Fatalf("step = %s, want Begin Combat", e.G.Step)
	}
	if bear := e.G.Obj(oppBear); bear == nil || bear.Zone != state.ZBattlefield {
		t.Fatalf("opposing bear precondition failed: %+v", bear)
	}
	if got := e.G.Obj(id); got == nil || got.Zone != state.ZHand {
		t.Fatalf("Borrower precondition failed: %+v", got)
	}
	if plain := adventureOption(t, e, id, ""); plain != nil {
		t.Fatalf("creature front offered at instant timing: %+v", plain)
	}
	alt := adventureOption(t, e, id, "adventure_alt")
	if alt == nil || alt.Label != "Cast Petty Theft" {
		t.Fatalf("instant Adventure face not offered: %+v", castOptions(t, e))
	}
	replayCheck(t, e, cfg)
}
