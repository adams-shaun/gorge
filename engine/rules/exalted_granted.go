package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// checkGrantedExaltedTriggers synthesizes Exalted's attack trigger (CR
// 702.83a) for each instance of the keyword the object holds beyond its
// printed lines: a layer-6 grant (Sublime Archangel's "Other creatures you
// control have exalted") gives the creature the same rules text as a printed
// keyword, and the printed K:Exalted expansion (engine/cards/kw_exalted.go)
// covers only printed lines. Each instance triggers separately (CR 702.83b,
// 113.2c): the Derived list keeps duplicates, so a printed Exalted plus a
// grant, or two Archangels' grants, is one trigger per instance. The
// synthesized trigger reuses the printed shape (Attacks, ValidCard$
// Creature.YouCtrl, Alone$ True), so it matches and expands per attacker the
// way the printed line does, and its body is the printed Pump of the
// triggering attacker, rebuilt by events.Apply from the KeywordTriggerPush
// payload alone.
func (e *Engine) checkGrantedExaltedTriggers(observer *Engine, id state.ObjID, o *state.Object, f *cards.Face, ev events.Event, objLKI *state.Object) {
	if ev.Kind != events.DeclareAttackers || len(ev.IDs) == 0 {
		return
	}
	if !e.hasKeywordH(id, kwhExalted) {
		return
	}
	instances := 0
	for _, kw := range e.Derived(id).Keywords {
		if cards.KeywordHead(kw) == "Exalted" {
			instances++
		}
	}
	// A printed line is queued by the ordinary trigger walk only while the
	// printed face's abilities exist: not face down (CR 708.2) nor after the
	// object lost its abilities (CR 613.1f), where the derived list holds
	// only grants and every one is an instance queued here.
	if !e.faceDownPrintedHides(o) && !observer.printedAbilitiesLost(o) {
		for _, t := range f.Triggers {
			if t.Mode == "Attacks" && t.ParamStr(cards.PKKeyword) == "Exalted" {
				instances--
			}
		}
	}
	if instances <= 0 {
		return
	}
	t, ok := cards.ParseTriggerLine("Mode$ Attacks | ValidCard$ Creature.YouCtrl | Alone$ True")
	if !ok {
		return
	}
	if !observer.triggerMatches(t, id, ev, objLKI) {
		return
	}
	pt := pendingTrigger{
		Source: id, Controller: o.Controller, Idx: -1, Exalted: true,
		cause: causeOf(boardOf(observer), t, id, ev, 0),
		Ctx: effects.NewCtx(id, o.Controller, effects.CtxInit{Remembered: triggerRemembered(ev, id), LKI: objLKI,
			TriggerContext: observer.triggerReferents(t, id, ev, objLKI)}),
	}
	insts := e.grantedTriggerInstances(observer, &t, id, ev, pt)
	var all []pendingTrigger
	for range instances {
		all = append(all, slices.Clone(insts)...)
	}
	e.appendReservedTriggers(triggerKey{Source: id, Idx: -1}, all)
}
