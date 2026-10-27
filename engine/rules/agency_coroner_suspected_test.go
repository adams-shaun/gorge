package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// agencyCoronerDraw activates Agency Coroner sacrificing the Grizzly Bears
// (suspected first when suspect is set) and returns how many cards seat 0
// drew from the activation. The suspected designation ends the moment the
// creature leaves the battlefield (CR 702.157b), so "if the sacrificed
// creature was suspected" must be answered from last known information.
func agencyCoronerDraw(t *testing.T, suspect bool) int {
	t.Helper()
	reg := searchTestRegistry(t)
	e, ids := attachedToFixture(t, reg, 9412, nil, []string{"Agency Coroner", "Grizzly Bears"}, nil)
	coroner, bear := ids["Agency Coroner"], ids["Grizzly Bears"]
	if suspect {
		e.emit(events.Event{Kind: events.AlterAttribute, Obj: bear, Text: suspectedAttribute, Amount: 1})
	}
	if got := e.G.Obj(bear).Suspected; got != suspect {
		t.Fatalf("precondition: Bears Suspected = %v, want %v", got, suspect)
	}
	before := len(e.G.Zone(state.ZHand, 0))

	addMana(t, e, 0, "CCB")
	submitChoices(t, e, abilityOption(t, e, coroner, 0).Index)
	// The sacrifice cost may pose a pick; the Bears are the only candidate.
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		chooseObj(t, e, d.Options[0].Kind, bear)
	}
	attachDrain(t, e, 40)

	if z := e.G.Obj(bear).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: Bears zone = %s, want graveyard (sacrificed)", z)
	}
	if e.G.Obj(bear).Suspected {
		t.Fatal("precondition: the zone change should have cleared the live Suspected flag")
	}
	return len(e.G.Zone(state.ZHand, 0)) - before
}

// TestAgencyCoronerSuspected pins "Sacrificed$Valid Card.IsSuspected": the
// sacrificed creature's suspected status is read as it last existed.
func TestAgencyCoronerSuspected(t *testing.T) {
	t.Parallel()
	if got := agencyCoronerDraw(t, true); got != 2 {
		t.Fatalf("sacrificing a suspected creature drew %d, want 2", got)
	}
	if got := agencyCoronerDraw(t, false); got != 1 {
		t.Fatalf("sacrificing an unsuspected creature drew %d, want 1", got)
	}
}
