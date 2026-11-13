package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 702.62a: "If you cast a creature spell this way, it gains haste until
// you lose control of the spell or the permanent it becomes."
func TestSuspendedCreatureCastGainsHaste(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, "Name:Suspender\nManaCost:3 R\nTypes:Creature Human\nPT:2/2\nK:Suspend:1:R\nOracle:x\n"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MR] = 1
	castMode(t, e, id, "suspend")
	if o := e.G.Obj(id); o.Zone != state.ZExile || o.Counter("TIME") != 1 {
		t.Fatalf("suspend did not exile with one time counter: %+v", o)
	}
	e.beginTurn(0)
	if d := e.Pending(); d == nil || len(d.Options) != 2 || d.Options[0].Kind != "suspend_cast_yes" {
		t.Fatalf("final counter did not offer the may-cast: %+v", d)
	}
	submitChoices(t, e, 0)
	if e.G.Obj(id).Zone != state.ZStack {
		t.Fatalf("suspend cast left the card in %s, want stack", e.G.Obj(id).Zone)
	}
	e.resolveTop()
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("creature in %s, want battlefield", o.Zone)
	}
	if !e.HasKeyword(id, "Haste") {
		t.Fatalf("a creature cast through suspend must have haste (CR 702.62a)")
	}
}

// The grant ends when the caster loses control of the permanent, and a
// creature put onto the battlefield from exile (never cast) gets none.
func TestSuspendedCreatureHasteEndsWithControl(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, "Name:Suspender\nManaCost:3 R\nTypes:Creature Human\nPT:2/2\nK:Suspend:1:R\nOracle:x\n"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MR] = 1
	castMode(t, e, id, "suspend")
	e.beginTurn(0)
	submitChoices(t, e, 0)
	e.resolveTop()
	if !e.HasKeyword(id, "Haste") {
		t.Fatalf("precondition: the suspended cast has haste")
	}
	e.emit(events.Event{Kind: events.ControlChange, Obj: id, Player: 1})
	if o := e.G.Obj(id); o.Controller != 1 {
		t.Fatalf("precondition: control did not change: %+v", o)
	}
	if e.HasKeyword(id, "Haste") {
		t.Fatalf("haste outlived the caster's control (CR 702.62a)")
	}
}

func TestSuspendedCardPutOntoBattlefieldHasNoHaste(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, "Name:Suspender\nManaCost:3 R\nTypes:Creature Human\nPT:2/2\nK:Suspend:1:R\nOracle:x\n"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MR] = 1
	castMode(t, e, id, "suspend")
	if o := e.G.Obj(id); o.Zone != state.ZExile || o.CastFlags&state.FlagSuspend == 0 {
		t.Fatalf("precondition: suspended card carries its provenance: %+v", o)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZExile, To: state.ZBattlefield})
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: card not on the battlefield: %+v", o)
	}
	if e.HasKeyword(id, "Haste") {
		t.Fatalf("a suspended card that was not cast gains no haste")
	}
}

// The grant belongs to the permanent the suspended cast became: when it
// returns to the battlefield after a bounce it is a new object (CR 400.7) and
// has no haste.
func TestSuspendedCreatureHasteEndsWhenItLeavesTheBattlefield(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, "Name:Suspender\nManaCost:3 R\nTypes:Creature Human\nPT:2/2\nK:Suspend:1:R\nOracle:x\n"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MR] = 1
	castMode(t, e, id, "suspend")
	e.beginTurn(0)
	submitChoices(t, e, 0)
	e.resolveTop()
	if !e.HasKeyword(id, "Haste") {
		t.Fatalf("precondition: the suspended cast has haste")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZHand})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the creature did not return: %+v", o)
	}
	if e.HasKeyword(id, "Haste") {
		t.Fatalf("the new object inherited the old one's suspend haste (CR 400.7)")
	}
}
