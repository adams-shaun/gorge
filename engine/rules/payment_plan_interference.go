package rules

// Mana/tap interference scoping for payment plans.
// Like payment_plan.go this file contains no call to emit: every read below
// is a pure query of the current state, run against a HYPOTHETICAL tap and
// production of one candidate source through the engine's own trigger and
// replacement matchers.
//
// A trigger or replacement on source S affects only S's tier:
//
//   - S's own `T:Mode$ Taps | ValidCard$ Card.Self` whose effect is exactly
//     `DealDamage | Defined$ You | NumDmg$ <literal>` (City of Brass) makes S
//     last resort with consequence damage:N;
//   - S's own `R:Event$ Untap | ValidCard$ Card.Self | Layer$ CantHappen`
//     doesn't-untap replacement (Mana Vault, Grim Monolith) makes S last
//     resort with consequence no_untap;
//   - any other Taps/TapsForMana trigger or ProduceMana replacement -- S's own
//     or another object's, printed, granted or effect-created -- that the real
//     matcher says would apply to S's activation defers S, naming the object;
//     two trigger rows are instead post-payment consequences that touch no
//     payment resource and stay ordinary units (the destroy-self row and the
//     poison-counter row below, joined by PaymentPlanTapConsequenceTrigger).
//
// Untap and LoseMana replacements on other objects alter neither a tap nor a
// production and are ignored. Only a global effect whose scope the planner
// cannot prove declines every plan of the affected player
// (paymentPlanGlobalManaEffect).

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/rules/pay"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/trigmatch"
	"github.com/adams-shaun/gorge/engine/state"
)

// paymentPlanGlobalManaEffect reports whether a mana effect whose scope the
// planner cannot prove reaches p paying for id, and names it as
// "global_mana_effect:<card name>". Exactly two shapes qualify:
//
//   - an effect-created ProduceMana replacement carrying no ValidCard$ or
//     ValidActivator$ filter (its scope is every production of every
//     player; a filtered one is scoped per source by the real matcher);
//   - a RESTRICTING ManaConvert static that reaches the payer, found the way
//     the payment path finds it (paymentConv) and classified by
//     paymentPlanConvRestricts: the planner prices with the ordinary solver,
//     which does not apply the conversion. A purely widening conversion
//     (Mycosynth Lattice) cannot make a priced plan unpayable and is not
//     global.
//
// id may be 0 when no spell is known (the zero-argument wrapper outside a
// cast); a ManaConvert static then reaches p only through its player scope.
func (e *Engine) paymentPlanGlobalManaEffect(p state.PlayerID, id state.ObjID) (bool, string) {
	for ceI, ceL := 0, e.active(); ceI < len(ceL); ceI++ {
		ce := &ceL[ceI]
		if ce.ReplacementEvent != "ProduceMana" {
			continue
		}
		if strings.TrimSpace(ce.ReplacementParam(cards.PKValidCard)) == "" && strings.TrimSpace(ce.ReplacementParam(cards.PKValidActivator)) == "" {
			return true, "global_mana_effect:" + pay.PaymentPlanObjName(asPayer(e), ce.Source)
		}
	}
	if conv := asEval(e).Conv(p, id, pay.SASpell); conv != nil && pay.PaymentPlanConvRestricts(conv) {
		return true, "global_mana_effect:" + e.paymentPlanManaConvertName(p)
	}
	return false, ""
}

// paymentPlanManaConvertName names the ManaConvert static behind a global
// decline, for the diagnostic only (the decision is paymentConv's, classified
// by paymentPlanConvRestricts): the first static reaching p whose
// ManaConversion$ carries a restricting "<-" token, printed sources in board
// order before active effects; else the first static reaching p.
func (e *Engine) paymentPlanManaConvertName(p state.PlayerID) string {
	var views []staticView
	for _, src := range e.manaConvPrintedSources() {
		views = append(views, src.sv)
	}
	for ceI, ceL := 0, e.active(); ceI < len(ceL); ceI++ {
		ce := &ceL[ceI]
		if ce.CostStaticMode == "ManaConvert" {
			views = append(views, staticView{Source: ce.Source, Controller: ce.Controller, Params: ce.CostStaticParams})
		}
	}
	first := state.ObjID(0)
	for _, sv := range views {
		if vp, ok := sv.Param(cards.PKValidPlayer); ok && !effects.MatchesPlayerSpec(e.G, vp, p, sv.Controller) {
			continue
		}
		if strings.Contains(sv.ParamStr(cards.PKManaConversion), "<-") {
			return pay.PaymentPlanObjName(asPayer(e), sv.Source)
		}
		if first == 0 {
			first = sv.Source
		}
	}
	if first != 0 {
		return pay.PaymentPlanObjName(asPayer(e), first)
	}
	return "ManaConvert"
}

// paymentPlanSourceInterference classifies what can act on activating ma on
// source id: pay.TierNormal when nothing does, pay.TierLastResort with
// the source's own fully determined consequence, or pay.TierDeferred with
// "source:interference:<card name>" naming the object whose trigger or
// replacement would apply. A choice-shaped production is checked for every
// colour it can be planned as, so a trigger restricted to one colour defers
// the whole source rather than one alternative.
func (e *Engine) paymentPlanSourceInterference(id state.ObjID, ma *cards.SA) (pay.Tier, pay.Consequence, string) {
	src := e.G.Obj(id)
	if src == nil || src.Face() == nil || ma == nil {
		return pay.TierDeferred, pay.Consequence{}, "source:interference"
	}
	deferredBy := func(obj state.ObjID) (pay.Tier, pay.Consequence, string) {
		return pay.TierDeferred, pay.Consequence{}, "source:interference:" + pay.PaymentPlanObjName(asPayer(e), obj)
	}
	var c pay.Consequence
	// The source's own "doesn't untap during your untap step". An Untap
	// replacement naming another object (Claustrophobia's enchanted creature)
	// is not about this source's tap at all.
	for _, r := range src.Face().Repls {
		if r.Event != "Untap" || !strings.HasPrefix(strings.TrimSpace(r.ParamStr(cards.PKValidCard)), "Card.Self") {
			continue
		}
		if !pay.PaymentPlanNoUntapShape(r) {
			return deferredBy(id)
		}
		c.NoUntap = true
	}
	controller := src.Controller
	cost := e.abilityCompiledCost(ma).Cost
	for _, produced := range pay.PaymentPlanProductions(asPayer(e), id, ma) {
		if cost.Tap {
			dmg, _, by, ok := e.paymentPlanTapObservers(id, controller, produced, false, nil)
			if !ok {
				return deferredBy(by)
			}
			// One activation fires the trigger once whichever colour it
			// takes: the consequence is the largest, not the sum.
			c.Damage = max(c.Damage, dmg)
		}
		// A dynamic Amount$ (Gaea's Cradle) is sized now: Damping Sphere's
		// ManaAmount$ GE2 reads how much the production really is.
		amount := availableAmount(ma)
		if amount == 0 {
			amount = e.manaEffectAmount(controller, id, ma, nil, nil, pay.GainedManaRef{})
		}
		if by, ok := e.paymentPlanProductionReplaced(id, controller, produced, amount, cost.Tap); ok {
			return deferredBy(by)
		}
	}
	if c != (pay.Consequence{}) {
		return pay.TierLastResort, c, "source:last_resort"
	}
	return pay.TierNormal, pay.Consequence{}, ""
}

// paymentPlanTapExtraMana is the additional mana the deterministic static
// TapsForMana triggers (pay.PaymentPlanFoldableTapMana) add when id taps for
// ma producing `produced`. It is the same walk paymentPlanSourceInterference
// ran, taken whole: a source that verdict deferred (Gran-Gran's own "becomes
// tapped" loot beside Great Divide Guide's granted {T}) still gets every
// folded trigger's mana when it taps (CR 605.4a), so the potential bound and
// the relaxed proof, which may only over-produce, read it too.
func (e *Engine) paymentPlanTapExtraMana(id state.ObjID, produced string) state.Mana {
	src := e.G.Obj(id)
	if src == nil {
		return state.Mana{}
	}
	_, extra, _, _ := e.paymentPlanTapObservers(id, src.Controller, produced, true, nil)
	return extra
}

// paymentPlanTapOpenExtra is the most the static TapsForMana triggers whose
// mana the player colours at the tap (pay.PaymentPlanOpenTapMana: Dawn's
// Reflection, Fertile Ground) can add when id taps for `produced`, as the
// potential bound reads it: those triggers are not folded into the plan (the
// source stays deferred), yet the mana is real (CR 605.3b), so a bound that
// ignored it would hide a cast the manual payment can pay.
func (e *Engine) paymentPlanTapOpenExtra(id state.ObjID, produced string) state.Mana {
	src := e.G.Obj(id)
	if src == nil {
		return state.Mana{}
	}
	var open state.Mana
	e.paymentPlanTapObservers(id, src.Controller, produced, true, &open)
	return open
}

// manaProductionAltered reports whether a tap trigger or a replacement would change
// what id's activation of ma adds to the pool.
func (e *Engine) manaProductionAltered(id state.ObjID, ma *cards.SA) bool {
	if tier, _, _ := e.paymentPlanSourceInterference(id, ma); tier == pay.TierDeferred {
		return true
	}
	productions := pay.PaymentPlanProductions(asPayer(e), id, ma)
	if alt, ok := paymentPlanReflectedAlt(e, e.G.Obj(id).Controller, id, ma, 1, false); ok {
		productions = paymentPlanReflectedProductions(alt)
	}
	for _, produced := range productions {
		if e.paymentPlanTapExtraMana(id, produced) != (state.Mana{}) {
			return true
		}
	}
	return false
}

// paymentPlanTapObservers runs every Taps/TapsForMana trigger that could see
// a mana tap of id (activated by activator, declaring produced) through the
// real matcher. It returns the source's own City-of-Brass damage, or ok=false
// with the object whose trigger would otherwise act on the tap or the mana.
// whole keeps walking past such an object (by and ok still name the first),
// so extra sums every folded trigger whatever else watches the tap. A non-nil
// open also accumulates the bound of each open-colour trigger's mana
// (pay.PaymentPlanOpenTapMana), which extra never carries.
//
// The printed walk visits every carrier object (paymentPlanInterferenceCarriers)
// of an alive player that the engine's own trigger walk would consider --
// printed faces, unlocked Room faces and mutated under-cards, as
// checkFaceTriggers walks them -- and leaves the zone question to the real
// zoneGate. Granted-trigger statics that observe a Tap are walked over the
// battlefield recipients their Affected$ admits.
func (e *Engine) paymentPlanTapObservers(id state.ObjID, activator state.PlayerID, produced string, whole bool, open *state.Mana) (damage uint32, extra state.Mana, by state.ObjID, ok bool) {
	// The synchronous context emitManaTap/emitTap establish around the Tap
	// event, restored before return: this is matcher scratch, not game state.
	// The mana facts mirror what resolveManaEffectColor establishes around a
	// mana ability's ManaAdd emissions, so a Mode$ ManaAdded line matches the
	// synthetic add exactly as it will match the real one.
	savedObj, savedPlayer, savedEntering := e.tapObj, e.tapPlayer, e.tapEntering
	savedMana, savedProduced := e.tappingForMana, e.tappingManaProduced
	savedProducer, savedManaAbility := e.manaProducer, e.manaAddFromManaAbility
	e.tapObj, e.tapPlayer, e.tapEntering = id, activator, false
	e.tappingForMana, e.tappingManaProduced = id, produced
	e.manaProducer, e.manaAddFromManaAbility = id, true
	defer func() {
		e.tapObj, e.tapPlayer, e.tapEntering = savedObj, savedPlayer, savedEntering
		e.tappingForMana, e.tappingManaProduced = savedMana, savedProduced
		e.manaProducer, e.manaAddFromManaAbility = savedProducer, savedManaAbility
	}()
	ev := events.Event{Kind: events.Tap, Obj: id}
	ok = true
	// deferBy records the first object whose trigger defers the tap; the walk
	// stops there unless whole.
	deferBy := func(obj state.ObjID) bool {
		if ok {
			by, ok = obj, false
		}
		return !whole
	}
	tapMode := func(mode string) bool { return mode == "Taps" || mode == "TapsForMana" }
	for _, oid := range pay.PaymentPlanInterferenceCarriers(asPayer(e)) {
		o := e.G.Obj(oid)
		if o == nil || o.PhasedOut || o.Face() == nil || e.printedAbilitiesGone(o) || !pay.PaymentPlanAlive(asPayer(e), o) {
			continue
		}
		faces, n := roomTriggerFaces(o, o.Face())
		walk := faces[:n]
		if len(o.MergedCards) > 0 {
			walk = triggerFacesWithMerged(o, walk)
		}
		for _, fc := range walk {
			for _, t := range fc.face.Triggers {
				if t.ModeKind() == cards.TriggerManaAdded {
					// A Mode$ ManaAdded line is first matched against the
					// synthetic ManaAdd batches, exactly as the real
					// resolution's add will match them (the mana facts above
					// stand in for emitManaTap's). A line its own filters
					// reject -- Caged Sun's ValidSA$ SpellAbility.!ManaAbility
					// Secondary line on a mana-ability add, a land-source or
					// chosen-colour gate that fails -- cannot fire on this
					// production and is silent, not deferring. A deferring
					// line reports through deferBy like every other unvetted
					// observer, and the walk's tail check returns it: the
					// partial extra it may have built is discarded with ok.
					mo := manaAddedFold{
						face:     fc.face,
						matches:  e.triggerMatches,
						replaced: e.paymentPlanExtraReplaced,
					}
					mo.observe(t, oid, activator, produced, deferBy, &extra)
					continue
				}
				if !tapMode(t.Mode) || !e.triggerMatches(t, oid, ev, nil) {
					continue
				}
				if oid == id && fc.active && fc.merged == 0 {
					if d, self := pay.PaymentPlanSelfDamageTrigger(fc.face, t); self {
						damage += d
						continue
					}
				}
				// A folded trigger's mana goes to the trigger controller (absent
				// Defined$ or You) or to the tapped permanent's controller, which
				// is the payer; a ProduceMana replacement on that extra mana
				// defers as it does for the source's own production.
				tm, folds := pay.PaymentPlanFoldableTapMana(fc.face, t)
				if !folds {
					// a reflected body (Lavaleaper) folds as one mana of the
					// single type this production names
					if tm, folds = pay.PaymentPlanReflectedTapMana(fc.face, t); folds {
						tm.Mana, folds = pay.PaymentPlanReflectedMana(produced)
					}
				}
				if !folds && open != nil {
					if body, toTapped, isOpen := pay.PaymentPlanOpenTapMana(fc.face, t); isOpen && (toTapped || o.Controller == activator) {
						addPotentialOpenTapMana(open, body)
					}
				}
				if folds && (tm.ToTapped || o.Controller == activator) {
					if rby, replaced := e.paymentPlanExtraReplaced(oid, activator, tm.Mana); replaced {
						if deferBy(rby) {
							return 0, state.Mana{}, by, false
						}
						continue
					}
					for i := range extra {
						extra[i] += tm.Mana[i]
					}
					continue
				}
				// A foreign tap-damage trigger (Hawkeye's Bow) resolves after the
				// payment that caused it (CR 603.2) and consumes no payment
				// resource: the vetted source stays an ordinary unit. Only
				// damage that can reach the payer itself (PaymentPlanTapDamageHits)
				// is disclosed as the source's consequence, which the last-resort
				// phase's life gate prices as caster-paid; anything the
				// classifier cannot vet defers the source.
				if d, ref, known := pay.PaymentPlanTapDamageTrigger(fc.face, t); known {
					if pay.PaymentPlanTapDamageHits(ref, o.Controller, activator, e.G.Obj(id).Controller) {
						damage += d
					}
					continue
				}
				// A trigger whose only effect is to destroy the tapped
				// permanent itself (Brink of Disaster's "when enchanted
				// permanent becomes tapped, destroy it") or to give a player
				// the payment cannot move a poison counter (Relic Putrescence's
				// "whenever enchanted artifact becomes tapped, its controller
				// gets a poison counter") is a post-payment consequence, not
				// interference: the mana was produced at the tap (CR 605.4a)
				// and the trigger resolves after the payment that caused it
				// (CR 405.6c/603.2), touching no payment resource, so the tap
				// stays an ordinary unit and the trigger fires on its own at
				// the tap, on both engines.
				if pay.PaymentPlanTapConsequenceTrigger(fc.face, t) {
					continue
				}
				if deferBy(oid) {
					return 0, state.Mana{}, by, false
				}
			}
		}
	}
	// Triggers granted by a static (AddTrigger$ / GainsTriggerAbsOf$): any
	// battlefield recipient whose granted Taps/TapsForMana trigger would see
	// this tap defers the source, named by the granting object.
	for _, ce := range grantedTriggerStaticsFor(e.active(), events.Tap, nil) {
		var granted []cards.Trigger
		var svars []map[string]string
		if ce.AddTrigger != nil && tapMode(ce.AddTrigger.Mode) {
			t := *ce.AddTrigger
			grantor := ce.Source
			if ce.TriggerGrantor != 0 {
				grantor = ce.TriggerGrantor
			}
			var sv map[string]string
			if f := grantedTriggerFace(e.G.Obj(grantor), t.ParamStr(cards.PKExecute)); f != nil {
				sv = f.SVars
			}
			granted, svars = append(granted, t), append(svars, sv)
		}
		for _, gf := range ce.GainedTriggerFaces {
			if gf.Face == nil {
				continue
			}
			for _, t := range gf.Face.Triggers {
				if tapMode(t.Mode) {
					granted, svars = append(granted, t), append(svars, gf.Face.SVars)
				}
			}
		}
		if len(granted) == 0 {
			continue
		}
		for _, p := range e.G.AliveFrom(0) {
			for _, rid := range e.G.Zone(state.ZBattlefield, p) {
				if !e.matchesSpecFrom(ce.Affects, rid, ce.Controller, ce.Source) {
					continue
				}
				for i, t := range granted {
					if e.triggerMatchesWithSVars(t, rid, ev, nil, svars[i]) && deferBy(ce.Source) {
						return 0, state.Mana{}, by, false
					}
				}
			}
		}
	}
	if dby, seen := e.paymentPlanDelayedTapObserver(id, ev); seen && deferBy(dby) {
		return 0, state.Mana{}, by, false
	}
	if !ok {
		return 0, extra, by, false
	}
	return damage, extra, 0, true
}

// manaAddedFold is the feature struct paymentPlanTapObservers runs a Mode$
// ManaAdded line through. It holds only the two walks the fold needs -- the
// real trigger matcher against a synthetic ManaAdd batch, and the
// ProduceMana-replacement walk on the folded bonus -- so its method is not
// engine surface.
type manaAddedFold struct {
	face     *cards.Face
	matches  func(t cards.Trigger, oid state.ObjID, ev events.Event, lki *state.Object) bool
	replaced func(oid state.ObjID, activator state.PlayerID, m state.Mana) (state.ObjID, bool)
}

// observe folds the ManaAdded bonus of trigger t on carrier oid into extra:
// it matches t's own filters against one synthetic ManaAdd batch per symbol
// the planned production names (exactly as the real resolution's adds will
// match, the mana facts standing in for emitManaTap's), and credits one unit
// per matched batch -- one batch of a symbol triggers the printed "add one
// or more mana" once, whatever its amount, the batch cadence effMana's
// coalescing and the real resolution agree on. A line whose own filters
// reject every batch -- Caged Sun's ValidSA$ SpellAbility.!ManaAbility
// Secondary line on a mana-ability add, a land-source or chosen-colour gate
// that fails -- is silent on the planned tap, not deferring (the chosen
// colour read sits inside the matcher's Produced$ ChosenColor arm, so a
// carrier with no colour recorded neither folds nor defers). A matched line
// the classifier cannot vet, or whose bonus a ProduceMana replacement would
// act on, defers the tap through deferBy, the direction every unmodelled
// observer takes. It reports whether a line deferred the tap; the walk's
// tail check returns it, and the caller discards the partial extra with ok.
func (f *manaAddedFold) observe(t cards.Trigger, oid state.ObjID, activator state.PlayerID, produced string, deferBy func(state.ObjID) bool, extra *state.Mana) bool {
	counts, _ := cards.ProducedCounts(produced)
	for i, n := range counts {
		if n <= 0 {
			continue
		}
		evAdd := events.Event{Kind: events.ManaAdd, Player: activator, Counter: string(cards.ManaSymbol(i))}
		if !f.matches(t, oid, evAdd, nil) {
			continue
		}
		if !pay.PaymentPlanFoldableManaAdded(f.face, t) {
			deferBy(oid)
			return true
		}
		var unit state.Mana
		unit[i] = 1
		if rby, replaced := f.replaced(oid, activator, unit); replaced {
			deferBy(rby)
			return true
		}
		(*extra)[i]++
	}
	return false
}

// paymentPlanExtraReplaced reports the first ProduceMana replacement that
// applies to the mana a folded trigger of oid adds for activator.
func (e *Engine) paymentPlanExtraReplaced(oid state.ObjID, activator state.PlayerID, m state.Mana) (state.ObjID, bool) {
	for i, n := range m {
		if n <= 0 {
			continue
		}
		if by, ok := e.paymentPlanProductionReplaced(oid, activator, string(cards.ManaSymbol(i)), n, false); ok {
			return by, true
		}
	}
	return 0, false
}

// paymentPlanDelayedTapObserver reports the source of the first live
// effect-created Taps/TapsForMana registration (state.Game.Delayed with
// EffectRepeat: Bubbling Muck's "until end of turn, whenever a player taps a
// Swamp for mana, that player adds an additional {B}") that would see ev.
// It reads the registration exactly as checkEventDelayedTriggers' EffectRepeat
// arm does -- the stored body, the generic trigMatchers dispatch, matched as
// the registration's owner under the effect-match scope -- and restores that
// matcher scratch before returning. A non-repeating registration with a tap
// mode is never fired by that scan, so it observes nothing. Without this walk
// the planner priced a Swamp at {B} while the tap added {B}{B} (cardfuzz
// cardfuzz mirror seed 14886721532440673633: wrong_production and a
// pool_after the witness did not promise).
func (e *Engine) paymentPlanDelayedTapObserver(id state.ObjID, ev events.Event) (state.ObjID, bool) {
	savedSrc, savedCtl := e.effectMatchSource, e.effectMatchController
	savedRem, savedOverride := e.effectMatchRemembered, e.effectMatchOverride
	defer func() {
		e.effectMatchSource, e.effectMatchController = savedSrc, savedCtl
		e.effectMatchRemembered, e.effectMatchOverride = savedRem, savedOverride
	}()
	for i := range e.G.Delayed {
		dt := &e.G.Delayed[i]
		if !dt.EffectRepeat || (dt.EventMode != "Taps" && dt.EventMode != "TapsForMana") || dt.Trigger == "" {
			continue
		}
		if (dt.MaxTurn > 0 && e.G.Turn > dt.MaxTurn) || !e.delayedRegistrationLive(dt) {
			continue
		}
		raw := dt.Trigger
		if !strings.HasPrefix(raw, "Mode$") {
			raw = events.SVarAcrossFaces(e.G.Obj(dt.Source), raw)
		}
		t, ok := cards.ParseTriggerLine(raw)
		if !ok || t.Mode != dt.EventMode {
			continue
		}
		fn := trigmatch.LookupMode(t.ModeKind())
		if fn == nil || !triggerLineEvents(&t).allows(ev.Kind) {
			continue
		}
		e.effectMatchSource, e.effectMatchController = dt.Source, dt.Controller
		e.effectMatchRemembered, e.effectMatchOverride = dt.Remembered, true
		if fn(boardOf(e), t, dt.Source, ev, nil) {
			return dt.Source, true
		}
	}
	return 0, false
}

// paymentPlanProductionReplaced reports the first object whose ProduceMana
// replacement -- printed on a carrier's face (paymentPlanInterferenceCarriers,
// in any zone: activeZonesGateOK, applied by the matcher, is the zone
// authority) or created by an effect -- the real matcher applies to id
// producing `produced` (amount units per symbol) for activator. Each produced
// symbol is proposed as its own ManaAdd batch, the shape effMana emits.
func (e *Engine) paymentPlanProductionReplaced(id state.ObjID, activator state.PlayerID, produced string, amount int32, fromTap bool) (state.ObjID, bool) {
	if amount <= 0 {
		amount = 1
	}
	active := e.active()
	carriers := pay.PaymentPlanInterferenceCarriers(asPayer(e))
	// With no effect-created ProduceMana replacement and no carrier object
	// both walks below find nothing for any symbol: answer without parsing
	// the production.
	if len(carriers) == 0 {
		produceMana := false
		for i := range active {
			if active[i].ReplacementEvent == "ProduceMana" {
				produceMana = true
				break
			}
		}
		if !produceMana {
			return 0, false
		}
	}
	counts, _ := cards.ProducedCounts(produced)
	savedTap, savedProducer := e.manaFromTap, e.manaProducer
	e.manaFromTap, e.manaProducer = fromTap, id
	defer func() { e.manaFromTap, e.manaProducer = savedTap, savedProducer }()
	for i, n := range counts {
		if n <= 0 {
			continue
		}
		ev := events.Event{Kind: events.ManaAdd, Player: activator, Counter: string(cards.ManaSymbol(i)), Amount: n * amount}
		for ceI, ceL := 0, active; ceI < len(ceL); ceI++ {
			ce := &ceL[ceI]
			if ce.ReplacementEvent != "ProduceMana" {
				continue
			}
			r := cards.Repl{Event: ce.ReplacementEvent, Params: ce.ReplacementParams, With: replacementBodySA(ce.ReplacementBody)}
			if e.replacementMatchesEffectCreatedBy(r, ce.Source, ev, ce.Remembered, ce.RememberedPlayers, ce.Controller) {
				return ce.Source, true
			}
		}
		for _, oid := range carriers {
			o := e.G.Obj(oid)
			if o == nil || !pay.PaymentPlanAlive(asPayer(e), o) {
				continue
			}
			f := e.replacementFace(oid, ev)
			if f == nil {
				continue
			}
			for _, r := range f.Repls {
				if r.Event == "ProduceMana" && e.replacementMatches(r, oid, ev) {
					return oid, true
				}
			}
		}
	}
	return 0, false
}

// paymentPlanSunburstGrantOut is the planner-local sunburst arm of the cast
// shape gate: whether any alive player's battlefield face GRANTS sunburst to
// a spell (Solar Array's and Lux Artillery's `Animate | Keywords$ Sunburst`,
// or a Pump/Continuous keyword grant). The shared capture gate
// sunburstGrantOut deliberately over-reads any mention -- it only arms a
// pay-time capture -- but a face with its own K:Sunburst (Engineered
// Explosives) changes nothing about another spell's payment.
func (e *Engine) paymentPlanSunburstGrantOut() bool {
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			// faceGrantsSunburstForPlan, memoised (face_scan_memo.go).
			// CR 702.26b: a phased-out permanent grants nothing
			if o := e.G.Obj(id); o != nil && !o.PhasedOut && e.faceScanHas(o.Face(), faceScanSunburstGrantPlan) {
				return true
			}
		}
	}
	return false
}

// addPotentialOpenTapMana folds into m the bound of an any-colour static
// TapsForMana body: a literal Amount$ of "Any" or "Combo Any" is that many
// mana of each of the five colours (never colourless). Any other open body
// (Utopia Sprawl's chosen colour, a counted amount) adds nothing: the bound
// keeps its pre-existing reading of those taps.
func addPotentialOpenTapMana(m *state.Mana, body *cards.SA) {
	mp := effects.ManaOf(body)
	amt, indeterminate := potentialAmount(mp)
	if indeterminate || (mp.Produced != "Any" && mp.Produced != "Combo Any") {
		return
	}
	for i := 0; i < 5; i++ {
		m[i] = saturatingPotentialMana(m[i], amt)
	}
}
