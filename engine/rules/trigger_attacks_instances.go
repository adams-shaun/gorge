package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/trigmatch"
	"github.com/adams-shaun/gorge/engine/state"
)

// attacksTriggerInstances expands one queued Mode$ Attacks trigger into one
// instance per attacker its ValidCard$ matched (CR 508.3a: "Whenever [a
// creature] attacks" triggers if that creature is declared as an attacker;
// CR 603.2c: one event with several occurrences triggers repeatedly). The
// matched set is the same walk the boolean matcher ran, so no instance can
// name an attacker the match did not admit. Attackers stay in declaration
// order, and each instance carries its own Panharmonicon echoes.
//
// A nil result means the event admitted no attacker, and the caller keeps its
// generic single-instance append. A queue site that does not expand (a
// delayed registration) leaves TriggerCard absent and keeps the historical
// Remembered fallback.
func attacksTriggerInstances(board trigmatch.Board, t *cards.Trigger, id state.ObjID, ev events.Event, pt pendingTrigger, echoes int) []pendingTrigger {
	matched := trigmatch.AttacksMatchedAttackers(board, *t, id, ev)
	if len(matched) == 0 {
		return nil
	}
	insts := make([]pendingTrigger, 0, len(matched)*(echoes+1))
	for _, attacker := range matched {
		inst := pt
		inst.Ctx.TriggerContext.TriggerCard = attacker
		inst.Ctx.TriggerContext.TriggerSource = attacker
		// The AbilityTriggered marker's "own" bit is per instance: only the
		// attacker that IS the source caused that instance. causeOf answers
		// for the line's source as an attacker, so clear it on every other
		// instance (a self-inclusive non-self spec such as Creature.YouCtrl).
		if inst.cause.set && attacker != id {
			inst.cause.own = false
		}
		for k := 0; k <= echoes; k++ {
			insts = append(insts, inst)
		}
	}
	return insts
}

// appendReservedTriggers appends insts to the pending queue, spending one
// triggerFireCount use per queued instance (the cascade bound) and stopping at
// maxTriggerFires. Per-instance reservation keeps the bound exact when one
// event expands into several Mode$ Attacks instances.
func (e *Engine) appendReservedTriggers(key triggerKey, insts []pendingTrigger) {
	if e.triggerFireCount == nil {
		e.triggerFireCount = map[triggerKey]int32{}
	}
	for _, inst := range insts {
		if e.triggerFireCount[key] >= maxTriggerFires {
			return
		}
		e.triggerFireCount[key]++
		e.pendingTriggers = append(e.pendingTriggers, inst)
	}
}
