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
func (e *Engine) triggerReferents(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) effects.TriggerContext {
	var c effects.TriggerContext
	player := func(p state.PlayerID) state.Target { return state.Target{Player: p, IsPlayer: true} }
	// CR 603.10a: a card that left the battlefield keeps, for this trigger,
	// the controller it had there. Recorded here so it travels with the
	// ability onto the stack (triggerContexts) and every "that card's
	// controller" read -- effects.TriggeredCardController -- sees it.
	if lki != nil && lki.ID == ev.Obj && leftBattlefield(ev) {
		c.TriggerCardController = player(lki.Controller)
	}
	switch t.Mode {
	case "BecomesTarget":
		// This matcher fires only for its own source being targeted, even when
		// the causing spell chose several targets. ev.Obj is that spell/ability.
		c.TriggerTarget = state.Target{Obj: source}
		c.TriggerSource = e.protectionSource(ev.Obj)
	case "DamageDone", "DamageDealtOnce", "DamageDoneOnce":
		c.TriggerSource = e.damaging
		c.TriggerAmount = ev.Amount
		c.TriggerTarget = state.Target{Obj: ev.Obj}
		if ev.Obj == 0 {
			c.TriggerTarget = player(ev.Player)
		}
		if o := e.G.Obj(e.damaging); o != nil && o.IsAttacking {
			c.DefendingPlayer = player(o.Attacking)
		}
	case "Attacks", "AttackersDeclaredOneTarget":
		c.DefendingPlayer = player(ev.Player)
		c.AttackedTarget = player(ev.Player)
		if len(ev.IDs) > 0 {
			c.AttackingPlayer = player(e.controllerOf(ev.IDs[0]))
		}
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
	case "Taps", "TapsForMana":
		c.TriggerActivator = player(e.tapActor(ev))
	case "ChangesZone", "LandPlayed":
		c.TriggerCard = ev.Obj
	case "SpellCast", "AbilityCast", "SpellAbilityCast":
		c.TriggerCard = ev.Obj
		c.TriggerSource = e.protectionSource(ev.Obj)
		// TriggeredActivator is the player who cast or activated it, the
		// same ev.Player ValidActivatingPlayer$ is matched against
		// (Tangleroot: "that player adds {G}").
		c.TriggerActivator = player(ev.Player)
	case "Phase":
		c.TriggerPlayer = player(e.G.Active)
	}
	return c
}

// targetSpecContext accepts the actual stack id, so simultaneous triggers of
// the same permanent cannot inherit one another's bindings. A prospective cast
// or an ordinary static has no entry and therefore no trigger context.
func (e *Engine) targetSpecContext(source, stack state.ObjID, you state.PlayerID) effects.SpecContext {
	sc := effects.SpecContext{You: you, Source: source, TriggerContext: e.triggerContexts[stack]}
	// The stack object's Remembered (the trigger-captured set for a
	// triggered ability) feeds the IsRemembered predicate at offer/placement
	// time, exactly as the resolution's own Ctx feeds it later -- Forge's
	// IsRemembered is a property of the host card's remembered list either
	// way. A cast proposal (stack == the card) and an ability proposal
	// (stack == 0) carry no remembered set, so this changes nothing for them.
	if o := e.G.Obj(stack); o != nil {
		sc.Remembered = append(sc.Remembered, o.Remembered...)
	}
	return sc
}
