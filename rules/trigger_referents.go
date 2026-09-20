package rules

import (
	"strings"

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
		c.TriggerStack = ev.Obj
	case "DamageDone", "DamageDealtOnce", "DamageDoneOnce":
		// The damage source the causing event names: the published override
		// when a DamageSource$ emitter set one (Kediss' DamageAll with
		// DamageSource$ TriggeredSource resolves its own execute through
		// exactly this role), else the resolution/combat source. The
		// defending-player read below is combat-shaped by construction: an
		// override is never published during combat's assignment loop.
		c.TriggerSource = e.inFlightDamageSource()
		c.TriggerAmount = ev.Amount
		c.TriggerTarget = state.Target{Obj: ev.Obj}
		if ev.Obj == 0 {
			c.TriggerTarget = player(ev.Player)
		}
		if o := e.G.Obj(e.inFlightDamageSource()); o != nil && o.IsAttacking {
			c.DefendingPlayer = player(o.Attacking)
		}
	case "DamagePreventedOnce":
		// The prevention Note carries the prevented damage in Amount and the
		// damaged side in Obj/Player (rules/replacement.go's stored-prevention
		// arms). TriggerCount$DamageAmount reads TriggerAmount when the
		// trigger's DB$ PutCounter resolves (Selfless Squire's TrigPut).
		c.TriggerAmount = ev.Amount
		c.TriggerTarget = state.Target{Obj: ev.Obj}
		if ev.Obj == 0 {
			c.TriggerTarget = player(ev.Player)
		}
	case "Attacks", "AttackersDeclaredOneTarget", "AttackersDeclared":
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
			// Attacks reads ValidCard$; the AttackersDeclared family reads
			// ValidAttackers$ -- the same per-attacker filter under its own
			// name.
			spec := t.Params["ValidCard"]
			if spec == "" && t.Mode == "AttackersDeclared" {
				spec = t.Params["ValidAttackers"]
			}
			if (spec == "" && id == source) || (spec != "" && effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(source, e.controllerOf(source)))) {
				matches++
				c.TriggerCard = id
			}
		}
		if matches != 1 {
			c.TriggerCard = 0
		}
		c.TriggerSource = c.TriggerCard
	case "Taps":
		c.TriggerActivator = player(e.tapActor(ev))
	case "ChangesZone", "LandPlayed":
		c.TriggerCard = ev.Obj
	case "Drawn":
		c.TriggerCard = ev.Obj
		c.TriggerPlayer = player(ev.Player)
	case "LifeLost", "LifeLostAll":
		if p, amount, ok := lifeLoss(ev); ok {
			c.TriggerPlayer = player(p)
			c.TriggerAmount = amount
		}
	case "SpellCast", "AbilityCast", "SpellAbilityCast":
		c.TriggerCard = ev.Obj
		c.TriggerSource = e.protectionSource(ev.Obj)
		// TriggeredActivator is the player who cast or activated it, the
		// same ev.Player ValidActivatingPlayer$ is matched against
		// (Tangleroot: "that player adds {G}").
		c.TriggerActivator = player(ev.Player)
		// The activation arm (abcopy1): an AbilityPush event's Obj is the
		// SOURCE PERMANENT -- the ability's stack wrapper is minted inside
		// events.Apply and never travels on the event, so Remembered names the
		// battlefield permanent. Capture the minted wrapper here, at fire
		// time, when it is deterministically the topmost non-trigger ability
		// wrapper whose source is the triggering permanent (checkTriggers
		// runs synchronously inside emit immediately after the AbilityPush
		// applied, and a log-only replay folds the same AbilityPush, mints the
		// same id and re-runs this capture at the same point -- no event
		// schema change, the TriggerPaidX/TriggerConverge mechanism). Absent
		// for PutOnStack: a spell cast's ev.Obj IS the spell object.
		if ev.Kind == events.AbilityPush {
			c.TriggerAbility = e.abilityCastStackObject(ev.Obj)
		}
	case "Attached":
		// ev.Obj is the attaching Aura/Equipment, ev.IDs[0] the bearer it
		// became attached to (attachedMatches guarantees a bearer-bearing
		// Attach event reached this mode). The TriggerTarget role serves the
		// TriggeredTarget/TriggeredTargetController spellings (Bramble
		// Elemental's token owner); TriggerBearer -- a field ONLY this case
		// sets -- is what TriggeredTargetLKICopy (Enormous Energy Blade's
		// "tap that creature") resolves, so the bearer never masquerades as
		// another mode's TriggerTarget provenance (a BecomesTarget trigger's
		// TriggerTarget is its own source permanent; reading it as a bearer
		// would make Horobi destroy himself on every targeting).
		c.TriggerCard = ev.Obj
		if len(ev.IDs) > 0 {
			c.TriggerTarget = state.Target{Obj: ev.IDs[0]}
			c.TriggerBearer = ev.IDs[0]
		}
	case "Phase":
		c.TriggerPlayer = player(e.G.Active)
	case "Explores":
		// The explore record's roles (task explore1): TriggerCard is the
		// EXPLORER (what ValidCard$ matched), the same ChangesZone read.
		// The revealed card rode the record's IDs, but every corpus body
		// reads the trigger's own source or asks its own targets, so no
		// separate referent field is minted for it.
		c.TriggerCard = ev.Obj
	case "Exerted":
		// The Exert event names the exerted permanent (ev.Obj) and its
		// controller at exert time (ev.Player). TriggerCard is the exerted
		// permanent, so TriggeredCard/TriggeredCardLKICopy resolve against it
		// (Rohirrim Chargers' AttachedTo$ TriggeredCardLKICopy rider and the
		// general "that creature" spelling). triggerRemembered already seeds
		// Remembered with ev.Obj for any non-zero ev.Obj, so this adds the
		// dedicated role without changing the Remembered list.
		c.TriggerCard = ev.Obj
		c.TriggerPlayer = player(ev.Player)
	case "TapsForMana":
		// The ManaAdd event names the activating player, producing permanent,
		// produced type and amount without overloading Remembered. This mode's
		// matcher remains a separate primitive; retaining all four roles here
		// makes ReflectProperty$ Produced exact once that trigger is queued.
		c.TriggerPlayer = player(ev.Player)
		c.TriggerCard = ev.Obj
		c.TriggerSource = ev.Obj
		c.TriggerMana = ev.Counter
		c.TriggerAmount = ev.Amount
	}
	// CR 107.3m binds X when the trigger fires, not when it resolves. In
	// particular, an ETB trigger may remain on the stack after its permanent
	// dies, at which point Move has correctly cleared the object's live X.
	// Keep the event's card value with the rest of the trigger provenance.
	// CR 107.4f's converge colour count rides the same capture: a SpellCast
	// trigger's card carries the pay-time CastInfo stamp (rules/cast.go's
	// payCast) at fire time, and a spell countered before the trigger resolves
	// has had the stack->graveyard move clear it -- the snapshot is what lets
	// evalRefProperty's Converge property answer with the colours actually
	// spent, regardless of the spell's fate.
	if card := e.G.Obj(c.TriggerCard); card != nil {
		c.TriggerPaidX = card.X
		c.TriggerConverge = card.ConvergeColours
	}
	return c
}

// abilityCastStackObject is the fire-time twin of effects.changeXAbilityObject's
// scan (effects cannot import rules, so the scan is mirrored, not shared): the
// topmost non-trigger ability wrapper on the stack whose Source is the
// activating permanent. Called synchronously inside the AbilityPush emit,
// when that wrapper is exactly the stack top; 0 when no such wrapper exists
// (a stale registration) -- the role simply stays absent.
func (e *Engine) abilityCastStackObject(perm state.ObjID) state.ObjID {
	for i := len(e.G.Stack) - 1; i >= 0; i-- {
		o := e.G.Obj(e.G.Stack[i])
		if o == nil || o.Card != nil || o.Ability == nil || o.Source != perm {
			continue
		}
		if _, isTrig := state.TriggerOf(e.G, o); isTrig {
			continue
		}
		return o.ID
	}
	return 0
}

// targetSpecContext accepts the actual stack id, so simultaneous triggers of
// the same permanent cannot inherit one another's bindings. A prospective cast
// or an ordinary static has no entry and therefore no trigger context.
//
// The Resolve hook closes over the in-flight cast/activation proposal: a
// ValidTgts$ numeric bound naming "X" (Chthonian Nightmare's
// Creature.YouCtrl+cmcEQX) is evaluated at the announced {X} value once the
// cast-flow X ask has fixed it (CR 107.3i). Without it the bound could never
// resolve -- targetSpecContext had no resolver at all -- and an X-targeted
// ability offered no candidates at either the offer gate or the 601.2c ask.
// No resolver (or an in-flight cast that is not this source) still answers
// (0, false), which is numericPred's "recognised shape, unresolvable RHS
// never matches" — so every non-X name and every no-cast caller behaves
// exactly as before.
func (e *Engine) targetSpecContext(source, stack state.ObjID, you state.PlayerID) effects.SpecContext {
	sc := effects.SpecContext{You: you, Source: source, TriggerContext: e.triggerContexts[stack],
		Resolve: func(name string) (int32, bool) {
			if !strings.EqualFold(name, "X") {
				return 0, false
			}
			// In flight: the cast/activation proposal's announced value.
			if e.cast != nil && (e.cast.card == source || e.cast.stackObj == source) {
				return e.cast.x, true
			}
			// Resolving: the stack object's own recorded {X} (CastInfo's Amount,
			// CR 107.3m binds X when announced -- an ability resolving after the
			// cast flow closed still carries it on the stack object).
			if o := e.G.Obj(stack); o != nil {
				return o.X, true
			}
			return 0, false
		}}
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
