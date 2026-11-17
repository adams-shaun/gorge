package rules

import (
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// autoPayOffer reports whether the real-pool offer walk should offer the
// printed activated ability pile index idx of id although the floating pool
// cannot pay its cost: the planner has a witness that taps the player's mana
// sources for it, and the engine's own offer gate, priced with the pool plus
// that witness's production, confirms it (CR 602.2b: mana abilities may be
// activated while paying an activation's costs). beginActivation arms the
// same witness (armAbilityAutoPay), so the offer and the payment cannot drift.
//
// The potential walk (hyp != nil) already prices against the bound, so only
// the ordinary walk asks. Every non-mana part of the cost is read for real by
// the confirming gate, and the witness is planned so those parts stay payable.
func (w *legalWalk) autoPayOffer(p state.PlayerID, id state.ObjID, idx int, base Cost, ab *cards.SA) bool {
	if w.hyp != nil || !w.e.paidManaCensus || !(abilityCostNeedsMana(base) || len(base.Phyrexian) > 0) {
		return false
	}
	scope := abilityScope(ab)
	statics := w.costStatics.get()
	if w.autoBound == nil {
		b := w.e.PotentialMana(p)
		w.autoBound = &b
	}
	if *w.autoBound == w.e.G.Players[p].Pool {
		return false // nothing to tap: the pool is already the bound.
	}
	bound := *w.autoBound
	probe := base
	if !w.e.offerCastableUsing(statics, p, id, &probe, scope, true, &bound) {
		return false
	}
	if len(base.Phyrexian) > 0 {
		return w.autoPayPhyrexian(p, id, idx, base, scope, statics)
	}
	// The exact fast path (autoPayFast): when it proves the planner would
	// accept, skip the planner. Under verify mode it still runs the planner
	// and panics on disagreement, so the two can never silently drift.
	if w.autoPayFast(p, id, base, ab, statics) {
		if autoPayFastVerify && !w.autoPayPlan(p, id, idx, base, scope, statics) {
			panic("rules: autopay fast path accepted where the planner declined")
		}
		return true
	}
	return w.autoPayPlan(p, id, idx, base, scope, statics) || w.autoPayScriptOffer(p, id, idx, scope)
}

// autoPayPlan is the ordinary autopay path: run the payment planner and
// confirm its witness with the engine's own offer gate, exactly as autoPayOffer
// always did before the fast path existed.
func (w *legalWalk) autoPayPlan(p state.PlayerID, id state.ObjID, idx int, base Cost, scope costScope, statics costStaticViews) bool {
	return w.autoPayConfirm(p, id, base, scope, statics, w.e.planAbilityPayment(p, id, idx))
}

// autoPayPhyrexian is the autopay offer for a cost carrying Phyrexian pips
// (CR 107.4f: each is paid with one mana of its colour or 2 life). The
// planner prices a pip as neither (PlanManaHalf), so a witness over the raw
// cost taps nothing for a pip the life total cannot cover. Try each count k of
// life-paid pips the life total affords (most life first, so the least colour
// is demanded), plan the cost with the other pips as coloured pips, and let
// autoPayConfirm's offer gate -- which resolves the real mixed cost against the
// witness's pool -- accept it.
func (w *legalWalk) autoPayPhyrexian(p state.PlayerID, id state.ObjID, idx int, base Cost, scope costScope, statics costStaticViews) bool {
	k := len(base.Phyrexian)
	if life := w.e.G.Players[p].Life; life < 0 {
		k = 0
	} else if afford := int(life) / decision.PhyrexianLife; afford < k {
		k = afford
	}
	for ; k >= 0; k-- {
		if w.autoPayConfirm(p, id, base, scope, statics, w.e.planAbilityPaymentLifePips(p, id, idx, k)) {
			return true
		}
	}
	return w.autoPayScriptOffer(p, id, idx, scope)
}

// autoPayScriptOffer is autoPayOffer's last resort: the planner prices no
// witness, and its census cannot execute a mana-fee mana ability (Heap Gate's
// "{1}, {T}: Add one mana of any color" beside a floating {C}), so its
// "insufficient" is not the proof a complete census would make it. CR 601.2g
// lets the payer activate mana abilities while paying, and CR 605.3a funds a
// fee from an earlier same-window activation, so the route the potential
// surface's prefix search proves (potentialPrefixScript: the same census, the
// same relax and bounded proofs, the same node budget) is a legal payment. The
// play is offered on a found script; the payment falls to the ordinary manual
// window (armAbilityAutoPay's planner decline leaves it in place), which offers
// the route's abilities one step at a time, or to the planner's witness once
// the prefix has floated.
//
// The census and the search-slot count are shared across the walk's plays the
// way the potential query's are (autoBound, potentialScriptPlays): a slot is
// consumed when the census is incomplete and the play is not target-priced,
// whether or not the bounded skip and the search run below.
func (w *legalWalk) autoPayScriptOffer(p state.PlayerID, id state.ObjID, idx int, scope costScope) bool {
	e := w.e
	if w.autoCensus == nil {
		e.beginDerivedMemo()
		c := e.paymentPlanCensusOf(p, w.autoBound)
		e.endDerivedMemo()
		w.autoCensus = &c
	}
	census := *w.autoCensus
	if os.Getenv("GORGE_SPDBG") != "" && id == 14 {
		fmt.Fprintf(os.Stderr, "SPDBG id=14 complete=%v relaxable=%v gaps=%d searched=%d\n", census.complete, census.relaxable, len(census.gaps), w.autoScriptSearched)
	}
	if census.complete {
		return false
	}
	verdict := func() PaymentPlanOutcome { return e.planAbilityPayment(p, id, idx) }
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	if r := e.paymentPlanRelaxProof(census, verdict); census.relaxable && r.Reason == pay.ReasonInsufficient {
		if os.Getenv("GORGE_SPDBG") != "" && id == 14 {
			fmt.Fprintf(os.Stderr, "SPDBG id=14 relax proof insufficient\n")
		}
		return false
	}
	o := decision.Option{Kind: "ability", Obj: id, Ability: idx}
	if tp := e.potentialTargetPriced(p, o); w.autoScriptSearched >= potentialScriptPlays || tp {
		if os.Getenv("GORGE_SPDBG") != "" && id == 14 {
			fmt.Fprintf(os.Stderr, "SPDBG id=14 cap=%v targetPriced=%v\n", w.autoScriptSearched >= potentialScriptPlays, tp)
		}
		return false
	}
	w.autoScriptSearched++
	if !census.relaxable && census.bounded &&
		e.paymentPlanRelaxProof(census.boundedProof(), verdict).Reason == pay.ReasonInsufficient {
		if os.Getenv("GORGE_SPDBG") != "" && id == 14 {
			fmt.Fprintf(os.Stderr, "SPDBG id=14 bounded skip\n")
		}
		return false
	}
	gaps2 := e.scriptSpendableGaps(p, o, census.gaps)
	sr := e.newScriptSearch(p, decision.PotentialAction{Kind: "ability", Obj: id, Ability: idx}, 0, potentialScriptNodes)
	var ok bool
	if sr != nil {
		sr.gaps, sr.gapsOnly, sr.funding, sr.printedOnly = gaps2, true, census.fees(), true
		sr.passes = true
		_, ok = sr.run(e)
		if os.Getenv("GORGE_SPDBG") != "" && id == 14 {
			fmt.Fprintf(os.Stderr, "SPDBG id=14 inline search gaps=%d found=%v limited=%v approx=%v unproven=%v (direct=%v)\n",
				len(gaps2), ok, sr.limited, sr.approx, sr.unproven, func() bool { _, d := e.potentialPrefixScript(p, o, census.gaps, census.fees()); return d }())
		}
	}
	return ok
}

// autoPayConfirm runs the engine's own offer gate over a planner outcome's
// witness: the pool plus everything the plan taps.
func (w *legalWalk) autoPayConfirm(p state.PlayerID, id state.ObjID, base Cost, scope costScope, statics costStaticViews, out PaymentPlanOutcome) bool {
	if out.Plan == nil || out.Reason != "" || len(out.Plan.Activations) == 0 {
		return false
	}
	pool := w.e.G.Players[p].Pool
	for _, a := range out.Plan.Activations {
		for c, n := range a.Produces {
			pool[c] += int32(n)
		}
	}
	probe := base
	return w.e.offerCastableUsing(statics, p, id, &probe, scope, true, &pool)
}

// abilityCostNeedsMana reports whether c has a mana part a tap could pay.
func abilityCostNeedsMana(c Cost) bool {
	return c.Generic > 0 || c.Colored.Total() > 0
}

// armAbilityAutoPay gives the activation just begun (pc, the printed ability
// pile index idx of its source) the planner's witness when the floating pool
// cannot pay its mana: manaWindowAsk then taps the witness's sources one step
// at a time (revalidating each) instead of posing the manual window, exactly
// as a planned cast does. A shape the planner declines leaves the ordinary
// manual window in place.
func (e *Engine) armAbilityAutoPay(pc *pendingCast, idx int) {
	if pc == nil || pc.Payment != nil {
		return
	}
	// A TapCreaturesForMana ability (Heirloom Epic) lets the payer tap
	// creatures for part of the cost in an election that comes AFTER this
	// point, so the mana still owed is not known yet: a witness armed now
	// would carry a Cost the election then changes (cost_changed fallback).
	// Leave such an activation to the ordinary window; the offer still
	// prices the creature credit (planAbilityPayment, offerCastableUsing).
	if pay.TapCreaturesForMana(e.pcAbility(pc)) {
		return
	}
	mana := e.castPaymentMana(pc)
	if !abilityCostNeedsMana(mana) {
		return
	}
	if pay.CostPayableClassLife(asPayer(e), pc.player, paymentForCast(pc, mana),
		pipRider{AnyColor: pc.mayPlayIgnore, AnyType: pc.mayPlayIgnoreType}, mana, false) {
		return
	}
	out := e.planAbilityPayment(pc.player, pc.card, idx)
	// A search that reached its node limit still returns its best plan, which
	// a cast is offered and pays (planCastPaymentMemo); an activation arms it
	// the same way.
	if out.Plan == nil || out.Reason != "" && out.Reason != "search_limit" || len(out.Plan.Activations) == 0 {
		return
	}
	pc.Payment = &plannedCastPayment{ActionID: decision.OptAutoFill, Plan: decision.ClonePaymentPlan(*out.Plan)}
	pc.PaymentNext = 0
}

// autoPayArmedOnly reports whether pc's witness is one armAbilityAutoPay armed and
// no step of it has run: the seat activated the ability and chose no plan.
func (e *Engine) autoPayArmedOnly(pc *pendingCast) bool {
	return pc.isAbility() && !pc.Announced && pc.PaymentNext == 0 && pc.Payment.ActionID == decision.OptAutoFill
}

// rearmAutoPay plans again for a witness its announcements outdated, before any
// step ran: a target's reduction, a Phyrexian pip's answer, a tap or waterbend
// helper settle only after armAbilityAutoPay armed it. No seat selected that
// witness. A spell's plan is the seat's, but it can't price the seat's own later
// answers: a Phyrexian pip, which it counts as neither mana nor life (PlanManaHalf,
// CR 107.4f), and a tap for a convoke-family keyword granted only once the spell is
// cast (CR 702.51a). It reports whether pc holds a new witness.
func (e *Engine) rearmAutoPay(pc *pendingCast) bool {
	if pc.Announced || pc.PaymentNext != 0 {
		return false
	}
	if pc.isAbility() {
		if pc.Payment.ActionID != decision.OptAutoFill {
			return false
		}
	} else if pc.PayIdx == 0 && len(pc.Convoke) == 0 {
		return false
	}
	plan := e.announcedAutoFillPlan(pc, e.castPaymentMana(pc))
	if plan == nil {
		return false
	}
	pc.Payment = &plannedCastPayment{ActionID: pc.Payment.ActionID, Plan: *plan}
	e.PaymentStats.RecordRearm(*plan)
	return true
}
