package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestOppositionAgentExilesSeatZeroSearchFinds is the seat-swapped
// TestOppositionAgentExilesOpponentSearchFinds: the agent is seat 1's and
// seat 0 searches. The search scope's searcher is seat 0, PlayerID 0, which
// must still read as a search in flight, so the found card is exiled.
func TestOppositionAgentExilesSeatZeroSearchFinds(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	agent := e.G.AddObject(choiceCorpusCard(t, "Opposition Agent"), 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: agent.ID, From: state.ZLibrary, To: state.ZBattlefield})
	if c := e.G.Obj(agent.ID).Controller; c != 1 {
		t.Fatalf("agent controller = %d, want 1", c)
	}
	tutor := handFixture(t, e, 0, "Name:Dark Tutor\nManaCost:B\nTypes:Instant\nA:SP$ ChangeZone | Origin$ Library | Destination$ Hand | ChangeType$ Card | ChangeNum$ 1\nOracle:x\n")
	addMana(t, e, 0, "B")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("seat 0's priority with the tutor missing: %+v", d)
	}
	castIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == tutor {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("tutor cast not offered: %+v", d.Options)
	}
	submitChoices(t, e, castIdx)
	d = drainUntilAsk(t, e, 30)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "search" {
		t.Fatalf("search pick ask missing: %+v", d)
	}
	if d.Player != 1 {
		t.Fatalf("search pick posed to seat %d, want the agent's controller (1)", d.Player)
	}
	found := d.Options[0].Obj
	if z := e.G.Obj(found).Zone; z != state.ZLibrary {
		t.Fatalf("found card starts in %s, want the library", z)
	}
	submitChoices(t, e, d.Options[0].Index)
	passUntilStackEmpty(t, e, 30)
	if z := e.G.Obj(found).Zone; z != state.ZExile {
		t.Fatalf("found card zone = %s, want exile (the agent's replacement)", z)
	}
}
