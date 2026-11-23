package rules

// A creator-sourced delayed rider must not lose its AFFECTED object when the
// CREATOR was blinked between the rider's push and its resolution. CR
// 603.7/603.7c: the delayed trigger refers to the objects it was created for,
// and only the creator changed zones, so the rider's Defined$ Self still
// names the token. Rionya, Fire Dancer's token-copy exile rider
// (scheduleAtEOTRider's |CR= trailer, CR 603.7d) blinked by Thassa,
// Deep-Dwelling is the live case (dirac pair dr-mbc-abaa seed 447610,
// pair 169). The rider's registration carries the affected object as its
// Source and the creator as the |CR= trailer, exactly what
// effects.scheduleAtEOTRider emits for the AtEOT$ Exile rider.

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const creatorBlinkBoardSrc = "Name:Creator Blink Board\nManaCost:0\nTypes:Enchantment\nOracle:x\n"
const creatorBlinkTokenSrc = "Name:Creator Blink Token\nManaCost:0\nTypes:Creature\nPT:1/1\nOracle:x\n"

// creatorBlinkFixture puts a creator and a token on seat 0's battlefield and
// registers the __kwWarpExile rider scheduleAtEOTRider emits for the AtEOT$
// Exile rider: the token as the affected object, the creator riding the
// |CR= trailer.
func creatorBlinkFixture(t *testing.T) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	creator := battlefieldFixture(t, e, 0, creatorBlinkBoardSrc)
	token := battlefieldFixture(t, e, 0, creatorBlinkTokenSrc)
	e.emit(events.Event{Kind: events.DelayedRegister, Obj: token, Player: 0,
		Step: state.StepEnd, Counter: "__kwWarpExile",
		Text: "|CR=" + strconv.FormatUint(uint64(creator), 10)})
	if len(e.G.Delayed) != 1 {
		t.Fatalf("precondition: delayed registrations = %d, want one", len(e.G.Delayed))
	}
	return e, creator, token
}

// creatorBlinkBlink round-trips id through exile and back and fails unless it
// came back as a new incarnation.
func creatorBlinkBlink(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	before := e.G.Obj(id).Incarnation
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZExile, To: state.ZBattlefield})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: id %d zone = %v after the blink, want battlefield", id, o)
	}
	if o.Incarnation == before {
		t.Fatalf("precondition: incarnation = %d before and after the blink, want a new one", before)
	}
}

// creatorBlinkPush drives to the end step and pushes the delayed rider onto
// the stack, mirroring delayedSelfFixture's drain.
func creatorBlinkPush(t *testing.T, e *Engine) {
	t.Helper()
	driveToEndStep(t, e)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		if len(e.G.Stack) != 0 {
			break
		}
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("the delayed rider never reached the stack")
	}
}

// TestDelayedExileSurvivesCreatorBlink: the creator was blinked while its
// rider sat on the stack, so at resolution the creator is a new object — and
// the token, which never moved, must still be exiled. Before the
// delayedSourceReturned guard the rider's SourceReturned stamp followed the
// creator's re-entry and Defined$ Self named nothing.
func TestDelayedExileSurvivesCreatorBlink(t *testing.T) {
	t.Parallel()
	e, creator, token := creatorBlinkFixture(t)
	creatorBlinkPush(t, e)

	// The push stamped the creator's pre-blink incarnation; blink it in
	// response, before the rider resolves.
	creatorBlinkBlink(t, e, creator)

	mark := len(e.L.Events)
	passUntilStackEmpty(t, e, 40)
	if got := e.G.Obj(token); got == nil || got.Zone != state.ZExile {
		t.Fatalf("token zone = %v after the rider resolved, want exile (CR 603.7: the token did not change zones, so the promise still holds)", got)
	}
	moved := false
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.MoveZone && ev.Obj == token && ev.To == state.ZExile {
			moved = true
		}
	}
	if !moved {
		t.Fatal("no MoveZone took the token to exile")
	}
}

// TestDelayedExileSkipsReturnedToken is the control the affected-object
// incarnation check owns: the TOKEN itself left and returned, so it is a new
// object the promise does not find (CR 400.7) and stays on the battlefield.
func TestDelayedExileSkipsReturnedToken(t *testing.T) {
	t.Parallel()
	e, creator, token := creatorBlinkFixture(t)
	_ = creator

	// Blink the token before the end step: the registration's incarnation
	// stamp no longer matches it, so the resolving rider must leave it alone.
	creatorBlinkBlink(t, e, token)
	creatorBlinkPush(t, e)

	mark := len(e.L.Events)
	passUntilStackEmpty(t, e, 40)
	if got := e.G.Obj(token); got == nil || got.Zone != state.ZBattlefield {
		t.Fatalf("returned token zone = %v after the rider resolved, want battlefield (CR 400.7: a token that itself left and returned is a new object)", got)
	}
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.MoveZone && ev.Obj == token && ev.To == state.ZExile {
			t.Fatal("the returned token was exiled; the affected-object incarnation check moved")
		}
	}
}
