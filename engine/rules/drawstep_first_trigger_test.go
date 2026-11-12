package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestOrcishBowmastersDrawStepExemptionIsTheDrawersOwn pins CR 504.1's
// "their draw step" half of FirstCardInDrawStep$ False. During seat 0's draw
// step an opponent's first draw is not that opponent's turn-based draw, so
// Orcish Bowmasters must trigger. Before the active-player gate the helper
// counted any drawer's first Draw since the StepChange and wrongly exempted
// it.
func TestOrcishBowmastersDrawStepExemptionIsTheDrawersOwn(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	source := onBoardCard(t, e, 0, corpusCard(t, "Orcish Bowmasters"))

	e.G.Active = 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDraw})
	if e.G.Step != state.StepDraw || e.G.Active != 0 {
		t.Fatalf("precondition: step=%v active=%d, want seat 0's draw step", e.G.Step, e.G.Active)
	}
	drawn := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.Draw, Player: 1, Obj: drawn, From: state.ZLibrary, To: state.ZHand, Secret: true})
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != source {
		t.Fatalf("non-active opponent's draw queued %#v, want one Orcish Bowmasters trigger", e.pendingTriggers)
	}
}
