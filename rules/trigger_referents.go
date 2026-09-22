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
	case "RolledDie", "RolledDieOnce":
		// The canonical roll Note (effects/dice.go): the per-die DieRollNote
		// for Mode$ RolledDie and the per-resolution DieRollBatchNote for
		// Mode$ RolledDieOnce. Both name the roller (ev.Player) and the
		// reported result (ev.Amount). TriggerResult is what the
		// TriggerCount$Result head answers at resolution (Mr. House's
		// BranchConditionSVar$ reads it long after the RollDice resolution that
		// produced it has finished), TriggerResultMax is the
		// TriggerCountMax$Result head (Farideh), and TriggerPlayer is the
		// roller, so a body reading "that player" resolves the seat that
		// rolled. Each decoder rejects the other mode's Note, so the two
		// collectors can never cross-fire.
		if roller, _, _, result, ok := effects.DieRollResult(ev); ok {
			c.TriggerPlayer = player(roller)
			c.TriggerResult = result
			c.TriggerResultMax = result
		}
		if roller, _, maxResult, result, ok := effects.DieRollBatchResult(ev); ok {
			c.TriggerPlayer = player(roller)
			c.TriggerResult = result
			c.TriggerResultMax = maxResult
		}
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
	case "CounterAddedOnce":
		// The batch size the body reads as TriggerCount$Amount (Simic
		// Ascendancy's "put that many growth counters"): one CounterChange
		// event carries the whole placement batch in Amount, and ev.Obj is
		// the permanent the counters landed on.
		c.TriggerCard = ev.Obj
		c.TriggerAmount = ev.Amount
	case "CounterPlayerAddedAll":
		// The batch "whenever you put one or more counters on ..." mode's
		// roles (Generous Patron, Rikku, Kros, All Will Be One): the
		// recipient permanent is ev.Obj -- triggerRemembered already seeds
		// Remembered with it, so Defined$ TriggeredObjectLKICopy (Rikku's
		// RememberObjects$ on the DB body) resolves against it -- and the
		// batch size rides TriggerAmount for the count head TriggerCount$Amount
		// (All Will Be One's "deals that much damage" NumDmg$ X). A player
		// recipient (PlayerCounterChange, The Great Goblin's "or player")
		// carries no object: the recipient player is the TriggerTarget role
		// and the object roles stay absent rather than pointing at the
		// trigger's source.
		c.TriggerCard = ev.Obj
		if ev.Obj == 0 {
			c.TriggerTarget = player(ev.Player)
		}
		c.TriggerAmount = ev.Amount
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
	case "LifeGained":
		// The gaining player and the gained magnitude: TriggerCount$LifeAmount
		// (Prize Pig's CounterNum$ Y) reads both off this context.
		if ev.Kind == events.LifeChange && ev.Amount > 0 && int(ev.Player) >= 0 && int(ev.Player) < len(e.G.Players) {
			c.TriggerPlayer = player(ev.Player)
			c.TriggerAmount = ev.Amount
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
	case "Exploited":
		// The exploit record's roles (task exploit1): the EXPLOITING creature
		// is ev.Obj and rides TriggerSource (Colonel Autumn's team watch has
		// its own body, but a body reading TriggeredSource/TriggeredCard gets
		// a defined referent); the EXPLOITED creature is ev.IDs[0] and rides
		// TriggerCard, so TriggeredCard/TriggeredCardLKICopy resolve against
		// what the exploiter sacrificed (a "that creature" body).
		c.TriggerSource = ev.Obj
		if len(ev.IDs) > 0 {
			c.TriggerCard = ev.IDs[0]
		}
		c.TriggerPlayer = player(ev.Player)
	case "Enlisted":
		// The Enlist event names the ATTACKING creature that enlisted (ev.Obj,
		// the trigger's source for ValidCard$ Card.Self) and the creature it
		// tapped in ev.IDs[0]. TriggerCard is the attacker (so
		// TriggeredCard/TriggeredCardLKICopy resolve against it, the Exerted
		// shape) and TriggerEnlisted is the enlisted creature, what
		// ValidEnlisted$ matched and what Defined$ TriggeredEnlisted reads
		// (Goblin Morale Sergeant's conjured duplicate).
		c.TriggerCard = ev.Obj
		c.TriggerSource = ev.Obj
		c.TriggerPlayer = player(ev.Player)
		if len(ev.IDs) > 0 {
			c.TriggerEnlisted = ev.IDs[0]
		}
	case "Connives":
		// The connive record's roles (task connive1): TriggerCard is the
		// CONNIVER (what ValidCard$ matched), the same ChangesZone read. The
		// discarded cards rode the record's IDs, but every corpus body reads
		// the conniving creature or asks its own targets, so no separate
		// referent field is minted for them.
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
	case "Vote":
		// The canonical vote-finished carrier (effects/vote.go): the raw
		// ballots ride the Note as player refs (each Pair is
		// [PlayerRef(voter), pick+1]) and are RE-SPLIT here against the
		// trigger SOURCE'S controller -- "a choice you voted for" means the
		// carrier permanent's controller's own ballot, never the vote
		// caster's (the two differ whenever an opponent casts the vote, the
		// ordinary multiplayer case). The resulting sets are bound so the
		// resolution-time spellings -- Defined$
		// TriggeredOpponentVotedSame/TriggeredOpponentVotedDiff and the count
		// ref TriggeredPlayersOpponentVotedDiff$Amount -- read them long after
		// the event, from this per-stack capture. List$ gates the BINDING (the
		// referent scope, never the firing -- see voteMatches): a spelling
		// whose List$ does not name it binds empty.
		if _, ballots, ballotExisted, ok := effects.VoteFinishedResult(ev); ok {
			same, diff := effects.VoteSplit(e.controllerOf(source), ballots, ballotExisted)
			if listAdmits(t.Params["List"], "OppVotedSame") {
				c.TriggeredOpponentsVotedSame = same
			}
			if listAdmits(t.Params["List"], "OppVotedDiff") {
				c.TriggeredOpponentsVotedDiff = diff
			}
		}
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

// listAdmits reports whether a trigger's List$ scope admits one referent
// token: an absent (or empty) List$ names every set, a present one must name
// the token among its comma entries. Shared by the Vote capture only today.
func listAdmits(list, token string) bool {
	list = strings.TrimSpace(list)
	if list == "" {
		return true
	}
	for _, part := range strings.Split(list, ",") {
		if strings.TrimSpace(part) == token {
			return true
		}
	}
	return false
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
