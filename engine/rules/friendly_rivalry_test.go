package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// friendlyRivalryCast casts Friendly Rivalry naming the first creature as the
// root target, the legendary one as the up-to-one second target, and the wolf
// as the recipient, then returns the board. Every target is announced on cast
// (CR 601.2c), so the spell is on the stack with all three slots filled.
func friendlyRivalryCast(t *testing.T, seed uint64) (*Engine, Config, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	e, cfg, mine, theirs := cr601Board(t, seed,
		map[string]state.Zone{"Friendly Rivalry": state.ZHand, "Grizzly Bears": state.ZBattlefield, "Jasmine Boreal": state.ZBattlefield},
		map[string]state.Zone{"Colossal Dreadmaw": state.ZBattlefield})
	addMana(t, e, 0, "RG")
	edrSeatZeroPriority(t, e)
	bears, jasmine, wolf := mine["Grizzly Bears"], mine["Jasmine Boreal"], theirs["Colossal Dreadmaw"]
	if got := e.Power(bears); got != 2 {
		t.Fatalf("precondition: Grizzly Bears power %d, want 2", got)
	}
	if got := e.Power(jasmine); got != 4 {
		t.Fatalf("precondition: Jasmine Boreal power %d, want 4", got)
	}
	cr601Cast(t, e, mine["Friendly Rivalry"], "")
	answerTargetAsk(t, e, []state.ObjID{bears})
	answerCastSubObj(t, e, jasmine)
	answerCastSubObj(t, e, wolf)
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack %v, want Friendly Rivalry alone", e.G.Stack)
	}
	return e, cfg, bears, jasmine, wolf
}

// TestFriendlyRivalryBothDamagersDeal: "Target creature you control and up to
// one other target legendary creature you control each deal damage equal to
// their power to target creature you don't control." The EachDamage link's
// DefinedDamagers$ Targeted.YouCtrl names every target slot's creature (CR
// 115.1), so Bears (2) and Jasmine (4) together kill a 3/3; and when Bears is
// gone on resolution Jasmine, still legal, deals her 4 (CR 608.2b).
func TestFriendlyRivalryBothDamagersDeal(t *testing.T) {
	t.Parallel()
	t.Run("both_deal", func(t *testing.T) {
		e, cfg, _, _, wolf := friendlyRivalryCast(t, 60201)
		cr601ResolveQuietly(t, e)
		if o := e.G.Obj(wolf); o.Zone != state.ZGraveyard {
			t.Fatalf("Colossal Dreadmaw zone %s damage %d, want dead from 2 + 4 damage (neither alone kills a 6/6)", o.Zone, o.Damage)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("first_target_gone_second_still_deals", func(t *testing.T) {
		e, cfg, bears, jasmine, wolf := friendlyRivalryCast(t, 60202)
		e.emit(events.Event{Kind: events.MoveZone, Obj: bears, From: state.ZBattlefield, To: state.ZGraveyard})
		cr601ResolveQuietly(t, e)
		if o := e.G.Obj(jasmine); o.Zone != state.ZBattlefield {
			t.Fatalf("Jasmine zone %s, want battlefield", o.Zone)
		}
		if o := e.G.Obj(wolf); o.Zone != state.ZBattlefield || o.Damage != 4 {
			t.Fatalf("Colossal Dreadmaw zone %s damage %d, want 4 damage from Jasmine alone", o.Zone, o.Damage)
		}
		replayCheck(t, e, cfg)
	})
}
