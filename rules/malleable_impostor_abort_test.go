package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestMalleableImpostorAbortRestoresCopyChoice pins CR 733.1 for the
// event-backed ETB copy answer. The first arm distinguishes no prior election
// from an Optional decline; the second proves a prior selected template is
// restored by the same replayable Choose event form.
func TestMalleableImpostorAbortRestoresCopyChoice(t *testing.T) {
	for _, tc := range []struct {
		name       string
		priorValid bool
	}{
		{name: "no_prior_election"},
		{name: "prior_template", priorValid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := handEngine(t, corpusAlternativeCard(t, "Malleable Impostor"))
			prior := e.G.AddObject(corpusAlternativeCard(t, "Colossal Dreadmaw"), 1)
			prior.Zone = state.ZBattlefield
			picked := e.G.AddObject(corpusAlternativeCard(t, "Craw Wurm"), 1)
			picked.Zone = state.ZBattlefield
			e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{prior.ID, picked.ID})
			id := e.G.Zone(state.ZHand, 0)[0]
			if e.G.Obj(id).ETBCloneChoiceValid || e.G.Players[0].Pool.Total() != 0 {
				t.Fatalf("fixture precondition: choice valid=%t mana=%d, want false/0", e.G.Obj(id).ETBCloneChoiceValid, e.G.Players[0].Pool.Total())
			}
			if tc.priorValid {
				e.emit(events.Event{Kind: events.Choose, Obj: id, Counter: "clone", IDs: []state.ObjID{prior.ID}})
			}
			beforeChoice, beforeValid := e.G.Obj(id).ETBCloneChoice, e.G.Obj(id).ETBCloneChoiceValid

			// Direct beginCast deliberately exercises the ordinary insufficient-
			// mana abort after the real ETB answer; the legal-action gate would
			// withhold this unaffordable cast before its answer can be probed.
			castMode(t, e, id, "")
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose {
				t.Fatalf("expected Malleable Impostor copy choice, got %+v", d)
			}
			var selected decision.Option
			found := false
			for _, opt := range d.Options {
				if opt.Kind == "clone" && opt.Obj == picked.ID {
					selected, found = opt, true
				}
			}
			if !found {
				t.Fatalf("fixture precondition: copy target %d absent from %+v", picked.ID, d.Options)
			}
			e.pending = nil // emulate Submit consuming the ETB decision.
			e.etbAnswer(d, []decision.Option{selected})
			if o := e.G.Obj(id); !o.ETBCloneChoiceValid || o.ETBCloneChoice != picked.ID {
				t.Fatalf("copy answer was not recorded: valid=%t choice=%d, want true/%d", o.ETBCloneChoiceValid, o.ETBCloneChoice, picked.ID)
			}

			e.continueCast() // pushes, fails payment, and reaches the normal abortCast path.
			o := e.G.Obj(id)
			if o == nil || o.Zone != state.ZHand {
				t.Fatalf("unpayable cast did not reverse to hand: %+v", o)
			}
			if o.ETBCloneChoiceValid != beforeValid || o.ETBCloneChoice != beforeChoice {
				t.Fatalf("abort choice = valid=%t choice=%d, want valid=%t choice=%d", o.ETBCloneChoiceValid, o.ETBCloneChoice, beforeValid, beforeChoice)
			}
			wantCounter := "clone-clear"
			if beforeValid {
				wantCounter = "clone"
			}
			foundRestore := false
			for _, ev := range e.L.Events {
				if ev.Kind == events.Choose && ev.Obj == id && ev.Counter == wantCounter {
					foundRestore = true
				}
			}
			if !foundRestore {
				t.Fatalf("abort did not emit replayable %q restoration event", wantCounter)
			}
		})
	}
}
