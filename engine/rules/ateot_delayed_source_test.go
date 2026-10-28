package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestAtEOTDelayedTriggerSourceIsCreator pins CR 603.7d for the AtEOT$ token
// rider: Harried Dronesmith creates a Thopter whose end-step sacrifice is a
// delayed triggered ability whose SOURCE is the Dronesmith (the permanent
// whose ability created it), not the token. The body still names the token,
// so resolving sacrifices the token and keeps its creator. The second case
// has the creator leave the battlefield before the end step: the trigger
// still resolves, and its source is the creator's last-known information.
func TestAtEOTDelayedTriggerSourceIsCreator(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		creatorLeaves bool
	}{
		{name: "creator stays"},
		{name: "creator left the battlefield"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := searchTestRegistry(t)
			e, cfg := ateotEngine(t, reg, "Harried Dronesmith")
			dronesmith := ateotFind(t, e, "Harried Dronesmith", 0)
			ateotTo(t, e, dronesmith, state.ZLibrary, state.ZBattlefield)

			// The Dronesmith's own beginning-of-combat trigger mints the
			// Thopter and its AtEOT$ YourSacrifice rider.
			ateotDriveToStep(t, e, e.G.Turn, e.G.Active, state.StepBeginCombat)
			for e.putTriggersOnStack() {
				answerTriggerOrders(t, e)
				passUntilStackEmpty(t, e, 20)
			}
			passUntilStackEmpty(t, e, 20)

			var token state.ObjID
			minted := 0
			for i := range e.G.Objs {
				o := &e.G.Objs[i]
				if o.IsToken && o.Zone == state.ZBattlefield {
					minted++
					token = o.ID
				}
			}
			if minted != 1 {
				t.Fatalf("battlefield tokens = %d, want exactly the Dronesmith's Thopter", minted)
			}
			// Precondition: the AtEOT$ rider registered against the token but
			// recorded the Dronesmith as its creator.
			if len(e.G.Delayed) != 1 {
				t.Fatalf("delayed registrations = %d, want 1", len(e.G.Delayed))
			}
			dt := e.G.Delayed[0]
			if dt.Source != token || dt.Creator != dronesmith {
				t.Fatalf("registration = %+v, want Source %d (token) / Creator %d (Dronesmith)",
					dt, token, dronesmith)
			}
			replayCheck(t, e, cfg)

			if tc.creatorLeaves {
				ateotTo(t, e, dronesmith, state.ZBattlefield, state.ZGraveyard)
				if z := e.G.Obj(dronesmith).Zone; z != state.ZGraveyard {
					t.Fatalf("creator zone = %s, want graveyard before the end step", z)
				}
			}

			ateotDriveToStep(t, e, e.G.Turn, e.G.Active, state.StepEnd)
			for e.putTriggersOnStack() {
				answerTriggerOrders(t, e)
				passUntilStackEmpty(t, e, 20)
			}
			if len(e.G.Stack) == 0 {
				t.Fatal("the delayed sacrifice never reached the stack")
			}
			top := e.G.Obj(e.G.Stack[len(e.G.Stack)-1])
			if top == nil || top.Ability == nil {
				t.Fatalf("stack top %+v is not an ability", top)
			}
			if top.Source != dronesmith {
				t.Fatalf("delayed trigger source = %d, want creator Harried Dronesmith %d",
					top.Source, dronesmith)
			}
			if top.Controller != 0 {
				t.Fatalf("delayed trigger controller = %d, want p0", top.Controller)
			}
			replayCheck(t, e, cfg)

			passUntilStackEmpty(t, e, 20)
			if z := e.G.Obj(token).Zone; z != state.ZCeased && z != state.ZGraveyard {
				t.Fatalf("the Thopter survived the end step (zone %s), want it sacrificed", z)
			}
			if !tc.creatorLeaves {
				if z := e.G.Obj(dronesmith).Zone; z != state.ZBattlefield {
					t.Fatalf("Harried Dronesmith moved at the end step (zone %s), want it kept", z)
				}
			}
			replayCheck(t, e, cfg)
		})
	}
}
