// trigger_match.go is the read-only half of the split: checkTriggers walks
// every object once per event and triggerMatches (with its per-kind helpers
// below -- zoneGate, zoneChangeMatches, spellCastMatches, attacksMatches,
// damageMatches, becomesTargetMatches, landPlayedMatches, phaseMatches)
// decides whether a given T: line fires for it. init() registers those
// trigger kinds, and repl:Moved (replacement.go's own kind), as non-API so
// effects.Supported does not mistake them for something a card SVar could
// invoke directly.
//
// Triggered abilities (T: lines) and replacement effects (R: lines). Every
// state mutation still goes through events.Emit -- engine.go's emit wraps it
// with applyReplacements ahead of logging and checkTriggers behind it, so
// this file's job is entirely about *deciding* what fires and *ordering*
// what goes on the stack, never about writing state directly.
package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// pendingTrigger is a matched trigger waiting to be placed on the stack.
//
// Idx is Source's own Face().Triggers index for the matched T: line. SA is
// kept alongside it (the brief's own interface names both Source and SA)
// even though putTriggersOnStack ends up re-deriving the ability from
// Source+Idx rather than using SA directly: Ruling T20-a means the eventual
// stack object is created inside events.Apply, which cannot carry a raw
// *cards.SA pointer through the log -- only data (an ObjID and a small
// index) that lets Apply find the same *cards.SA a live engine already
// holds.
type pendingTrigger struct {
	Source     state.ObjID
	Controller state.PlayerID
	Idx        int
	SA         *cards.SA
	Ctx        effects.Ctx
}

// triggerKey identifies one T: line: the object that carries it, plus that
// object's own Triggers index (a card can have more than one). Used both for
// the cascade bound and for DamageDealtOnce's once-per-turn gate.
type triggerKey struct {
	Source state.ObjID
	Idx    int
}

// maxTriggerFires bounds how many times a single (source, trigger index)
// pair may queue a pending trigger over the life of a match. Ordinary play
// stays far below this -- even a trigger that fires every turn for a hundred
// turns is two orders of magnitude under it. The pathological case this
// guards is "a trigger that fires in response to its own effect": nothing in
// this build can hang or overflow the stack resolving any *one* ability
// (Resolve's own maxChain bounds a sub-ability chain, and every stack
// resolution requires a fresh external Submit round-trip -- resolveTop is
// only ever called from handlePriority's "pass" case), but nothing stops a
// naive auto-pass driver from sustaining pop-one/push-one-again forever
// across many such round-trips, tying up the match's one goroutine
// indefinitely. This cap is what makes that terminate: once a specific
// trigger has fired this many times, it simply stops matching, so the loop
// runs dry instead of running forever.
const maxTriggerFires = 256

// forEachObject walks every object currently in the game exactly once, in a
// fixed, deterministic order: living seats in ascending order from seat 0,
// then zone in Zone's own declared order (library, hand, battlefield,
// graveyard, exile, stack), then position within that zone's slice.
// checkTriggers and applyReplacements both need this same walk for their own
// discovery to be deterministic, so it is factored out here rather than
// duplicated. Game.Zone ignores its player argument for ZStack (the stack is
// shared across controllers, not per-seat), so that zone is visited only
// once, on the first living seat, rather than once per living player.
func (e *Engine) forEachObject(fn func(id state.ObjID)) {
	for si, p := range e.G.AliveFrom(0) {
		for z := state.ZLibrary; z <= state.ZStack; z++ {
			if z == state.ZStack && si != 0 {
				continue
			}
			for _, id := range append([]state.ObjID(nil), e.G.Zone(z, p)...) {
				fn(id)
			}
		}
	}
}

// controllerOf is a nil-safe Object.Controller read: a nonexistent ObjID
// (stale data, a malformed trigger source) degrades to seat 0 rather than
// panicking.
func (e *Engine) controllerOf(id state.ObjID) state.PlayerID {
	if o := e.G.Obj(id); o != nil {
		return o.Controller
	}
	return 0
}

// checkTriggers is called from emit after every event. It walks every
// object once (forEachObject) and, for each cards.Trigger on that object's
// face, asks triggerMatches whether this event satisfies it. A match
// appends to e.pendingTriggers with a context whose Remembered holds the
// triggering object; putTriggersOnStack later drains that queue onto the
// stack in APNAP order.
//
// lki is the object a MoveZone/Draw/PutOnStack event's own Obj was, just
// before emit applied the event (nil for every other event kind, or if that
// object could not be found -- see emit). It describes ev.Obj, not id (the
// object whose T: line is being checked in a given iteration below), so it
// is handed to EVERY trigger this event fires, not only a Card.Self trigger
// on ev.Obj itself: a bystander's "whenever a creature you control dies"
// wants the LKI of the creature that died, exactly the same as that
// creature's own dies trigger would. Fix round 1, Important 2: an earlier
// version of this doc claimed LKI was withheld from every trigger but the
// matching object's own, which was never what the code below does (see
// objLKI's guard) -- lki.ID == ev.Obj is already guaranteed by construction
// in emit (lki, when non-nil, is always built from e.G.Obj(ev.Obj)), so
// there is no per-trigger gate here at all, only a defensive belt-and-
// braces check against a future emit change that might one day pass a
// mismatched lki.
func (e *Engine) checkTriggers(ev events.Event, lki *state.Object) {
	e.forEachObject(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil {
			return
		}
		f := o.Face()
		if f == nil {
			// Ruling F3: an ability or token object has no Face and
			// therefore no printed Triggers to check -- only real cards
			// carry triggered abilities.
			return
		}
		for ti, t := range f.Triggers {
			if !e.triggerMatches(t, id, ev) {
				continue
			}
			key := triggerKey{Source: id, Idx: ti}
			if e.triggerFireCount == nil {
				e.triggerFireCount = map[triggerKey]int32{}
			}
			if e.triggerFireCount[key] >= maxTriggerFires {
				continue // cascade bound: see maxTriggerFires.
			}
			if t.Mode == "DamageDealtOnce" {
				if e.damageOnceFired == nil {
					e.damageOnceFired = map[triggerKey]int32{}
				}
				if e.damageOnceFired[key] == e.G.Turn {
					continue // already fired this turn.
				}
				e.damageOnceFired[key] = e.G.Turn
			}
			e.triggerFireCount[key]++
			if t.Effect == nil {
				// Execute$ named an SVar this face never defined (or one
				// that failed to parse): the trigger matched, but there is
				// nothing to run.
				continue
			}
			// Defensive, not a per-trigger gate: lki.ID == ev.Obj always
			// holds when lki != nil (see checkTriggers's own doc above), so
			// every trigger this loop matches for this event gets the same
			// objLKI.
			var objLKI *state.Object
			if lki != nil && lki.ID == ev.Obj {
				objLKI = lki
			}
			e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
				Source:     id,
				Controller: o.Controller,
				Idx:        ti,
				SA:         t.Effect,
				Ctx: effects.Ctx{
					Source:     id,
					Controller: o.Controller,
					Remembered: triggerRemembered(ev, id),
					LKI:        objLKI,
				},
			})
		}
	})
}

// triggerRemembered is what a matched trigger's Ctx.Remembered holds: the
// object the triggering event was actually about (the card that changed
// zones, was cast, or is being targeted), or -- for an event with no object
// of its own, such as a step change, or a player-only Damage event -- the
// trigger's own source. Most of the eight M1 modes' Execute$ abilities in
// the acceptance deck don't read Remembered at all (they use Defined$
// Self/You), so this is deliberately one simple, general rule rather than a
// mode-specific one -- except DeclareAttackers, which carries every attacker
// declared this combat in Event.IDs rather than a single Event.Obj (see
// attacksMatches): Remembered there is every declared attacker, in order,
// followed by one more entry for the defending player. ev.Player is set by
// handleAttackers (rules/combat.go) to chosen[0].Player -- the FIRST
// declared attacker's own defender, taken as the defender for the whole
// declaration; a pre-existing simplification of this event's shape (M1 has
// no per-attacker defender), not something this trailing entry introduces.
//
// effects.context.go's Defined already recognises Defined$
// TriggeredDefendingPlayer/TriggeredPlayer (Task 5's playersOf(Remembered),
// merged after this task started). The trailing entry only matters once it
// survives onto the stack, which needed FL-41's fix: pushTrigger
// (trigger_queue.go) could not just write tgt.Obj for this entry into the
// logged TriggerPush event's IDs (always 0 for a player target) -- Apply
// would rebuild it as {Obj: 0}, and playersOf (effects/context.go) filters
// that entry out, so Defined$ TriggeredDefendingPlayer resolves to nothing
// and the effect silently no-ops (PlayerOf is never reached). state.PlayerRef
// is what lets a player reference survive that log-and-replay round trip
// intact.
//
// The one acceptance-game behaviour this trailing entry actually changes is
// Knight of Infamy's intrinsic Exalted keyword trigger (cards/keywords.go
// expands it to Mode$ Attacks | ValidCard$ Creature.YouCtrl | Alone$ True),
// which fires twice in the 4-seat game (mono-black-aggro is dealt in at
// seat 0 there). Exalted's DB$ Pump | Defined$ TriggeredAttacker resolves
// through effects/context.go's objectsOf(c.Remembered): Remembered used to
// be [{Obj: source}] (the exalted permanent itself) and is now every
// declared attacker, so -- because attacksMatches ignores Alone$ (a known
// approximation, tracked separately as FL-48) -- Exalted now pumps every
// attacker when several attack instead of only the lone one. That is the
// measured cause of the 4-seat chain-head move 203ea3a9b2268292 ->
// 81a8a100641b5442. Goblin Guide and Goblin Piledriver never fire in the
// 8-seat game, and Ulamog is in the tron deck, which is never dealt at
// 2/4/6/8 seats: neither is a cause.
func triggerRemembered(ev events.Event, source state.ObjID) []state.Target {
	if ev.Kind == events.DeclareAttackers {
		out := make([]state.Target, 0, len(ev.IDs)+1)
		for _, id := range ev.IDs {
			out = append(out, state.Target{Obj: id})
		}
		return append(out, state.Target{Player: ev.Player, IsPlayer: true})
	}
	if ev.Obj != 0 {
		return []state.Target{{Obj: ev.Obj}}
	}
	return []state.Target{{Obj: source}}
}

// triggerMatches decides whether one cards.Trigger fires for ev. zoneGate
// (TriggerZones$) applies uniformly first; the rest is mode-specific.
func (e *Engine) triggerMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if !e.zoneGate(t, source, ev) {
		return false
	}
	switch t.Mode {
	case "ChangesZone":
		return e.zoneChangeMatches(t, source, ev)
	case "SpellCast":
		return e.spellCastMatches(t, source, ev)
	case "Attacks":
		return e.attacksMatches(t, source, ev)
	case "DamageDone", "DamageDealtOnce":
		return e.damageMatches(t, source, ev)
	case "BecomesTarget":
		return e.becomesTargetMatches(t, source, ev)
	case "LandPlayed":
		return e.landPlayedMatches(t, source, ev)
	case "Phase":
		return e.phaseMatches(t, source, ev)
	}
	// An unimplemented Mode$ never fires -- a malformed or unsupported
	// trigger doing nothing is the safe failure mode (Ruling: see
	// filter.go's own "unknown predicate never matches" precedent).
	return false
}

// zoneGate implements TriggerZones$: a trigger only fires while its source is
// in one of the listed zones. The default is the battlefield, which is why an
// enchantment's upkeep trigger stops when it is destroyed.
//
// checkTriggers calls this after the event has already been folded into
// state (emit logs before it checks triggers), so o.Zone alone only ever
// reflects the zone the object is in *now*. That is correct for an
// entering-the-zone trigger (Snapcaster's ETB: o.Zone is already
// Battlefield by the time this runs) but wrong for a leaving-the-zone one --
// a plain "dies" trigger (Origin$ Battlefield, Destination$ Graveyard,
// ValidCard$ Card.Self, default TriggerZones$ Battlefield) would never see
// its own source "in" the battlefield, because by the time checkTriggers
// runs the move has already happened and o.Zone reads Graveyard. CR 603.10's
// full "look back in time" is not modeled, but the one case Task 20's own
// ChangesZone mode needs it for is narrow and self-contained: when the event
// under test is itself the zone change of this trigger's own source (source
// == ev.Obj), the zone it was in immediately before (ev.From) counts as well
// as the zone it is in now, so both an ETB and a dies trigger with the
// ordinary default work from the same rule.
func (e *Engine) zoneGate(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	o := e.G.Obj(source)
	if o == nil {
		return false
	}
	spec := t.Params["TriggerZones"]
	if spec == "" {
		spec = "Battlefield"
	}
	zones := [2]state.Zone{o.Zone, o.Zone}
	n := 1
	if source == ev.Obj && ev.Obj != 0 &&
		(ev.Kind == events.MoveZone || ev.Kind == events.Draw || ev.Kind == events.PutOnStack) {
		zones[1] = ev.From
		n = 2
	}
	for _, z := range strings.Split(spec, ",") {
		want := effects.ParseZone(strings.TrimSpace(z))
		for _, zone := range zones[:n] {
			if want == zone {
				return true
			}
		}
	}
	return false
}

// zoneChangeMatches implements Mode$ ChangesZone.
func (e *Engine) zoneChangeMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.MoveZone && ev.Kind != events.Draw && ev.Kind != events.PutOnStack {
		return false
	}
	if o, ok := t.Params["Origin"]; ok && o != "Any" && effects.ParseZone(o) != ev.From {
		return false
	}
	if d, ok := t.Params["Destination"]; ok && d != "Any" && effects.ParseZone(d) != ev.To {
		return false
	}
	if v, ok := t.Params["ValidCard"]; ok {
		return effects.MatchesSpecFrom(e.G, v, ev.Obj, e.controllerOf(source), source)
	}
	return true
}

// spellCastMatches implements Mode$ SpellCast: ValidCard$ and
// ValidActivatingPlayer$ against a PutOnStack event.
func (e *Engine) spellCastMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.PutOnStack {
		return false
	}
	// Casting a spell means an actual card entering the stack. This build
	// also uses PutOnStack-shaped Move()s for nothing else today (triggered
	// abilities go on the stack via a dedicated TriggerPush event --
	// putTriggersOnStack, above, and events.Apply's TriggerPush case), but a
	// Face()-less object could otherwise satisfy a bare "Any"/"Spell"
	// ValidCard$ regardless (matchesBase's Spell/Any cases don't consult
	// Face()), so this guard holds regardless of how a future ability-object
	// path might reach here. Ruling F3.
	obj := e.G.Obj(ev.Obj)
	if obj == nil || obj.Face() == nil {
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidCard"]; ok {
		if !effects.MatchesSpecFrom(e.G, v, ev.Obj, ctrl, source) {
			return false
		}
	}
	if v, ok := t.Params["ValidActivatingPlayer"]; ok {
		if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	return true
}

// attacksMatches implements Mode$ Attacks against a DeclareAttackers event.
// DeclareAttackers carries every attacker declared this combat in one event
// (IDs), so -- like every other mode here -- this fires at most once per
// event rather than once per qualifying attacker: a documented M1
// simplification, not a missed multi-attacker case.
func (e *Engine) attacksMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.DeclareAttackers {
		return false
	}
	spec, ok := t.Params["ValidCard"]
	if !ok {
		for _, id := range ev.IDs {
			if id == source {
				return true
			}
		}
		return false
	}
	ctrl := e.controllerOf(source)
	for _, id := range ev.IDs {
		if effects.MatchesSpecFrom(e.G, spec, id, ctrl, source) {
			return true
		}
	}
	return false
}

// damageSource identifies who dealt a just-emitted Damage event, for
// ValidSource$ matching. events.Event carries no explicit source field for
// Damage -- every Damage event this build emits (effects/damage.go's
// DealDamage/DamageAll) comes from a primitive running inside Resolve,
// called only from resolveTop while the resolving spell or ability is still
// the top of the stack (resolveTop pops it only after Resolve returns), so
// the current stack top is that source for every code path this build has
// today. A future combat-damage implementation, or any Damage emission
// outside ability resolution, would need Event to carry an explicit source
// instead of relying on this.
func (e *Engine) damageSource() state.ObjID {
	if len(e.G.Stack) == 0 {
		return 0
	}
	return e.G.Stack[len(e.G.Stack)-1]
}

// damageMatches implements Mode$ DamageDone and DamageDealtOnce (the once-
// per-turn gate itself lives in checkTriggers, alongside the cascade bound;
// this is purely the per-event parameter match, shared by both modes).
func (e *Engine) damageMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.Damage {
		return false
	}
	if strings.EqualFold(t.Params["CombatDamage"], "True") {
		// dealCombatDamage (rules/combat.go) is fully implemented, but
		// events.Event still carries nothing to distinguish combat from
		// noncombat damage at the point a trigger checks it, so a trigger
		// that insists on CombatDamage$ True can never fire yet regardless.
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidSource"]; ok {
		src := e.damageSource()
		if src == 0 || !effects.MatchesSpecFrom(e.G, v, src, ctrl, source) {
			return false
		}
	}
	if v, ok := t.Params["ValidTarget"]; ok {
		if ev.Obj != 0 {
			if !effects.MatchesSpecFrom(e.G, v, ev.Obj, ctrl, source) {
				return false
			}
		} else if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	return true
}

// becomesTargetMatches implements Mode$ BecomesTarget: the trigger's own
// source must be among the chosen targets recorded by a TargetsChosen event
// (rules.handleTarget -- "the target decision being answered").
func (e *Engine) becomesTargetMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.TargetsChosen {
		return false
	}
	targeted := false
	for _, id := range ev.IDs {
		if id == source {
			targeted = true
			break
		}
	}
	if !targeted {
		return false
	}
	if v, ok := t.Params["ValidTarget"]; ok {
		return effects.MatchesSpecFrom(e.G, v, source, e.controllerOf(source), source)
	}
	return true
}

// landPlayedMatches implements Mode$ LandPlayed. This fires on the MoveZone
// hand->battlefield of a land specifically -- not on the separate LandPlayed
// event legal.go's "play_land" case also emits, which carries only a Player
// (no Obj), and so has nothing ValidCard$ could ever match against.
func (e *Engine) landPlayedMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.MoveZone || ev.From != state.ZHand || ev.To != state.ZBattlefield {
		return false
	}
	obj := e.G.Obj(ev.Obj)
	if obj == nil || obj.Face() == nil || !obj.Face().IsLand() {
		return false
	}
	if v, ok := t.Params["ValidCard"]; ok {
		return effects.MatchesSpecFrom(e.G, v, ev.Obj, e.controllerOf(source), source)
	}
	return true
}

// phaseMatches implements Mode$ Phase.
func (e *Engine) phaseMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.StepChange {
		return false
	}
	want := strings.ToLower(t.Params["Phase"])
	if want != "" && !strings.Contains(ev.Step.String(), want) {
		return false
	}
	if v, ok := t.Params["ValidPlayer"]; ok {
		// StepChange carries no Player of its own -- a step always belongs
		// to the current active player.
		if !effects.MatchesPlayerSpec(e.G, v, e.G.Active, e.controllerOf(source)) {
			return false
		}
	}
	return true
}

func init() {
	effects.RegisterNonAPI(
		"trig:ChangesZone", "trig:SpellCast", "trig:Attacks", "trig:DamageDone",
		"trig:DamageDealtOnce", "trig:BecomesTarget", "trig:LandPlayed", "trig:Phase",
		"repl:Moved",
	)
}
