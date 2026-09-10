package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// triggerReferents captures roles from the causing event, NOT from the trigger's
// later target answer or from Remembered (which scripts may overwrite). Nothing
// is added to the event schema: replay discovers the same bindings from the same
// events. A role not provided by a supported trigger mode remains absent.
func (e *Engine) triggerReferents(t cards.Trigger, source state.ObjID, ev events.Event) effects.TriggerContext {
	var c effects.TriggerContext
	player := func(p state.PlayerID) state.Target { return state.Target{Player: p, IsPlayer: true} }
	switch t.Mode {
	case "BecomesTarget":
		// This matcher fires only for its own source being targeted, even when
		// the causing spell chose several targets. ev.Obj is that spell/ability.
		c.TriggerTarget = state.Target{Obj: source}
		c.TriggerSource = e.protectionSource(ev.Obj)
	case "DamageDone", "DamageDealtOnce":
		c.TriggerSource = e.damaging
		c.TriggerTarget = state.Target{Obj: ev.Obj}
		if ev.Obj == 0 {
			c.TriggerTarget = player(ev.Player)
		}
		if o := e.G.Obj(e.damaging); o != nil && o.IsAttacking {
			c.DefendingPlayer = player(o.Attacking)
		}
	case "Attacks":
		c.DefendingPlayer = player(ev.Player)
		// The current engine batches attackers per defender. Preserve the
		// defending player, but do not invent a singular TriggeredCard when
		// several matching attackers caused this one queued trigger.
		matches := 0
		for _, id := range ev.IDs {
			spec := t.Params["ValidCard"]
			if (spec == "" && id == source) || (spec != "" && effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(source, e.controllerOf(source)))) {
				matches++
				c.TriggerCard = id
			}
		}
		if matches != 1 {
			c.TriggerCard = 0
		}
		c.TriggerSource = c.TriggerCard
	case "ChangesZone", "LandPlayed":
		c.TriggerCard = ev.Obj
	case "SpellCast", "AbilityCast", "SpellAbilityCast":
		c.TriggerCard = ev.Obj
		c.TriggerSource = e.protectionSource(ev.Obj)
	case "Phase":
		c.TriggerPlayer = player(e.G.Active)
	}
	return c
}

// targetSpecContext accepts the actual stack id, so simultaneous triggers of
// the same permanent cannot inherit one another's bindings. A prospective cast
// or an ordinary static has no entry and therefore no trigger context.
func (e *Engine) targetSpecContext(source, stack state.ObjID, you state.PlayerID) effects.SpecContext {
	return effects.SpecContext{You: you, Source: source, TriggerContext: e.triggerContexts[stack]}
}
