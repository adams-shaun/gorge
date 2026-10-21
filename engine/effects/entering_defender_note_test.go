package effects

// cr-g11b review finding (MINOR): the CR 508.4 entering-controller
// classification must not emit the "unrecognised GainControl$" diagnostic a
// second time, and must not run at all for a selector rider that names its
// own defender. emitEntryMove's own move is the one place the Note belongs.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func enteringDefenderUnreadGainControl(t *testing.T, attacking string) int {
	t.Helper()
	h, c := fixtureHost(t)
	src := h.g.Obj(c.Source)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: src.Zone, To: state.ZGraveyard})
	if o := h.g.Obj(c.Source); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: source = %+v, want graveyard", o)
	}
	h.g.Active = 0
	h.g.Step = state.StepDeclareAttackers
	c.Remembered = []state.Target{{Player: 1, IsPlayer: true}}
	effChangeZone(h, c, &cards.SA{API: "ChangeZone", Params: map[string]string{
		"Defined": "Self", "Origin": "Graveyard", "Destination": "Battlefield",
		"Attacking": attacking, "GainControl": "Bogus",
	}})
	return notesContaining(h, "unrecognised ChangeZone GainControl$")
}

func TestEnteringDefenderUnreadGainControlNotesOnce(t *testing.T) {
	if n := enteringDefenderUnreadGainControl(t, "True"); n != 1 {
		t.Fatalf("Attacking$ True: %d unrecognised GainControl$ notes, want 1 (classification must not duplicate emitEntryMove's)", n)
	}
	if n := enteringDefenderUnreadGainControl(t, "RememberedPlayer"); n != 1 {
		t.Fatalf("selector: %d unrecognised GainControl$ notes, want 1 (a named defender never classifies a controller)", n)
	}
}
