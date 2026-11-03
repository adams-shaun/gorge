package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestDisableTriggersDestinationList(t *testing.T) {
	e := newSeats(t, 2)
	staticCard := card(t, "Name:Destination List Gate\nTypes:Artifact\n"+
		"S:Mode$ DisableTriggers | ValidCause$ Creature | ValidMode$ ChangesZone | Destination$ Graveyard,Exile\nOracle:x\n")
	trigger := "T:Mode$ ChangesZone | Origin$ Any | Destination$ Any | TriggerZones$ Battlefield | Execute$ TrigDraw | TriggerDescription$ When this changes zones, draw a card.\n" +
		"SVar:TrigDraw:DB$ Draw | NumCards$ 1\n"
	creature := card(t, "Name:Zone Trigger Creature\nTypes:Creature\n"+trigger+"Oracle:x\n")

	gate := parkObj(t, e, staticCard, 0, state.ZBattlefield)
	if got := e.activeStatics("DisableTriggers"); len(got) != 1 || got[0].Source != gate {
		t.Fatalf("precondition: active DisableTriggers statics = %+v, want gate %d", got, gate)
	}
	sv := e.activeStatics("DisableTriggers")[0]
	code, ok := sv.ParamCode(cards.PKDestination)
	if !ok {
		t.Fatal("precondition: Destination$ has no compiled parameter")
	}
	dest := effects.Destination(code)
	if !dest.Admits(state.ZGraveyard) || !dest.Admits(state.ZExile) || dest.Admits(state.ZBattlefield) {
		t.Fatalf("precondition: compiled destination %v must admit Graveyard and Exile but not Battlefield", dest)
	}

	// Establish that this exact trigger fixture fires for a non-listed zone;
	// subsequent zero counts therefore exercise the active static.
	control := parkObj(t, e, creature, 1, state.ZHand)
	if n := countQueued(e, control, state.ZHand, state.ZBattlefield); n != 1 {
		t.Fatalf("control transition to Battlefield queued %d triggers, want 1", n)
	}
	for _, to := range []state.Zone{state.ZGraveyard, state.ZExile} {
		id := parkObj(t, e, creature, 1, state.ZHand)
		if n := countQueued(e, id, state.ZHand, to); n != 0 {
			t.Errorf("transition to %v queued %d triggers, want 0 under Destination$ Graveyard,Exile", to, n)
		}
	}
}

// TestDisableTriggersEnteringIsNotLeaving: a creature returning from a graveyard
// under "creatures entering don't cause abilities to trigger" stops the lines
// that watch it enter, not the ones that watch it leave the graveyard.
func TestDisableTriggersEnteringIsNotLeaving(t *testing.T) {
	e := newSeats(t, 2)
	parkObj(t, e, card(t, "Name:Entry Gate\nTypes:Artifact\n"+
		"S:Mode$ DisableTriggers | ValidCause$ Creature | ValidMode$ ChangesZone,ChangesZoneAll | Destination$ Battlefield\nOracle:x\n"), 0, state.ZBattlefield)
	listener := func(name, params string) {
		parkObj(t, e, card(t, "Name:"+name+"\nTypes:Artifact\nT:Mode$ ChangesZoneAll | ValidCards$ Creature | "+params+
			" | TriggerZones$ Battlefield | Execute$ TrigDraw\nSVar:TrigDraw:DB$ Draw | NumCards$ 1\nOracle:x\n"), 0, state.ZBattlefield)
	}
	listener("Leaves Graveyard", "Origin$ Graveyard | Destination$ Any")
	bear := parkObj(t, e, card(t, "Name:Bear\nTypes:Creature\nOracle:x\n"), 0, state.ZGraveyard)
	if n := countQueued(e, bear, state.ZGraveyard, state.ZBattlefield); n != 1 {
		t.Fatalf("leaving the graveyard for the battlefield queued %d, want 1", n)
	}
	listener("Enters", "Origin$ Graveyard | Destination$ Battlefield")
	e.pendingTriggers = nil
	bear = parkObj(t, e, card(t, "Name:Bear\nTypes:Creature\nOracle:x\n"), 0, state.ZGraveyard)
	if n := countQueued(e, bear, state.ZGraveyard, state.ZBattlefield); n != 1 {
		t.Fatalf("with an entry line beside it, the same move queued %d, want the leaving line's 1", n)
	}
}
