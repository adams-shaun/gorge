package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestViewGoadedByReadsEventGoads pins view.CardView.GoadedBy on a real
// engine: an event-backed goad (events.Goad) names its goader (seat 2) for
// every viewer, while a Goad$ True static's (Shiny Impetus, seat 0), which
// is derived rather than state, is not listed; an ungoaded creature lists
// none.
func TestViewGoadedByReadsEventGoads(t *testing.T) {
	t.Parallel()
	e := staticGoadEngine(t)
	impetus := onBoardCard(t, e, 0, corpusCardByName(t, "Shiny Impetus"))
	victim := onBoardReady(t, e, 1, "Name:Victim\nTypes:Creature\nPT:1/1\nOracle:x\n")
	other := onBoardReady(t, e, 1, "Name:Other\nTypes:Creature\nPT:1/1\nOracle:x\n")
	e.emit(events.Event{Kind: events.Attach, Obj: impetus, IDs: []state.ObjID{victim}})
	e.emit(events.Event{Kind: events.Goad, Obj: victim, Player: 2})
	for viewer := state.PlayerID(0); viewer < 3; viewer++ {
		v := view.Project(e.G, e, viewer, nil)
		for _, cv := range v.Players[1].Battlefield {
			want := []state.PlayerID(nil)
			if cv.ID == victim {
				want = []state.PlayerID{2}
			}
			if cv.ID == other || cv.ID == victim {
				if !slices.Equal(cv.GoadedBy, want) {
					t.Errorf("viewer %d: %s GoadedBy = %v, want %v", viewer, cv.Name, cv.GoadedBy, want)
				}
			}
		}
	}
}
