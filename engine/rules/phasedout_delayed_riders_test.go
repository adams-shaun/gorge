package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// These tests pin CR 702.26b for the delayed end-of-turn riders. A rider
// names the object it promised to act on rather than targeting it, so the
// battlefield target filters do not exclude the object once it phases out.
// The rider still triggers and goes on the stack, but it does nothing to the
// phased-out permanent, which stays phased out to phase in at its
// controller's next untap.

// TestAtEOTDelayedExileSkipsAPhasedOutToken pins Reg Barclay's "exile that
// token at the beginning of the next end step" (AtEOT$ Exile) over a token
// that phased out first.
func TestAtEOTDelayedExileSkipsAPhasedOutToken(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := ateotEngine(t, reg, "Reg Barclay, Fidgety Fantasist")

	barclay := ateotFind(t, e, "Reg Barclay, Fidgety Fantasist", 0)
	ateotTo(t, e, barclay, state.ZLibrary, state.ZBattlefield)
	bear := ateotFind(t, e, "Grizzly Bears", 0)
	bearZone := e.G.Obj(bear).Zone
	if bearZone != state.ZBattlefield {
		ateotTo(t, e, bear, bearZone, state.ZBattlefield)
	}
	if e.G.Obj(bear).Zone != state.ZBattlefield || e.G.Obj(barclay).Zone != state.ZBattlefield {
		t.Fatalf("precondition: the copy source is in %v and the copier in %v, want both on the battlefield",
			e.G.Obj(bear).Zone, e.G.Obj(barclay).Zone)
	}

	o := e.G.Obj(barclay)
	svars := o.Card.Faces[0].SVars
	copyToken := cards.ResolveSVar(svars, "TrigCopy")
	if copyToken == nil || copyToken.API != "CopyPermanent" || copyToken.ParamStr(cards.PKAtEOT) != "Exile" {
		t.Fatalf("precondition: Reg Barclay's copy body is %+v, want CopyPermanent with AtEOT$ Exile", copyToken)
	}
	resolveUnderProbe(e, func() *effects.Ctx {
		return &effects.Ctx{Source: barclay, Controller: 0, SVars: svars}
	}, copyToken)
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		if !offersObj(d, bear) {
			t.Fatalf("the copy target ask does not offer the Bear: %+v", d)
		}
		submitChoices(t, e, objOptionIndex(t, d, bear))
	}
	settlePassing(e)

	token := onlyBattlefieldToken(t, e)
	registered := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedRegister && ev.Obj == token {
			registered = true
		}
	}
	if !registered {
		t.Fatal("precondition: the token's AtEOT$ Exile registered no delayed trigger")
	}

	e.emit(events.Event{Kind: events.PhaseOut, Obj: token, Amount: 1})
	if !e.G.Obj(token).PhasedOut {
		t.Fatal("precondition: the token did not phase out")
	}
	replayCheck(t, e, cfg)

	ateotDriveToStep(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		passUntilStackEmpty(t, e, 20)
	}
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(token); got == nil || got.Zone != state.ZBattlefield || !got.PhasedOut {
		t.Fatalf("CR 702.26b: the delayed exile affected the phased-out token: %+v", got)
	}
	replayCheck(t, e, cfg)
}

// TestAtEOTDelayedDestroySkipsAPhasedOutToken pins Old Hob's "destroy it at
// the beginning of the next end step" (AtEOT$ Destroy) over the token it
// created once that token has phased out.
func TestAtEOTDelayedDestroySkipsAPhasedOutToken(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := ateotEngine(t, reg, "Old Hob, Alleycat Blues")
	hob := ateotFind(t, e, "Old Hob, Alleycat Blues", 0)
	ateotTo(t, e, hob, state.ZLibrary, state.ZBattlefield)

	ateotDriveToStep(t, e, e.G.Turn, e.G.Active, state.StepBeginCombat)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		passUntilStackEmpty(t, e, 20)
	}
	passUntilStackEmpty(t, e, 20)
	token := onlyBattlefieldToken(t, e)
	if len(e.G.Delayed) == 0 {
		t.Fatal("precondition: Old Hob's AtEOT$ Destroy registered no delayed trigger")
	}

	e.emit(events.Event{Kind: events.PhaseOut, Obj: token, Amount: 1})
	if !e.G.Obj(token).PhasedOut {
		t.Fatal("precondition: the token did not phase out")
	}
	replayCheck(t, e, cfg)

	ateotDriveToStep(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		passUntilStackEmpty(t, e, 20)
	}
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(token); got == nil || got.Zone != state.ZBattlefield || !got.PhasedOut {
		t.Fatalf("CR 702.26b: the delayed destroy affected the phased-out token: %+v", got)
	}
	replayCheck(t, e, cfg)
}

// TestAtEOTDelayedSacrificeSkipsAPhasedOutToken pins Kiki-Jiki's "sacrifice
// it at the beginning of the next end step" (AtEOT$ Sacrifice) over its
// token copy once that token has phased out.
func TestAtEOTDelayedSacrificeSkipsAPhasedOutToken(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Kiki-Jiki, Mirror Breaker"), lookup(t, reg, "Grizzly Bears")},
		[]*cards.Card{})
	kiki := moveByName(t, e, 0, "Kiki-Jiki, Mirror Breaker", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)

	e.priorityRound()
	submitChoices(t, e, abilityOption(t, e, kiki, 0).Index)
	answerKTarget(t, e, bear)
	passUntilStackEmpty(t, e, 20)
	token := findTokenCopyOf(t, e, e.G.Obj(bear).Card, bear)

	e.emit(events.Event{Kind: events.PhaseOut, Obj: token, Amount: 1})
	if !e.G.Obj(token).PhasedOut {
		t.Fatal("precondition: the token did not phase out")
	}
	replayCheck(t, e, cfg)

	driveToStepAll(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		passUntilStackEmpty(t, e, 30)
	}
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Obj(token); got == nil || got.Zone != state.ZBattlefield || !got.PhasedOut {
		t.Fatalf("CR 702.26b: the delayed sacrifice affected the phased-out token: %+v", got)
	}
	if got := e.G.Obj(bear); got == nil || got.Zone != state.ZBattlefield {
		t.Fatalf("the copied Bear left the battlefield: %+v", got)
	}
	replayCheck(t, e, cfg)
}

// battlefieldToken returns the one token on the battlefield, failing unless
// there is exactly one.
func onlyBattlefieldToken(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	var token state.ObjID
	for i := range e.G.Objs {
		if o := &e.G.Objs[i]; o.IsToken && o.Zone == state.ZBattlefield {
			if token != 0 {
				t.Fatalf("precondition: more than one battlefield token (%d and %d)", token, o.ID)
			}
			token = o.ID
		}
	}
	if token == 0 {
		t.Fatal("precondition: no token on the battlefield")
	}
	return token
}
