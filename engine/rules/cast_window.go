package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// flipProposalFace records the proposal's pre-flip face in *faceBefore and
// emits the FlipFace that puts the alternate face on the stack. It is the
// shape an alternate-face cast proposal uses before its ordinary cast
// transaction; an aborted proposal restores *faceBefore (CR 733.1).
func (e *Engine) flipProposalFace(o *state.Object, faceBefore **uint8, preSuppress map[state.ObjID]bool, preAborts map[state.ObjID]int32) {
	before := o.FaceIdx
	*faceBefore = &before
	e.emitProposalFlip(o.ID, before, preSuppress, preAborts)
}

// emitProposalFlip records the FlipFace an alternate-face cast proposal
// (room_alt, adventure_alt, adventure_recast, aftermath, split_alt,
// omen_alt, defeat_cast) makes before its ordinary cast transaction, WITHOUT letting
// that flip count as game progress. emit clears the held-out no-progress
// state (suppressedCast/castAborts) on every state-changing event; the flip
// is one, but it is net no progress when the proposal is then reversed
// (abortCast flips the card back). Left cleared, pushCast captured the
// already-emptied maps as the proposal's pre-push state, so abortCast's
// F05-2 count restarted at zero on every attempt and the SECOND identical
// no-progress abort never held the option out: an Adventure cast the offer
// priced as payable but the flipped face could not pay was reversed and
// re-offered forever (the botbench flip_face livelock). Restoring the maps
// the proposal began with keeps the count across attempts; a cast that goes
// on to reach the stack clears them at its PutOnStack as before.
func (e *Engine) emitProposalFlip(id state.ObjID, before uint8, preSuppress map[state.ObjID]bool, preAborts map[state.ObjID]int32) {
	e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: int32(1 - int(before))})
	e.suppressedCast, e.castAborts = preSuppress, preAborts
}

// dropProposalTriggers removes the pending triggers a reversed spell
// proposal's own CR 601.2c target choice queued (pc.proposalTriggers). CR
// 733.1: "No abilities trigger and no effects apply as a result of an undone
// action." Only the recorded ranges go -- a trigger an unreversed mana
// ability produced during the payment window (Manabarbs) stays queued, since
// the engine never reverses those activations. Ranges are removed back to
// front so an earlier range's indices are unaffected; a range the queue no
// longer covers (it was drained, which a live proposal never does) is skipped
// rather than trusted, and an ordered prefix is never touched.
func (e *Engine) dropProposalTriggers(pc *pendingCast) {
	ranges := pc.proposalTriggers
	pc.proposalTriggers = nil
	for i := len(ranges) - 1; i >= 0; i-- {
		start, end := ranges[i][0], ranges[i][1]
		if start < e.orderedTriggers || start >= end || end > len(e.pendingTriggers) {
			continue
		}
		e.noteTrigShrink()
		e.pendingTriggers = append(e.pendingTriggers[:start], e.pendingTriggers[end:]...)
	}
}

// castWindowUnits is the CR 601.2g cast-payment window's provable mana reach:
// the shared fixed-production census (windowManaUnits) plus each
// choice-shaped source (Any, Combo, Chosen) as single-colour alternatives of
// the SAME permanent, never as another tap, plus the cast-only paid/dynamic
// layer (castWindowPaidUnits) for the deterministic activation shapes the
// shared census cannot price. The shared census omits those because an
// unless-pay window cannot pose their colour or payment sub-ask; cast
// payment can. The result is filtered to the sources manaWindowAsk will
// actually offer (a permanent this cast already committed to Convoke or
// Conspire is withheld).
func (e *Engine) castWindowUnits(pc *pendingCast) []windowManaUnit {
	p := pc.player
	windowUnits := e.windowManaUnits(p)
	windowUnits = e.castWindowProbeUnits(pc, windowUnits)
	windowUnits = e.castWindowReflectedUnits(pc, windowUnits)
	// SUPERSET GUARD (the anti-abort invariant): a source this cast already
	// committed to Convoke/Harmonize/Improvise (pc.convoke) or Conspire
	// (pc.taps) is NOT offered by manaWindowAsk (cast.go's convokeCommitted
	// filter), so the probe must not promise its tap either -- the cost fold
	// already credits its contribution, and counting the permanent a second
	// time would let the probe claim reach the window cannot complete.
	out := windowUnits[:0]
	for _, u := range windowUnits {
		if e.convokeCommitted(pc, u.ID) {
			continue
		}
		out = append(out, u)
	}
	return out
}

// castWindowReflectedUnits is castWindowUnits' reflected-mana pass: for each
// unit the census and probe already admitted, the alts of a
// pay.PaymentPlanReflectedShape ability (Star Compass, Exotic Orchard) are
// replaced by one concrete alt per colour paymentPlanReflectedAlt names --
// the same colours the payment planner expands, so the target and face gates
// agree with the plan by construction. The shared census lists such an
// ability as its colourless {C} stand-in (and the probe splits that blank
// again), which a reflected tap never produces. When the reflection names no
// colour right now the unit stays as it was (fail closed). Each rebuilt unit
// gets a fresh Alts slice: census spans are capped and may share backing.
func (e *Engine) castWindowReflectedUnits(pc *pendingCast, units []windowManaUnit) []windowManaUnit {
	for ui := range units {
		u := &units[ui]
		expanded := map[*cards.SA][]windowManaAlt{}
		rebuilt := false
		for _, a := range u.Alts {
			if _, seen := expanded[a.Ma]; seen || a.Ma == nil || a.Amt <= 0 || !pay.PaymentPlanReflectedShape(a.Ma) {
				continue
			}
			alt, ok := paymentPlanReflectedAlt(e, pc.player, u.ID, a.Ma, a.Amt)
			if !ok {
				expanded[a.Ma] = nil
				continue
			}
			var concrete []windowManaAlt
			for i, n := range alt.Counts {
				if n <= 0 {
					continue
				}
				c := windowManaAlt{Ma: a.Ma, Amt: a.Amt, Life: a.Life, LifeKeep: a.LifeKeep, CostGeneric: a.CostGeneric}
				c.Counts[i] = 1
				concrete = append(concrete, c)
			}
			expanded[a.Ma] = concrete
			rebuilt = rebuilt || concrete != nil
		}
		if !rebuilt {
			continue
		}
		alts := make([]windowManaAlt, 0, len(u.Alts))
		placed := map[*cards.SA]bool{}
		for _, a := range u.Alts {
			concrete, ok := expanded[a.Ma]
			if !ok || concrete == nil {
				alts = append(alts, a)
				continue
			}
			if !placed[a.Ma] {
				placed[a.Ma] = true
				alts = append(alts, concrete...)
			}
		}
		u.Alts = alts
	}
	return units
}

// castWindowProbeUnits is castWindowUnits' cast-only activation-cost layer.
// It adds the shapes the shared windowManaUnits withholds from EVERY payment
// window: choice-shaped productions (a colour chosen when the source is
// tapped), free abilities whose Amount$ is not a literal, and non-free
// activation costs (a literal generic <N>, a PayLife<N>, a deterministic
// self-sacrifice, or a choice-shaped production behind any of them). The
// conservatism of windowManaUnits is load-bearing for the attack-cost and
// unless-cost windows, which cannot pose a sub-ask while tapping; the CR
// 601.2g cast window CAN: manaWindowAsk offers any currently payable
// non-InstantSpeed mana source, including a tapped source with a {Q} ability,
// and activation runs through resolveManaAbilityRef, which pays the full
// activation cost and evaluates the Amount$ body. Every source added here
// comes from the same availableManaAbilitiesForWindow walk manaWindowAsk uses,
// so the probe can never promise an activation the window will not offer.
//
// A literal generic <N> activation cost is carried on the alt as costGeneric
// rather than netted into the production: the cast-only eligibility search
// (castWindowReachable) pays it from the mana this window has already
// accumulated, so the fee can be funded by an earlier same-window activation
// and a multi-colour production stays expressible without choosing which
// colour the generic consumed. A PayLife<N> activation carries its life on
// the alt so the search debits it. The CHOICE-SHAPED production is split
// into one alt per producible colour, exactly the shared walk's
// one-alt-per-alternative shape, so the payer's colour choice is made by
// picking an alt and never needs a nested sub-ask.
//
// Deliberately EXCLUDED (fail closed), each for a named reason:
//
//   - InstantSpeed$ True abilities: already withheld by the shared walk.
//   - RestrictValid$-governed abilities: the produced batch may not pay the
//     priced cost, and the dotted matcher only admits a subset of the
//     grammar, so no restriction is priced here (AGENTS.md row 1's
//     direction).
//   - tapXType, SubCounter, Mill, UnlessCost$, Return<>, coloured activation
//     pips, multi-part or overlapping Sac costs, loyalty-ability mana
//     producers (never exposed by availableManaAbilities), and every
//     Amount$ body the count evaluator does not understand.
func (e *Engine) castWindowProbeUnits(pc *pendingCast, windowUnits []windowManaUnit) []windowManaUnit {
	p := pc.player
	pl := e.G.Players[p]
	for _, id := range pay.BattlefieldManaSourceIDs(e.G, p) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		for _, ma := range e.castWindowProbeAbilities(p, id) {
			mp := effects.ManaOf(ma)
			if mp.RestrictValid != "" {
				continue
			}
			cost := e.abilityCompiledCost(ma).Cost
			if pay.ActivationTapCostUnavailable(o, &cost) {
				continue
			}
			lifeCost := int32(0)
			genericCost := int32(0)
			free := pay.ManaFreeCost(cost)
			switch {
			case free:
				// windowManaUnits already counted a free PLAIN literal
				// production; only a choice-shaped or dynamic amount it
				// withholds is added here (the skip test runs below, after
				// the production is parsed).
			case castWindowGenericCostShape(cost):
				genericCost = cost.Generic
			default:
				// The PayLife<N> and self-sacrifice arms are the paid-cost
				// census layer's shared shape test (paid_mana_census.go).
				lc, ok := e.paidCostShape(p, id, cost, pl.Life)
				if !ok {
					continue
				}
				lifeCost = lc
			}
			amt, ok := e.castWindowAmount(p, id, o, ma)
			if !ok {
				continue
			}
			// Chosen is an as-enters read: substitute the recorded colour
			// BEFORE the parse so a "Combo R Chosen" land recorded as G
			// offers R/G only, never the raw parser's WUBRG superset. Without
			// a record it produces no colour and stays out of this window
			// (the same fail-closed direction the shared walk takes).
			counts, any := mp.Counts, mp.CountsAny
			if mp.NeedsChosen {
				chosen := pay.ChosenProducedColour(e.G, id)
				if chosen == "" {
					continue
				}
				counts, any = cards.ProducedCounts(pay.SubstituteChosenProduced(mp.Produced, chosen))
			}
			total := int32(0)
			for _, n := range counts {
				total += n
			}
			if total <= 0 {
				continue
			}
			if free {
				// A free plain literal production is windowManaUnits' domain
				// (already counted); a free choice-shaped one is not, and a
				// free dynamic amount is the shape this layer adds. The shape
				// is the printed one: that census withholds a recorded Chosen
				// although the substitution above reads as a plain colour.
				if !mp.CountsAny && availableAmountOf(mp) > 0 {
					continue
				}
			}
			if any {
				for colour, n := range counts {
					if n == 0 {
						continue
					}
					var single [6]int32
					single[colour] = 1
					windowUnits = appendCastWindowAlt(windowUnits, id, ma, single, amt, lifeCost, genericCost)
				}
				continue
			}
			windowUnits = appendCastWindowAlt(windowUnits, id, ma, counts, amt, lifeCost, genericCost)
		}
	}
	return windowUnits
}

// castWindowProbeAbilities is the cast-window probe's own member walk: the
// same gates as availableManaAbilitiesForWindow, EXCEPT the live-pool
// payability gate, which a CR 601.2g window can satisfy later in the same
// window (appendAvailableManaAbilitiesGate's ignorePayable). InstantSpeed$
// True abilities stay withheld exactly as the shared window walk withholds
// them, so no ability the live payment window cannot activate is ever priced.
// Nor is one whose Condition$ this build doesn't evaluate: it resolves to no mana
// (CR 106.5), so it can't close a gap and says nothing about the census.
func (e *Engine) castWindowProbeAbilities(p state.PlayerID, id state.ObjID) []*cards.SA {
	var out []*cards.SA
	for _, ma := range e.appendAvailableManaAbilitiesGate(nil, nil, p, id, true) {
		if pay.InstantSpeedOnly(ma) || manaAbilityAddsNothing(ma) {
			continue
		}
		out = append(out, ma)
	}
	return out
}

// manaAbilityAddsNothing reports whether a mana ability's Condition$ is one the
// effect walk skips, so its activation taps the source and adds no mana.
func manaAbilityAddsNothing(ma *cards.SA) bool {
	_, unmodelled := effects.UnmodelledCondition(ma)
	return unmodelled
}

// castWindowSourceExcluded reports whether source id's probe-admitted mana
// abilities stay unpriced for a reason that can never become a unit inside
// this window: a fail-closed grammar exclusion (RestrictValid$, an
// unresolvable Amount$, an unpriced paid shape, a Chosen colour with no
// record) or a free plain literal the shared census withheld (its domain).
// A source whose every admitted ability is merely TAP-BLOCKED (a tapped
// permanent with a tap-required activation) would be priced the moment it
// untaps, so it leaves the census complete rather than failing it open:
// "priced everything the window can offer" and "priced everything, because
// the one unpriced source simply has nothing to offer right now" are the
// same fact. So is a RestrictValid$ ability this payment can't spend (CR 106.6).
// The gates mirror castWindowProbeUnits' skips so the verdict describes
// that loop's own reason for leaving no unit.
func (e *Engine) castWindowSourceExcluded(p state.PlayerID, id state.ObjID, desc pay.Descriptor) bool {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false
	}
	for _, ma := range e.castWindowProbeAbilities(p, id) {
		mp := effects.ManaOf(ma)
		cost := e.abilityCompiledCost(ma).Cost
		if pay.ActivationTapCostUnavailable(o, &cost) {
			continue
		}
		if mp.RestrictValid != "" {
			if pay.RestrictValidMatches(asPayer(e), p, desc, mp.RestrictValid, id) {
				return true
			}
			continue
		}
		free := pay.ManaFreeCost(cost)
		if !free && !castWindowGenericCostShape(cost) {
			if _, ok := e.paidCostShape(p, id, cost, e.G.Players[p].Life); !ok {
				return true
			}
		}
		if _, ok := e.castWindowAmount(p, id, o, ma); !ok {
			return true
		}
		counts := mp.Counts
		if mp.NeedsChosen {
			chosen := pay.ChosenProducedColour(e.G, id)
			if chosen == "" {
				return true
			}
			counts, _ = cards.ProducedCounts(pay.SubstituteChosenProduced(mp.Produced, chosen))
		}
		total := int32(0)
		for _, n := range counts {
			total += n
		}
		if total <= 0 {
			return true
		}
		if free && !mp.CountsAny && availableAmountOf(mp) > 0 {
			return true
		}
	}
	return false
}

// castWindowCensusComplete reports whether castWindowUnits priced every
// source the CR 601.2g window can offer: a permanent with a mana ability the
// shared gate walk admits (ignoring the live-pool gate, which a later step
// in the same window can satisfy) but that the probe's fail-closed exclusions
// left with no priced production makes the census incomplete, because that
// unpriced ability might be the production that closes the gap the probe
// measured short. A census that is complete lets a pool-short target-menu
// candidate be judged on the concrete reachability search alone; an
// incomplete one falls back to keeping the historical offer. So do a mana ability
// in hand or graveyard, which the census doesn't walk, and a production a tap
// trigger or a replacement would change, which it doesn't price (CR 605.1b).
func (e *Engine) castWindowCensusComplete(pc *pendingCast, units []windowManaUnit) bool {
	p := pc.player
	for _, z := range []state.Zone{state.ZHand, state.ZGraveyard} {
		for _, id := range e.G.Zone(z, p) {
			if e.untappedManaSource(p, id) {
				return false
			}
		}
	}
	if e.paymentPlanManaInterference() {
		return false
	}
	for _, u := range units {
		for _, a := range u.Alts {
			if e.manaProductionAltered(u.ID, a.Ma) {
				return false
			}
		}
	}
	priced := make(map[state.ObjID]bool, len(units))
	for _, u := range units {
		priced[u.ID] = true
	}
	desc := paymentForCast(pc, pc.resolvedMana())
	for _, id := range pay.BattlefieldManaSourceIDs(e.G, p) {
		if e.convokeCommitted(pc, id) {
			continue
		}
		if priced[id] {
			if e.castWindowRestrictedAdmits(p, id, desc) {
				return false
			}
			continue
		}
		if len(e.castWindowProbeAbilities(p, id)) > 0 && e.castWindowSourceExcluded(p, id, desc) {
			return false
		}
	}
	return true
}

// castWindowRestrictedAdmits reports whether a source priced by its plain mana
// abilities also has a RestrictValid$ one this payment may spend (CR 106.6,
// Tablet of Discovery's {R}{R} for an instant or sorcery): its units understate
// what the window can produce.
func (e *Engine) castWindowRestrictedAdmits(p state.PlayerID, id state.ObjID, desc pay.Descriptor) bool {
	for _, ma := range e.castWindowProbeAbilities(p, id) {
		if valid := effects.ManaOf(ma).RestrictValid; valid != "" && pay.RestrictValidMatches(asPayer(e), p, desc, valid, id) {
			return true
		}
	}
	return false
}

// castWindowAmount resolves a window mana ability's Amount$ the way the
// activation's own manaEffectAmount does -- the source face's SVar table and
// effects.Num's grammar -- but with the resolvability verdict the count
// ratchet demands: effects.NumResolvedStrict rejects a named SVar whose
// Count$ body the evaluator does not model, and the inline `Amount$ Count$...`
// form (which Strict does not itself re-check) is verified with EvalCountOK.
// A body that does not resolve deterministically, or resolves to zero or
// less, is not priced.
func (e *Engine) castWindowAmount(p state.PlayerID, source state.ObjID, o *state.Object, ma *cards.SA) (int32, bool) {
	mp := effects.ManaOf(ma)
	raw := mp.AmountTrim
	if raw == "" {
		return 1, true
	}
	if v := mp.AmountLit; mp.AmountIsLit {
		if v <= 0 {
			return 0, false
		}
		return int32(v), true
	}
	ctx := e.manaAmountCtx(p, source)
	effects.SetSVars(ctx, o.Face().SVars)
	n, ok := effects.ManaAmountResolvedStrict(mp, e, ctx, 1)
	if !ok || n <= 0 {
		return 0, false
	}
	ref := raw
	if len(ref) > 1 && (ref[0] == '+' || ref[0] == '-') {
		ref = ref[1:]
	}
	if strings.HasPrefix(ref, "Count$") {
		if _, evaluated := effects.EvalCountOK(e, ctx, ref); !evaluated {
			return 0, false
		}
	}
	return n, true
}

// castWindowPayLifeCost reports whether c is a PayLife<N> activation cost
// this probe can price: the fixed life part and nothing else (bar the tap).
// An announced PayLife<X> (LifeX), a life-halving token and every other cost
// component are refused.
func castWindowPayLifeCost(c Cost) bool {
	return c.Life > 0 && c.Generic == 0 && c.Colored == (state.Mana{}) &&
		len(c.Sac) == 0 && pay.CastWindowOtherPartsAbsent(c)
}

// castWindowGenericCostShape reports whether c is a literal generic <N>
// activation cost this probe can price: exactly one literal generic mana and
// nothing else (bar the tap). A coloured pip, an {X} component or any other
// part is refused.
//
// The pool is deliberately NOT consulted here: the fee may be funded by an
// earlier same-window activation, which the cast-only eligibility search
// (castWindowReachable) proves. The live window offers this source once it is
// untapped and re-checks payability at each activation
// (manaAbilityPayablePool), so promising only a source the search can actually
// fund stays sound.
func castWindowGenericCostShape(c Cost) bool {
	return c.Generic > 0 && c.Colored == (state.Mana{}) &&
		len(c.Sac) == 0 && pay.CastWindowOtherPartsAbsent(c)
}

// appendCastWindowAlt merges one production alternative into the unit for id
// (creating it if absent), so the affordability search can never tap the same
// permanent twice through two separate unit entries.
func appendCastWindowAlt(units []windowManaUnit, id state.ObjID, ma *cards.SA, counts [6]int32, amt, life, costGeneric int32) []windowManaUnit {
	idx := -1
	for i := range units {
		if units[i].ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		units = append(units, windowManaUnit{ID: id})
		idx = len(units) - 1
	}
	units[idx].Alts = append(units[idx].Alts, windowManaAlt{Ma: ma, Counts: counts, Amt: amt, Life: life, CostGeneric: costGeneric})
	return units
}

// castWindowReachable is the cast-only CR 601.2g eligibility search. It is
// unlessManaReachable plus the one transition the cast window has and the
// attack/unless windows do not: an activation may PAY a literal generic fee
// (windowManaAlt.costGeneric) and a life fee before its production is added,
// and that fee may be funded by mana an earlier same-window activation
// already produced. The live window offers one source at a time and
// re-enters after each activation, so the search explores executable source
// orders and never claims a paid source it cannot fund. Units are visited in
// stable fee/id order, but every remaining source can be selected next;
// every shared-window alt carries costGeneric 0 so the attack/unless callers
// of unlessManaReachable are untouched.
//
// spellPool is the payer's restriction-adjusted pool for the SPELL (the same
// projection unlessManaReachable receives); activation fees are funded from
// the payer's real pool minus every restricted batch a non-matching
// descriptor would hide (unrestrictedWindowPool), never from spell-restricted
// mana, so the sequence is executable under the live activation gate. Every
// produced unit is unrestricted (both probe walks exclude RestrictValid$
// abilities). rider is the cast's recorded may-play payment rider
// (pendingCast.mayPlayIgnore/mayPlayIgnoreType, captured at CR 601.2a while
// the card was still in the granted zone): "mana of any type can be spent to
// cast that spell" governs the whole payment, the window's productions
// included (CR 601.2g-h), exactly as the pool-only feasibility check beside
// each caller already applies it.
func (e *Engine) castWindowReachable(p state.PlayerID, cost Cost, spellPool, snow state.Mana,
	typed [7]state.Mana, life int32, conv *manaConv, rider pipRider, units []windowManaUnit) bool {
	// CR 119.8: under a CantChangeLife static no positive life may be paid,
	// whether by the cost or by a PayLife source's activation.
	if asEval(e).LifeChangeForbidden(p) {
		life = 0
	}
	payable := func(pool, snowPool state.Mana, lifeNow int32) bool {
		_, ok := resolveManaWith(cost, pool, snowPool, typed, lifeNow,
			asEval(e).PayLifeInsteadOfB(p), rider, conv)

		return ok
	}
	if payable(spellPool, snow, life) {
		return true
	}
	// Every activation adds at least one mana and a minimal sequence spends
	// some of each, so it needs no more activations than the cost's pips plus
	// the fees it pays.
	budget := cost.ManaPipCount()
	for _, u := range units {
		fee := int32(0)
		for _, a := range u.Alts {
			fee = max(fee, a.CostGeneric)
		}
		budget += int(fee)
	}
	free := e.unrestrictedWindowPool(p)
	// Stable free-first ordering: a free alt sorts before a paid one; ties
	// retain the zone walk's order, with the id as a determinism guard.
	ordered := append([]windowManaUnit(nil), units...)
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0; j-- {
			if !castWindowUnitLess(ordered[j], ordered[j-1]) {
				break
			}
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}
	nodes := 0
	used := make([]bool, len(ordered))
	// prevFree is the ordered index of the unit the previous step activated
	// through a FREE alternative (no generic fee, no life), or -1. Two free
	// activations in a row commute: neither spends anything, so either order
	// reaches the same pools at every later step. The search therefore takes
	// adjacent free activations in ascending index order only. Without this
	// the walk enumerated every ORDER of the payer's lands, and with ten
	// untapped sources whose first unit's first alternative is the wrong
	// colour (Blossoming Sands' {W} ahead of its {G}) the 1<<18 node cap
	// ran out before the right production was tried: a payable {G}{U}
	// activation was judged unreachable and every target dropped off its
	// Min 0 ask (lockstep pairs 1509 3986, Zimone, Paradox Sculptor).
	var rec func(pool, spellSnow, activationPool, activationSnow state.Mana, lifeLeft int32, prevFree, remaining int) bool
	rec = func(pool, spellSnow, activationPool, activationSnow state.Mana, lifeLeft int32, prevFree, remaining int) bool {
		if payable(pool, spellSnow, lifeLeft) {
			return true
		}
		if remaining <= 0 {
			return false
		}
		nodes++
		if nodes > 1<<18 {
			return false
		}
		for i := range ordered {
			if used[i] {
				continue
			}
			for _, a := range ordered[i].Alts {
				freeAlt := a.CostGeneric == 0 && a.Life == 0
				if freeAlt && prevFree > i {
					continue
				}
				// A positive life debit must be present before the tap; a
				// zero-debit alt is unaffected by the payer's life total,
				// even when the game has carried the payer below 0 (the
				// loser's remaining priority turns) -- a zero-debit guard
				// that reads lifeLeft directly would strand every alt
				// there.
				if a.Life > 0 && a.Life > lifeLeft {
					continue
				}
				// Pay a generic activation fee from mana the live activation
				// gate can spend. Track the same spent colours in the spell
				// pool; fees reduce that pool, they are not extra spell pips.
				activationFee, feeOK := resolveMana((Cost{Generic: a.CostGeneric}),
					activationPool, activationSnow, [7]state.Mana{}, lifeLeft, nil)

				if !feeOK {
					continue
				}
				spent := manaSub(activationPool, activationFee.Pool)
				snowSpent := manaSub(activationSnow, activationFee.Snow)
				nextPool := manaSub(pool, spent)
				nextSpellSnow := manaSub(spellSnow, snowSpent)
				nextActivation := activationFee.Pool
				nextActivationSnow := activationFee.Snow
				produced := a.Mana()
				next := -1
				if freeAlt {
					next = i
				}
				used[i] = true
				found := rec(pay.ManaAdd(nextPool, produced), nextSpellSnow,
					pay.ManaAdd(nextActivation, produced), nextActivationSnow, lifeLeft-a.Life, next, remaining-1)
				used[i] = false
				if found {
					return true
				}
			}
		}
		return false
	}
	return rec(spellPool, snow, free, snow, life, -1, budget)
}

func manaSub(a, b state.Mana) state.Mana {
	var m state.Mana
	for i := range m {
		m[i] = a[i] - b[i]
		if m[i] < 0 {
			m[i] = 0
		}
	}
	return m
}

// castWindowUnitLess orders a unit before another when its cheapest
// alternative is cheaper in generic activation cost, tie-broken by zone id,
// giving castWindowReachable a stable traversal order.
func castWindowUnitLess(a, b windowManaUnit) bool {
	ka, kb := int32(-1), int32(-1)
	for _, x := range a.Alts {
		if ka < 0 || x.CostGeneric < ka {
			ka = x.CostGeneric
		}
	}
	for _, x := range b.Alts {
		if kb < 0 || x.CostGeneric < kb {
			kb = x.CostGeneric
		}
	}
	if ka != kb {
		return ka < kb
	}
	return a.ID < b.ID
}

// unrestrictedWindowPool is the payer's real floating pool minus every
// restricted batch (a non-empty Valid) a non-matching descriptor would hide.
// Only these units are guaranteed spendable on a mana-ability activation
// regardless of the activation's own restriction terms, so castWindowReachable
// funds activation fees from them and never from spell-restricted mana.
// Empty-Valid batches are unrestricted (Boseiju's AddsNoCounter$ provenance)
// and stay counted, matching manaAvailableFor's own rule.
func (e *Engine) unrestrictedWindowPool(p state.PlayerID) state.Mana {
	pl := e.G.Players[p]
	free := pl.Pool
	for _, r := range pl.RestrictedMana {
		if r.Valid == "" {
			continue
		}
		idx := state.ManaSlot(r.Color)
		if free[idx] < r.Amount {
			free[idx] = 0
		} else {
			free[idx] -= r.Amount
		}
	}
	return free
}

// striveAffordableTargets is the Decision.AffordableTargets hint for a Strive
// spell's target ask: the largest n in [1, max] whose total cost -- the
// resolved cost, n-1 Strive payments (CR 702.52a), the proposal's modifier
// snapshot, commander tax, Delve credit and announced Convoke -- the caster
// can provably pay from the pool plus the window's fixed productions
// (castWindowUnits, the same probe the target-discount gate trusts). It
// returns 0 (no hint) when the proposal carries no priceable Strive or max is
// at most one; it returns at least 1 otherwise, since one target adds no
// Strive charge and the ask's own gate already admitted the base cost. A pure
// read: nothing is emitted.
func (e *Engine) striveAffordableTargets(pc *pendingCast, max int) int {
	if pc.isAbility() || !pc.striveSet || max <= 1 {
		return 0
	}
	o := e.G.Obj(pc.card)
	if o == nil || o.Face() == nil {
		return 0
	}
	if _, ok := o.Face().KeywordParam("Strive"); !ok {
		return 0
	}
	sc := ParseCost(pc.striveParam)
	if len(sc.Unknown) > 0 {
		return 0
	}
	pl := e.G.Players[pc.player]
	var units []windowManaUnit
	unitsBuilt := false
	affordable := func(n int) bool {
		cost := pc.resolvedMana()
		for i := int(pc.striveUnits); i < n-1; i++ {
			cost = cost.Plus(sc)
		}
		cost = pc.mods.Apply(cost)
		cost.Generic = addClampedGeneric(cost.Generic, int64(pc.taxGeneric))
		cost.Generic -= int32(len(pc.delve))
		if cost.Generic < 0 {
			cost.Generic = 0
		}
		convoked := e.applyConvoke(pc, cost)
		desc := paymentForCast(pc, convoked)
		rider := pipRider{AnyColor: pc.mayPlayIgnore, AnyType: pc.mayPlayIgnoreType}
		if e.manaFeasibleDescriptor(pc.player, desc, convoked, costMods{}, 0, 0, rider) {
			return true
		}
		if !convoked.HasManaPayment() {
			return false
		}
		if !unitsBuilt {
			units, unitsBuilt = e.castWindowUnits(pc), true
		}
		av := pay.AvailableFor(asPayer(e), pc.player, desc)
		return e.castWindowReachable(pc.player, convoked, av.Pool, pl.Snow, av.Typed, pl.Life,
			asEval(e).Conv(pc.player, desc.ID, desc.Class == paymentActivated), rider, units)
	}
	// Ascending: the price is monotone in n, so the first unaffordable count
	// ends the walk and at most one exhaustive (failing) window search runs.
	best := 1
	for n := 2; n <= max; n++ {
		if !affordable(n) {
			break
		}
		best = n
	}
	return best
}

// targetCountAffordable is the Decision.AffordableTargets hint for a spell whose
// own cost reads how many targets it announces (Fireball, Officious
// Interrogation): the largest n in [1, limit] whose total cost CR 601.2f composes,
// with X already announced, the pool and the window can pay. Only the count is
// read, so any n of the candidates price it. It returns 0 (no hint) for any
// other spell. A pure read, like striveAffordableTargets.
func (e *Engine) targetCountAffordable(pc *pendingCast, candidates []targetCandidate, limit int) int {
	o := e.G.Obj(pc.card)
	if pc.isAbility() || limit <= 1 || o == nil || o.Face() == nil || !o.Face().Mentions("TargetedObjects$Amount") {
		return 0
	}
	scope, ok := e.pendingCastScope(pc)
	if !ok {
		return 0
	}
	targets := make([]state.Target, limit)
	for i := range targets {
		targets[i] = candidateTarget(candidates[i])
	}
	units := e.castWindowUnits(pc)
	best := 1
	for n := 2; n <= limit; n++ {
		mods := pc.withDefiler(e.costModifiersForTargets(pc.player, pc.card, scope, targets[:n]))
		cost := e.paymentManaXUsing(pc, pc.x, mods)
		cost.Generic = max(cost.Generic-int32(len(pc.delve)), 0)
		if !e.castWindowPays(pc, cost, units) {
			break
		}
		best = n
	}
	return best
}
