package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// redirectBoard puts a Murder aimed at seat 0's creature A on the stack beside the
// redirect, as Deflecting Swat's test does, and resolves the redirect.
func redirectBoard(t *testing.T, redirect string, extra bool) (e *Engine, murder *state.Object, a, b state.ObjID) {
	t.Helper()
	e = stealEngine(t, 738)
	a = onBoard(t, e, 0, "Name:A\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	if extra {
		b = onBoard(t, e, 0, "Name:B\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	}
	murder = e.G.AddObject(card(t, "Name:Murder\nTypes:Instant\nManaCost:B\nA:SP$ Destroy | ValidTgts$ Creature.OppCtrl\nOracle:x\n"), 1)
	spell := e.G.AddObject(choiceCorpusCard(t, redirect), 0)
	for _, id := range []state.ObjID{murder.ID, spell.ID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZStack})
	}
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: murder.ID, IDs: []state.ObjID{a}})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: spell.ID, IDs: []state.ObjID{murder.ID}})
	resolveUnderProbe(e, func() *effects.Ctx {
		return &effects.Ctx{Source: spell.ID, Controller: 0, Targets: []state.Target{{Obj: murder.ID}},
			TargetsOffered: true, SVars: spell.Face().SVars}
	}, spell.Face().SpellAbility())
	return e, murder, a, b
}

// CR 115.7a: "change the target" moves it to another legal target, so the current
// target isn't an answer, and with no other legal target nothing is asked or changed.
func TestChangeTheTargetOffersAnotherTarget(t *testing.T) {
	t.Parallel()
	e, murder, a, b := redirectBoard(t, "Deflection", true)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 1 {
		t.Fatalf("Deflection did not ask: %+v", d)
	}
	if offersObj(d, a) {
		t.Fatalf("offered the current target: %+v", d.Options)
	}
	submitChoices(t, e, objOptionIndex(t, d, b))
	if got := e.G.Obj(murder.ID).Targets; len(got) != 1 || got[0].Obj != b {
		t.Fatalf("targets = %+v, want creature %d", got, b)
	}

	e, murder, a, _ = redirectBoard(t, "Deflection", false)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("asked with no other legal target: %+v", d)
	}
	if got := e.G.Obj(murder.ID).Targets; len(got) != 1 || got[0].Obj != a {
		t.Fatalf("targets = %+v, want the original %d", got, a)
	}
}

// CR 115.7d: "you may choose new targets" leaves a target unchanged for the asking.
func TestChooseNewTargetsMayLeaveTheTarget(t *testing.T) {
	t.Parallel()
	e, murder, a, _ := redirectBoard(t, "Redirect", true)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 0 {
		t.Fatalf("Redirect did not ask an optional choice: %+v", d)
	}
	submitChoices(t, e)
	if got := e.G.Obj(murder.ID).Targets; len(got) != 1 || got[0].Obj != a {
		t.Fatalf("targets = %+v, want the original %d", got, a)
	}
}
