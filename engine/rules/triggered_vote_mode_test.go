package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTriggeredVotePosesNoPlacementMode pins CR 701.38a against CR 603.3c:
// a triggered Vote's Choices$ names the ballot options, not the ability's
// modes, so putting the trigger on the stack must not pose a KModes
// placement ask (nor record ChosenModes on the stack object). The per-voter
// ballots are posed by effVote only when the trigger resolves.
func TestTriggeredVotePosesNoPlacementMode(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := voteCarrierEngine(t, reg, "Coercive Portal")
	carrier := enterCarrier(t, e, "Coercive Portal")

	// The upkeep trigger fires as the step is entered: one trigger queued.
	if n := stepTriggers(e, state.StepUpkeep, 0); n != 1 {
		t.Fatalf("upkeep triggers queued = %d, want Coercive Portal's one", n)
	}
	// Draining the queue pushes the trigger and runs the CR 603.3c
	// placement ask, if any.
	e.putTriggersOnStack()
	trig := voteTriggerOnStack(e, carrier)
	if trig == 0 {
		t.Fatalf("the Vote trigger is not on the stack (pending = %+v)", e.Pending())
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KModes {
		t.Fatalf("triggered Vote posed a placement KModes: %+v", d)
	}
	if modes := e.G.Obj(trig).ChosenModes; len(modes) != 0 {
		t.Fatalf("triggered Vote recorded ChosenModes %v on the stack object, want none", modes)
	}
	// No ballot is posed until the trigger resolves.
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose && d.ResumeKind == "vote" {
		t.Fatalf("ballot ask %+v posed before the trigger resolved", d)
	}

	// Resolve the trigger: the ballots, one per voter, now appear.
	ballots := 0
	for i := 0; i < 100; i++ {
		if o := e.G.Obj(trig); o == nil || o.Zone != state.ZStack {
			break
		}
		d := e.Pending()
		switch {
		case d == nil:
			e.priorityRound()
		case d.Kind == decision.KPriority:
			passOnce(t, e)
		case d.Kind == decision.KChoose && d.ResumeKind == "vote":
			ballots++
			if voteTriggerOnStack(e, carrier) == 0 {
				t.Fatalf("ballot ask %+v posed while the trigger is not on the stack", d)
			}
			submitChoices(t, e, 0)
		default:
			t.Fatalf("unexpected decision %v while resolving the Vote trigger: %+v", d.Kind, d)
		}
	}
	if ballots != 3 {
		t.Fatalf("ballots posed = %d, want 3 (one per seat after resolution)", ballots)
	}
}
