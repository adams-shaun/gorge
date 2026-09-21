// The CR 603.4 intervening-if gate, shared by every trigger mode.
//
// triggerMatches applies this to every mode after its matcher, so the same T:
// line grammar -- a LifeAmount$ or IsPresent$/PresentCompare$ clause -- is
// honoured wherever it appears.
//
// Split out of trigger_match.go so tickets touching different modes stop
// colliding on one file. Registration is at the bottom; a duplicate mode
// panics (registerTrigMatcher).

package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// triggerConditionHolds evaluates the CR 603.4 intervening-if clause (and the
// CR 603.8 state-trigger condition) carried on a T: line. Two clause shapes
// are recognised, the two the corpus uses on the trigger lines this engine
// routes through the modes above:
//
//   - LifeAmount$ <op><n> against LifeTotal$ (<who>): the named player's life
//     total compared to n.
//   - IsPresent$ <spec> with PresentCompare$ <op><n>: the count of objects
//     matching <spec> compared to n.
//   - Metalcraft$ True (and the bare-Condition$ Metalcraft spelling): the
//     controller controls three or more artifacts -- the trigger-side named
//     condition the ability word uses (6 corpus files: Vedalken Humiliator's
//     attack pump, Blade-Tribe Berserkers' and Bleak Coven Vampires' ETBs,
//     Lumengrid Drake's bounce, Inventors' Fair's upkeep lifegain, Screeching
//     Silcaw's mill), read through the SAME metalcraftHolds census
//     costConditionHolds' Condition$ Metalcraft case and the Continuous
//     static gate read, so the three cannot drift apart.
//
// An absent clause is vacuously true. A clause whose shape this build cannot
// evaluate FAILS CLOSED -- a false condition means the trigger simply does not
// fire, never that an unreadable life/creature count is presumed large
// enough to let a win or counter trigger slip through.
func (e *Engine) triggerConditionHolds(t cards.Trigger, source state.ObjID) bool {
	return e.triggerConditionHoldsAs(t, source, e.controllerOf(source))
}

// triggerConditionHoldsAs is triggerConditionHolds with "you" supplied
// explicitly rather than derived from source's controller. An event-matched
// delayed trigger's "you" is the registration's effect owner (dt.Controller),
// which can differ from the source card's own controller -- see
// checkEventDelayedTriggers.
func (e *Engine) triggerConditionHoldsAs(t cards.Trigger, source state.ObjID, you state.PlayerID) bool {
	// LifeLost's and LifeGained's LifeAmount$ are matched against the causing
	// loss/gain by their matchers (lifeLostMatches/lifeGainedMatches), rather
	// than against a player's current life total.
	if v, ok := t.Params["LifeAmount"]; ok && t.Mode != "LifeLost" && t.Mode != "LifeLostAll" && t.Mode != "LifeGained" {
		if !e.lifeConditionHoldsAs(t, you, v) {
			return false
		}
	}
	if spec, ok := t.Params["IsPresent"]; ok {
		cmp, hasCmp := t.Params["PresentCompare"]
		// PresentDefined$ names the base set the IsPresent$ spec is counted
		// over (Mana Vault's "if this artifact is tapped": PresentDefined$
		// Self narrows the scan to the source itself, where the old whole-
		// battlefield walk counted every tapped permanent). An absent
		// PresentDefined keeps the historic whole-battlefield scan. A value
		// that is not the source fails closed with the rest of the clause.
		if pd := strings.TrimSpace(t.Params["PresentDefined"]); pd != "" && pd != "Self" {
			return false
		}
		if !hasCmp {
			// Forge's own reading of an IsPresent$ clause with no
			// PresentCompare$ is "at least one match" (Mana Vault's draw-step
			// damage): a present-condition with no comparison never meant
			// "vacuously true", which is what the old hard return made it.
			spec2 := strings.TrimSpace(t.Params["IsPresent2"])
			// PresentZone$ scopes the count to one named zone (Jocasta's
			// "if this card is in your graveyard"); the no-compare branch
			// honours it exactly like the compared branch below. A zone word
			// this build does not know, or a combination with the IsPresent2$
			// union whose zone each member would scan, fails closed.
			if pz := strings.TrimSpace(t.Params["PresentZone"]); pz != "" {
				if spec2 != "" {
					return false
				}
				return e.countPresentZone(t, spec, source, you)
			}
			if spec2 != "" {
				return e.presentUnionCount(spec, spec2, source, you) > 0
			}
			return e.countPresent(spec, source, you) > 0
		}
		if !e.presentConditionHoldsAs(t, source, you, spec, cmp) {
			return false
		}
		// IsPresent2$ names a SECOND present set whose objects count alongside
		// IsPresent$'s, as one union ("Name Sticker" Goblin counts creatures
		// named Name Sticker Goblin plus the entering one; the source itself
		// sits in both sets, so a plain sum would count it twice and break the
		// boundary the comparison guards). Deduplicating by object identity is
		// the only reading that reproduces the card's "9 or fewer creatures
		// named ..." at every count.
		if spec2 := strings.TrimSpace(t.Params["IsPresent2"]); spec2 != "" {
			op, n, ok := splitCompare(strings.TrimSpace(cmp))
			if !ok {
				return false
			}
			return applyCompare(e.presentUnionCount(spec, spec2, source, you), op, n)
		}
	}
	if v, ok := t.Params["Metalcraft"]; ok {
		// The trigger-side named condition (task trig-attacks-metalcraft):
		// a value this build cannot read as True is an unreadable clause
		// shape and fails closed like the other clauses above.
		if !strings.EqualFold(strings.TrimSpace(v), "True") || !e.metalcraftHolds(you) {
			return false
		}
	}
	if strings.EqualFold(strings.TrimSpace(t.Params["Condition"]), "Metalcraft") {
		// The bare-Condition$ spelling of the same gate. The trigger path
		// reads no OTHER bare Condition$ value (LifePaid, Evolve,
		// Sacrificed and friends are matched by their own per-kind helpers
		// or stay unread), and no corpus trigger carries this spelling
		// today -- the bare Condition$ Metalcraft carriers are S: statics
		// the Continuous gate already reads -- but the spelling is kept
		// beside Metalcraft$ so the two cannot drift apart.
		if !e.metalcraftHolds(you) {
			return false
		}
	}
	if v, ok := t.Params["Revolt"]; ok {
		// Revolt$ (the CR 702.38 ability word, "if a permanent you
		// controlled left the battlefield this turn"): the SAME
		// revoltThisTurn scan the replacement path's Revolt$ clause reads
		// (rules/replacement.go), the one Engine.RevoltHolds -- effects'
		// bare Condition$ Revolt gate and Count$Revolt branch head --
		// delegates to, so the three spellings cannot drift apart. Measured
		// over the corpus: 29 files carry Revolt$ True; 24 of its lines are
		// triggers -- 16 Mode$ Phase end-step shapes (Aid from the Cowl,
		// Hidden Stockpile, Krang) and 8 Mode$ ChangesZone ETBs (Airdrop
		// Aeronauts, Vengeful Rebel, Deadeye Harpooner); the rest are a
		// replacement line (Aether Revolt, already read on the replacement
		// path) and non-trigger text. A value this build cannot read as
		// True is an unreadable clause shape and fails closed like the
		// Metalcraft$ clause above.
		if !strings.EqualFold(strings.TrimSpace(v), "True") || !e.revoltThisTurn(you) {
			return false
		}
	}
	if strings.EqualFold(strings.TrimSpace(t.Params["Condition"]), "Revolt") {
		// The bare-Condition$ spelling of the same gate. No corpus trigger
		// carries it today (the one bare Condition$ Revolt carrier,
		// Decommission, is a DB$ GainLife sub the effects condition gate
		// reads), kept beside the bare Condition$ Metalcraft spelling so
		// the two cannot drift apart.
		if !e.revoltThisTurn(you) {
			return false
		}
	}
	if name, ok := t.Params["CheckSVar"]; ok {
		// CheckSVar$/SVarCompare$ (Kozilek, the Great Distortion's cast
		// trigger: "if you have fewer than seven cards in hand"): the
		// CR 603.4 intervening-if the shared SVar-compare evaluator reads,
		// evaluated with the source face's SVar table and the trigger's
		// "you" -- the same wrapper rules' static gate uses. A condition
		// this build cannot evaluate fails closed (the trigger does not
		// fire), the same convention triggerConditionHolds' other clauses
		// document above.
		src := e.G.Obj(source)
		if src == nil || src.Face() == nil {
			return false
		}
		// A mutated pile's under-card trigger (CR 702.140d) is gated by the
		// UNDER-CARD's own SVar table: Face() on a pile is always its top
		// card, and the top card of a mutate pile can be any creature, so
		// reading its table would evaluate the gate against a body it never
		// defined (failing closed, i.e. silently never firing). An ordinary
		// trigger's owning face IS the top face, so nothing else moves.
		svars := src.Face().SVars
		if mf := e.faceOwningTrigger(source, t); mf != nil {
			svars = mf.SVars
		}
		ctx := &effects.Ctx{Source: source, Controller: you, SVars: svars}
		holds, evaluated := effects.CheckSVarHolds(e, ctx, name, strings.TrimSpace(t.Params["SVarCompare"]))
		if !evaluated || !holds {
			return false
		}
	}
	if spec, ok := t.Params["CheckDefinedPlayer"]; ok {
		holds, supported := e.checkDefinedPlayerHolds(spec, you)
		// A supported predicate is evaluated for every mode. An unsupported
		// one fails closed only for actionTriggerModes; other modes keep
		// firing as they did before the predicate was read at all.
		if (supported && !holds) || (!supported && actionTriggerModes[t.Mode]) {
			return false
		}
	}
	return true
}

// checkDefinedPlayerHolds evaluates the player-state predicate class used by
// event-trigger conditions. isMonarch, with the controller, opponent and any-
// player selectors, is the supported form; supported is false for any other
// predicate (hasInitiative, withMost*, committedCrimeThisTurn, ...).
func (e *Engine) checkDefinedPlayerHolds(spec string, you state.PlayerID) (holds, supported bool) {
	base, property, ok := strings.Cut(strings.TrimSpace(spec), ".")
	if !ok || property != "isMonarch" {
		return false, false
	}
	switch base {
	case "You":
		return e.G.IsMonarch(you), true
	case "Opponent", "Other":
		for _, p := range e.G.AliveFrom(you) {
			if p != you && e.G.IsMonarch(p) {
				return true, true
			}
		}
		return false, true
	case "Player", "Any":
		for _, p := range e.G.AliveFrom(0) {
			if e.G.IsMonarch(p) {
				return true, true
			}
		}
		return false, true
	}
	return false, false
}

// lifeConditionHoldsAs evaluates the LifeTotal$/LifeAmount$ intervening-if.
// The "you" for a You-qualified LifeTotal$ is the caller's chosen player --
// normally the source's controller, but an event-matched delayed trigger
// passes its registration's effect owner instead (see
// checkEventDelayedTriggers).
func (e *Engine) lifeConditionHoldsAs(t cards.Trigger, you state.PlayerID, amount string) bool {
	who := you
	if v, ok := t.Params["LifeTotal"]; ok {
		v = strings.TrimSpace(v)
		switch v {
		case "", "You":
			// the controller, which who already is
		case "ActivePlayer":
			who = e.G.Active
		default:
			return false // unevaluable player selector: fail closed
		}
	}
	if int(who) >= len(e.G.Players) || e.G.Players[who].Lost {
		return false
	}
	return compareLife(e.G.Players[who].Life, amount)
}

// presentConditionHoldsAs evaluates the IsPresent$/PresentCompare$
// intervening-if by counting the objects on the battlefield that match the
// spec (relative to the trigger's source and the caller's chosen "you") and
// comparing that count.
func (e *Engine) presentConditionHoldsAs(t cards.Trigger, source state.ObjID, you state.PlayerID, spec, cmp string) bool {
	// PresentDefined$ (Mana Vault's draw-step self-check): the IsPresent$
	// spec is evaluated over the DEFINED set rather than the whole
	// battlefield. "Self" -- the corpus's dominant value -- counts the
	// source object alone when it matches; any other selector falls back to
	// the battlefield-wide count, so an unreadable defined set degrades to
	// the pre-PresentDefined behaviour instead of fail-closing a trigger
	// whose spec the count would otherwise answer.
	if pd := strings.TrimSpace(t.Params["PresentDefined"]); pd != "" {
		if strings.EqualFold(pd, "Self") {
			if o := e.G.Obj(source); o == nil || o.Zone != state.ZBattlefield ||
				!effects.MatchesSpecCtx(e.G, spec, source, e.specCtx(source, you)) {
				return comparePresent(0, cmp)
			}
			return comparePresent(1, cmp)
		}
	}
	// PresentZone$ (Jocasta, Automaton Avenger's "if this card is in your
	// graveyard"): the IsPresent$ spec is counted over the named zone in
	// every living seat's copy of it, in deterministic seat/zone order, the
	// same walk countPresent makes over the battlefield. An unknown zone
	// word fails closed -- a clause this build cannot read must never read
	// as vacuously satisfied.
	if pz := strings.TrimSpace(t.Params["PresentZone"]); pz != "" {
		n, known := e.presentZoneCount(t, spec, source, you)
		if !known {
			return false
		}
		return comparePresent(n, cmp)
	}
	n := e.countPresent(spec, source, you)
	return comparePresent(n, cmp)
}

// presentZoneCount counts spec matches over one zone (PresentZone$'s value)
// across every living seat, the deterministic walk countPresent makes over
// the battlefield. known is false for a zone word this build does not know,
// which every caller fails closed on.
func (e *Engine) presentZoneCount(t cards.Trigger, spec string, source state.ObjID, you state.PlayerID) (int, bool) {
	zone, known := effects.ParseZoneWord(strings.TrimSpace(t.Params["PresentZone"]))
	if !known {
		return 0, false
	}
	n := 0
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(zone, p) {
			if effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(source, you)) {
				n++
			}
		}
	}
	return n, true
}

// countPresentZone is the no-compare IsPresent$ branch's PresentZone$ count:
// an unknown zone word fails closed to "never holds".
func (e *Engine) countPresentZone(t cards.Trigger, spec string, source state.ObjID, you state.PlayerID) bool {
	n, known := e.presentZoneCount(t, spec, source, you)
	return known && n > 0
}

// countPresent walks every object on the battlefield once and counts those
// matching spec, excluding the source where the spec's own Other/StrictlyOther
// predicate already handles it (Emperor Crocodile's Creature.Other+YouCtrl).
func (e *Engine) countPresent(spec string, source state.ObjID, you state.PlayerID) int {
	n := 0
	e.forEachObject(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			return
		}
		if effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(source, you)) {
			n++
		}
	})
	return n
}

// presentUnionCount counts the DISTINCT battlefield objects matching either
// spec — the IsPresent$+IsPresent2$ union a two-set present clause compares.
func (e *Engine) presentUnionCount(spec, spec2 string, source state.ObjID, you state.PlayerID) int {
	seen := map[state.ObjID]bool{}
	n := 0
	e.forEachObject(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			return
		}
		if seen[id] {
			return
		}
		sc := e.specCtx(source, you)
		if effects.MatchesSpecCtx(e.G, spec, id, sc) || effects.MatchesSpecCtx(e.G, spec2, id, sc) {
			seen[id] = true
			n++
		}
	})
	return n
}

// compareLife compares a life total against a Forge comparison literal such as
// GE40, EQ0, LE3. Any shape this build cannot fold (a non-numeric rhs, a
// missing operator) is false, so an unreadable condition never fires a
// trigger.
func compareLife(have int32, cmp string) bool {
	op, n, ok := splitCompare(strings.TrimSpace(cmp))
	if !ok {
		return false
	}
	return applyCompare(int(have), op, n)
}

// comparePresent compares a present-count against the same comparison literal
// grammar. A non-numeric rhs (PresentCompare$ EQX) fails closed.
func comparePresent(have int, cmp string) bool {
	op, n, ok := splitCompare(strings.TrimSpace(cmp))
	if !ok {
		return false
	}
	return applyCompare(have, op, n)
}

// splitCompare separates a Forge comparison literal ("GE40", "EQ0") into its
// two-character operator and its numeric rhs. ok is false for anything that is
// not a recognised operator followed by an integer.
func splitCompare(cmp string) (op string, n int, ok bool) {
	if len(cmp) < 3 {
		return "", 0, false
	}
	op = cmp[:2]
	num, err := strconv.Atoi(cmp[2:])
	if err != nil {
		return "", 0, false
	}
	return op, num, true
}

func applyCompare(have int, op string, n int) bool {
	switch op {
	case "GE":
		return have >= n
	case "LE":
		return have <= n
	case "EQ":
		return have == n
	case "GT":
		return have > n
	case "LT":
		return have < n
	case "NE":
		return have != n
	}
	return false
}

// stateTriggerOutstanding reports whether a state trigger (Mode$ Always)
// already has an instance where one would be enqueued -- either still in the
// pending queue or already placed on the stack. This is the CR 603.8 latch:
// it is what stops a continuously-true condition from enqueuing an unbounded
// run of the same trigger.
func (e *Engine) stateTriggerOutstanding(source state.ObjID, idx int) bool {
	for _, pt := range e.pendingTriggers {
		if pt.Source == source && pt.Idx == idx {
			return true
		}
	}
	o := e.G.Obj(source)
	if o == nil {
		return false
	}
	f := o.Face()
	if f == nil || idx < 0 || idx >= len(f.Triggers) {
		return false
	}
	sa := f.Triggers[idx].Effect
	for _, sid := range e.G.Stack {
		so := e.G.Obj(sid)
		if so != nil && so.Source == source && so.Ability == sa {
			return true
		}
	}
	return false
}
