package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// meleeRemembered captures one player reference per distinct opponent attacked
// in the WHOLE declaration. The Pump body reads this capture (cards.MeleePumpCount)
// snapshot after the ability is pushed and later resolves, even if an attacker
// or defender leaves combat in the meantime. A synthetic single-defender event
// has no declaration scratch and captures just that event's defending seat.
func (e *Engine) meleeRemembered(ev events.Event) []state.Target {
	defenders := e.declaredDefenders
	if len(defenders) == 0 {
		defenders = []state.PlayerID{ev.Player}
	}
	out := make([]state.Target, 0, len(defenders))
	for _, p := range defenders {
		out = append(out, state.Target{Player: p, IsPlayer: true})
	}
	return out
}

// checkGrantedAttackPumpTriggers synthesizes the granted instances of the two
// attack-triggered pumps, Melee then Exalted, that no live printed trigger
// already represents (grantedKeywordInstances). Derived preserves duplicates:
// two Titanias grant two Melee instances to another creature, and a printed
// keyword plus a layer-6 grant likewise triggers twice (CR 702.121a,
// 702.83b). Every Melee instance gets its own stack object with the same
// declaration-wide opponent snapshot. Exalted's holder need not attack
// (CR 702.83a: a creature its controller controls attacks alone), so it is
// not gated on the object being an attacker.
func (e *Engine) checkGrantedAttackPumpTriggers(observer *Engine, id state.ObjID, o *state.Object, f *cards.Face, ev events.Event, objLKI *state.Object) {
	if ev.Kind != events.DeclareAttackers {
		return
	}
	if slices.Contains(ev.IDs, id) {
		printedLive := !e.faceDownPrintedHides(o) && !observer.printedAbilitiesLost(o)
		instances := grantedKeywordInstances(e.Derived(id).Keywords, f.Triggers, "Melee", printedLive)
		t := cards.Trigger{Mode: "Attacks", Params: map[string]string{"Mode": "Attacks", "ValidCard": "Card.Self"}}
		if instances > 0 && observer.triggerMatches(t, id, ev, objLKI) {
			key := triggerKey{Source: id, Idx: -1}
			if e.triggerFireCount == nil {
				e.triggerFireCount = map[triggerKey]int32{}
			}
			for i := 0; i < instances && e.triggerFireCount[key] < maxTriggerFires; i++ {
				e.triggerFireCount[key]++
				r := e.meleeRemembered(ev)
				e.queueTrigger(observer, pendingTrigger{
					Source: id, Controller: o.Controller, Idx: -1, Melee: true,
					cause: causeOf(boardOf(observer), t, id, ev, 0),
					Ctx: effects.NewCtx(id, o.Controller, effects.CtxInit{Remembered: r,
						Captured: r, LKI: objLKI, TriggerContext: observer.triggerReferents(t, id, ev, objLKI)}),
				}, ev, []state.ObjID{id})
			}
		}
	}
	if !e.hasKeywordH(id, kwhExalted) {
		return
	}
	printedLive := !e.faceDownPrintedHides(o) && !observer.printedAbilitiesLost(o)
	instances := grantedKeywordInstances(e.Derived(id).Keywords, f.Triggers, "Exalted", printedLive)
	if instances <= 0 {
		return
	}
	t, ok := grantedExaltedTrigger()
	if !ok || !observer.triggerMatches(t, id, ev, objLKI) {
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
