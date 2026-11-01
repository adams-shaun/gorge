package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// kwhStorm is the Storm head the granted-storm walk reads.
var kwhStorm = newKWHead("Storm")

// stormGrantedPayload is the KeywordTriggerPush Counter a granted Storm
// trigger carries; events.Apply rebuilds the printed expansion's body from it
// (engine/events/apply_trigger.go). The trailing colon keeps it from aliasing the
// "__kwStorm" SVar a printed bare K:Storm line mints.
const stormGrantedPayload = "__kwStorm:"

// checkGrantedStormTriggers synthesizes Storm's cast trigger (CR 702.40a:
// "When you cast this spell, copy it for each other spell that was cast
// before it this turn") for a spell that currently HAS the keyword but does
// not print it: a layer-6 grant (Prismari, the Inspiration's "Instant and
// sorcery spells you cast have storm") gives the spell the same rules text as
// a printed keyword, and the printed K:Storm expansion (engine/cards/kw_storm.go)
// only covers printed lines. It is the Demonstrate shape exactly
// (checkGrantedDemonstrateTriggers): the synthesized trigger reuses the
// printed expansion's trigger (Mode$ SpellCast, ValidCard$ Card.Self,
// TriggerZones$ Stack), the body rides the push's __kwStorm: payload, and a
// face that PRINTS Storm is skipped (its expansion owns the line). A copy
// emits StackCopy, not PutOnStack, so the synthesis cannot fire on its own
// output.
func (e *Engine) checkGrantedStormTriggers(observer *Engine, id state.ObjID, o *state.Object, f *cards.Face, ev events.Event, objLKI *state.Object) {
	if ev.Kind != events.PutOnStack || id != ev.Obj {
		return
	}
	if !e.hasKeywordH(id, kwhStorm) || f.HasKeywordRef(kwRefStorm) {
		return
	}
	// The Keyword$ Storm param is what triggerReferents' fire-time capture
	// keys on (CR 702.40a: the copy count is fixed when the trigger fires,
	// not at resolution) -- the same stamp addKeywordTrigger puts on the
	// printed expansion's trigger.
	t := cards.Trigger{Mode: "SpellCast", Params: map[string]string{
		"Mode": "SpellCast", "ValidCard": "Card.Self", "TriggerZones": "Stack", "TriggerDescription": "Storm",
		"Keyword": "Storm",
	}, Effect: stormGrantedBody()}
	if !observer.triggerMatches(t, id, ev, objLKI) {
		return
	}
	key := triggerKey{Source: id, Idx: -1}
	if e.triggerFireCount == nil {
		e.triggerFireCount = map[triggerKey]int32{}
	}
	if e.triggerFireCount[key] >= maxTriggerFires {
		return
	}
	e.triggerFireCount[key]++
	e.queueTrigger(observer, pendingTrigger{
		Source:     id,
		Controller: o.Controller,
		Storm:      true,
		Ctx: effects.NewCtx(id, o.Controller, effects.CtxInit{
			Remembered:     triggerRemembered(ev, id),
			LKI:            objLKI,
			TriggerContext: observer.triggerReferents(t, id, ev, objLKI),
		}),
	}, ev, nil)
}

// stormGrantedBody is the printed K:Storm expansion's body
// (engine/cards/kw_storm.go), the one events.Apply rebuilds from the payload.
func stormGrantedBody() *cards.SA {
	return events.StormGrantedBody()
}

// pushGrantedStorm emits a queued granted-storm trigger: the Demonstrate
// push shape, with the cast spell riding IDs as Remembered because Defined$
// TriggeredSpellAbility reads the triggering spell off it.
func (e *Engine) pushGrantedStorm(pt pendingTrigger) {
	if int(pt.Controller) >= len(e.G.Players) || e.G.Players[pt.Controller].Lost {
		return
	}
	ids := make([]state.ObjID, 0, len(pt.Ctx.Remembered))
	for _, tgt := range pt.Ctx.Remembered {
		if tgt.IsPlayer {
			ids = append(ids, state.PlayerRef(tgt.Player))
			continue
		}
		ids = append(ids, tgt.Obj)
	}
	stackLen := len(e.G.Stack)
	e.emit(events.Event{Kind: events.KeywordTriggerPush, Player: pt.Controller,
		Obj: pt.Source, Counter: stormGrantedPayload, IDs: ids, Text: "storm ability"})
	if len(e.G.Stack) > stackLen {
		id := e.G.Stack[len(e.G.Stack)-1]
		if e.triggerContexts == nil {
			e.triggerContexts = make(map[state.ObjID]effects.TriggerContext)
		}
		e.triggerContexts[id] = pt.Ctx.TriggerContext
	}
	e.drainAwaitsTarget = e.Pending() != nil
}
