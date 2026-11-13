package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPopulateAsksAmongSeveralTokens pins CR 701.36a for the Populate$ arm:
// with two creature tokens the controller picks the one to copy (one
// copypermanent_choice KChoose over both), where the old arm copied the first
// without asking. The unpicked token is not copied.
func TestPopulateAsksAmongSeveralTokens(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Growing Ranks")
	searchMoveByName(t, e, "Growing Ranks", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)

	first := e.G.NextID
	e.emit(events.Event{Kind: events.CardToken, Obj: bear, Player: 0})
	second := e.G.NextID
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "w_1_1_human_warrior"})
	if o1, o2 := e.G.Obj(first), e.G.Obj(second); o1 == nil || o2 == nil || !o1.IsToken || !o2.IsToken || o1.Card == o2.Card {
		t.Fatalf("precondition: two creature tokens of different cards, got %+v %+v", o1, o2)
	}

	driveToStep(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	d := drainToTargetAsk(t, e, 200)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "copypermanent_choice" {
		t.Fatalf("populate with two tokens posed no copypermanent_choice ask: %+v", d)
	}
	if d.Min != 1 || d.Max != 1 || len(d.Options) != 2 || d.Options[0].Obj != first || d.Options[1].Obj != second {
		t.Fatalf("ask = Min %d Max %d options %+v, want 1/1 over tokens %d then %d", d.Min, d.Max, d.Options, first, second)
	}
	submitChoices(t, e, d.Options[1].Index)
	passUntilStackEmpty(t, e, 40)

	copies := 0
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.IsToken && o.IsCopy && o.ID != first && o.ID != second && o.Zone == state.ZBattlefield {
			copies++
			if o.Card != e.G.Obj(second).Card {
				t.Fatalf("populate copied card %v, want the picked second token's %v", o.Card, e.G.Obj(second).Card)
			}
		}
	}
	if copies != 1 {
		t.Fatalf("populate minted %d copies, want exactly 1", copies)
	}
	replayCheck(t, e, cfg)
}
