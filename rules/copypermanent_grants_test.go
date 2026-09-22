package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestHofriCopyPermanentGrants pins the real Hofri CopyPermanent mint: its
// AddSVars$/AddTriggers$ riders are attached to the Spirit copy, rather than
// being dropped as inert notes, AND the granted trigger works end to end --
// the Spirit's real departure (killed through the SBA path, not a raw
// MoveZone emit) fires the granted TrigLeavesBattlefield trigger and the
// exiled bearer returns to the graveyard.
func TestHofriCopyPermanentGrants(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg, ids := tokenRememberedBoard(t, reg, "Hofri Ghostforge", "Vampire Nighthawk")
	hofri, bearer := ids["Hofri Ghostforge"], ids["Vampire Nighthawk"]
	if e.G.Obj(hofri).Zone != state.ZBattlefield || e.G.Obj(bearer).Zone != state.ZBattlefield {
		t.Fatalf("precondition: Hofri and bearer must be on battlefield")
	}

	// Hofri's dies trigger first exiles the creature, then its sub-ability
	// CopyPermanent uses that exiled card's LKI.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bearer, From: state.ZBattlefield, To: state.ZGraveyard})
	e.putTriggersOnStack()
	e.resolveTop()
	passUntilStackEmpty(t, e, 40)

	var spirit state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && id != bearer && spirit == 0 {
			spirit = id
		}
	}
	if spirit == 0 {
		t.Fatalf("precondition: Hofri did not create a Spirit copy; log %+v", e.L.Events)
	}
	if grants := triggerGrantsOn(e, spirit); len(grants) != 1 {
		t.Fatalf("Spirit carries %d granted triggers, want 1", len(grants))
	}
	if got, ok := e.GrantedSVar(spirit, "HofriTrigReturn"); !ok || got == "" {
		t.Fatalf("Spirit lacks granted HofriTrigReturn SVar: %q, %v", got, ok)
	}

	// The departure is the pin's other half. Kill the Spirit through the REAL
	// SBA path -- damage + checkStateBased's destroyLethalDamage death batch,
	// never a raw MoveZone emit, whose live pass cannot match a granted
	// trigger on an already-departed source (the look-back pass needs the
	// death batch's snapshot). The granted trigger queues, resolves, and
	// HofriTrigReturn returns the exiled bearer to its owner's graveyard.
	if z := e.G.Obj(bearer).Zone; z != state.ZExile {
		t.Fatalf("precondition: the bearer must sit in exile before the Spirit's departure (zone %v)", z)
	}
	e.emit(events.Event{Kind: events.Damage, Obj: spirit, Amount: 99})
	e.checkStateBased()
	if z := e.G.Obj(spirit).Zone; z == state.ZBattlefield {
		t.Fatalf("precondition: the SBA pass did not kill the damaged Spirit (zone %v)", z)
	}
	e.putTriggersOnStack()
	e.resolveTop()
	passUntilStackEmpty(t, e, 40)
	if back := e.G.Obj(bearer); back == nil || back.Zone != state.ZGraveyard {
		t.Fatalf("the exiled bearer did not return to the graveyard when the Spirit left: zone %v, log %+v",
			e.G.Obj(bearer).Zone, e.L.Events)
	}

	// The whole round-trip replays byte-identically.
	replayCheck(t, e, cfg)
}
