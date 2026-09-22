package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestHofriCopyPermanentGrants pins the real Hofri CopyPermanent mint: its
// AddSVars$/AddTriggers$ riders are attached to the Spirit copy, rather than
// being dropped as inert notes.
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

	// The grant is the prerequisite for the copy's later leaves-the-battlefield
	// trigger; the effects-level trigger matcher owns that event path.
	replayCheck(t, e, cfg)
}
