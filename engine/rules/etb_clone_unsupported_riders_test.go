package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Flesh Duplicate's conditional-keyword rider (AddKeywords$ IfNew
// Vanishing:3 -- "vanishing 3 if that creature doesn't have vanishing",
// CR 707.9b) is inside the ETB clone whitelist: the inner keyword is a
// single-word head cards expands on the copy basis face, so the election is
// posed and the grant rides the ClonePermanent event's "ifnew " payload.

func fleshDuplicateEntry(t *testing.T, templateName string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := handEngine(t, corpusAlternativeCard(t, "Flesh Duplicate"))
	var tplID state.ObjID
	if templateName != "" {
		tpl := e.G.AddObject(corpusAlternativeCard(t, templateName), 1)
		tpl.Zone = state.ZBattlefield
		e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{tpl.ID})
		tplID = tpl.ID
	}
	id := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	return e, id, tplID
}

func etbElectionOption(t *testing.T, d *decision.Decision, tplID state.ObjID) int {
	t.Helper()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
		t.Fatalf("copy election = %+v", d)
	}
	for _, o := range d.Options {
		if tplID == 0 && o.Kind == "clone" && o.Obj == 0 && o.Label == "Enter as itself" {
			return o.Index
		}
		if o.Obj == tplID {
			return o.Index
		}
	}
	t.Fatalf("template %d absent from election: %+v", tplID, d.Options)
	return -1
}

// TestFleshDuplicateETBGrantedVanishing pins the grant itself: taking the
// copy of a vanishing-less creature enters the copy with the granted
// vanishing's three time counters (its printed-shaped entry machinery), the
// upkeep trigger removes one, and the last-counter trigger sacrifices.
func TestFleshDuplicateETBGrantedVanishing(t *testing.T) {
	t.Parallel()
	e, id, tplID := fleshDuplicateEntry(t, "Colossal Dreadmaw")
	idx := etbElectionOption(t, e.Pending(), tplID)
	submitChoices(t, e, idx)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Colossal Dreadmaw" {
		t.Fatalf("entry = %+v, want a Colossal Dreadmaw copy", o)
	}
	if got := o.Counter("TIME"); got != 3 {
		t.Fatalf("granted vanishing entered with %d TIME counters, want 3", got)
	}
	if !hasEvent(e, events.ClonePermanent, id) {
		t.Fatal("election was taken but Clone handler never ran")
	}
	// The rider must not leak onto the template.
	if tpl := e.G.Obj(tplID); tpl != nil && tpl.Counter("TIME") != 0 {
		t.Fatalf("template gained %d TIME counters, want none", tpl.Counter("TIME"))
	}
	// The granted vanishing is live: controller's upkeep removes a counter
	// through the ordinary Phase trigger.
	e.pendingTriggers = nil
	e.G.Active, e.G.Priority = 0, 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("upkeep put %d stack objects, want one vanishing trigger", len(e.G.Stack))
	}
	e.resolveTop()
	if got := o.Counter("TIME"); got != 2 {
		t.Fatalf("after upkeep removal TIME = %d, want 2", got)
	}
	// Seed the last two removals through the event log and check the
	// last-counter sacrifice.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "TIME", Amount: -2})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("last-counter removal put %d stack objects, want one sacrifice trigger", len(e.G.Stack))
	}
	e.resolveTop()
	if got := e.G.Obj(id); got == nil || got.Zone != state.ZGraveyard {
		t.Fatalf("after last-counter sacrifice zone = %+v, want graveyard", got)
	}
}

// TestFleshDuplicateConditionalSilentOnVanishingTemplate pins the
// conditional's negative arm: the copied template already carries vanishing,
// so the rider is silent -- the copy basis face carries exactly ONE
// Vanishing:3 line, the template's own. The entry plan reads the answered
// template's printed enters-with counters (CR 707.9b), so the copy enters with
// the Hermit's three time counters; the condition comparison still ran, which
// is what keeps the granted counters from ALSO being placed -- six -- and the
// single upkeep queue shows one vanishing, not two.
func TestFleshDuplicateConditionalSilentOnVanishingTemplate(t *testing.T) {
	t.Parallel()
	e, id, tplID := fleshDuplicateEntry(t, "Deep Forest Hermit")
	idx := etbElectionOption(t, e.Pending(), tplID)
	submitChoices(t, e, idx)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Deep Forest Hermit" {
		t.Fatalf("entry = %+v, want a Deep Forest Hermit copy", o)
	}
	if got := o.Counter("TIME"); got != 3 {
		t.Fatalf("copy of a vanishing creature entered with %d TIME counters, want the template's 3 (not 0, not the rider's extra 3)", got)
	}
	vanishing := 0
	for _, kw := range o.Face().Keywords {
		if cards.KeywordHead(kw) == "Vanishing" {
			vanishing++
		}
	}
	if vanishing != 1 {
		t.Fatalf("copy carries %d vanishing lines, want exactly the template's one", vanishing)
	}
	// One vanishing's machinery, no rider duplication: the upkeep step queues
	// the single counter-removal trigger (its IsPresent$ counters_GE1_TIME
	// gate reads three). The copy entered with the template's own ETB trigger
	// on the stack (CR 707.9); resolve it first so only the upkeep step is
	// under test.
	e.resolveTop()
	e.pendingTriggers = nil
	e.G.Active, e.G.Priority = 0, 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("upkeep put %d stack objects, want exactly one vanishing removal trigger", len(e.G.Stack))
	}
}

// TestFleshDuplicateETBDeclineEntersAsItself pins the decline arm and the
// SBA consequence: entering as itself is the printed 0/0 Flesh Duplicate
// with no vanishing, which dies to the creature-toughness check.
func TestFleshDuplicateETBDeclineEntersAsItself(t *testing.T) {
	t.Parallel()
	e, id, _ := fleshDuplicateEntry(t, "Colossal Dreadmaw")
	idx := etbElectionOption(t, e.Pending(), 0)
	submitChoices(t, e, idx)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZGraveyard || o.Face().Name != "Flesh Duplicate" {
		t.Fatalf("declined entry = %+v, want the printed 0/0 in the graveyard", o)
	}
	if hasEvent(e, events.ClonePermanent, id) {
		t.Fatal("declined election still copied")
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == id && ev.Counter == "TIME" {
			t.Fatalf("declined entry placed counters: %+v", ev)
		}
	}
}
