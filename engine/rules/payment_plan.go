package rules

// This file deliberately contains no call to emit.  Payment plans are an
// offer-time witness; the planned executor lives in cast.go.

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/rules/pay"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// PlanCastPayment builds one V1 witness for an ordinary cast from the hand
// or, in a Commander game, from the command zone (the plain taxed CR 903.8
// cast; alternate-cost command-zone casts stay manual like every other Mode
// cast).  It does not change the game, log, pending decision, or RNG.  It runs its own
// candidate walk for this one cast (ValidateCastPayment, the audit tools and
// tests call it for a single cast); the offer builder, which plans every
// candidate of one decision, shares one walk and one cost-static collection
// across them through planCastPaymentChecked.
func (e *Engine) PlanCastPayment(p state.PlayerID, cast decision.PlannedCast) PaymentPlanOutcome {
	statics := costStaticSource{e: e}
	candidates := paymentCastCandidates{e: e, p: p}
	return e.planCastPaymentChecked(p, cast, &statics, &candidates)
}

// CastPaymentCost is the mana cost a plain cast of the hand card id by p
// would lock in now (CR 601.2f): the printed cost with its spell-ability
// extras, re-priced by every live cost static exactly as the planner prices
// it (planCastPaymentChecked). ok is false for an object that is not a card
// in p's hand. A pure read, for diagnostics (testkit/paymirror) that need
// to show a cast's price moved since a plan was witnessed.
func (e *Engine) CastPaymentCost(p state.PlayerID, id state.ObjID) (cost decision.PaymentCost, ok bool) {
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZHand || o.Owner != p || o.Face() == nil {
		return decision.PaymentCost{}, false
	}
	base := pay.WithSpellAbilityExtras(o.Face(), pay.RawBaseCost(asPayer(e), p, id))
	return pay.WireCost(e.offerCostFor(p, id, base, spellScope(""))), true
}

// MayPlayCastPaymentCost is CastPaymentCost for the plain cast an untyped
// may-play permission grants a card in exile or the graveyard (the "mayplay"
// cast mode): the printed cost with its spell-ability extras, the
// permission's free-cast read and RaiseCost$ surcharge composed exactly as
// beginCast's "mayplay" case charges them (CR 118.9, 118.3a), re-priced by
// every live cost static over the mayplay scope. The card may be another
// player's (a may-play permission can grant it), so there is no owner check,
// and perm names a MayPlayText$-typed permission's option, whose riders
// beginCast charges alone. ok is false for anything else. A pure read, for
// the oracle's convoke frame, which must filter a may-play cast's
// announcement by the same cost the payment will charge.
func (e *Engine) MayPlayCastPaymentCost(p state.PlayerID, id state.ObjID, perm string) (cost decision.PaymentCost, ok bool) {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || !plannedMayPlayZone(o.Zone) {
		return decision.PaymentCost{}, false
	}
	base := pay.RawBaseCost(asPayer(e), p, id)
	free, raise, hasRaise, priced := e.mayPlayPermFreeRaise(p, id, perm)
	if free {
		base = Cost{}
	}
	if hasRaise && priced {
		base = base.Plus(raise)
	}
	return pay.WireCost(e.offerCostFor(p, id, pay.WithSpellAbilityExtras(o.Face(), base), spellScope("mayplay"))), true
}

// planCastPaymentChecked is PlanCastPayment over caller-owned per-build
// inputs: statics is the build's one lazy cost-static collection and
// candidates its one lazy hypothetical candidate walk. Both are pure reads of
// the unchanged state, so sharing them across the candidates of one build
// answers every check exactly as a fresh collection and walk would. The
// check order, and therefore every Reason and Detail, is PlanCastPayment's.
func (e *Engine) planCastPaymentChecked(p state.PlayerID, cast decision.PlannedCast, statics *costStaticSource, candidates *paymentCastCandidates) PaymentPlanOutcome {
	// cast.Face names the face the cast begins as: a plain cast's is the face
	// the card shows. Every ordinary card shows face 0, but a Room that left
	// the battlefield as its other door keeps that door in every zone
	// (rooms.go), so its plain cast is the shown half's (CR 709.3: the caster
	// chooses which half they are casting). The object check below requires
	// cast.Face to be the object's current face, so a bogus value still fails
	// closed.
	// V1 plans the ordinary-cast origins: the hand (the original shape),
	// the command zone (the CR 903.8 plain taxed cast; alternate
	// command-zone casts -- dash, evoke, bestow, ... -- carry a Mode and are
	// withheld, as everywhere else), and the exile zone, graveyard and
	// library top, where only an untyped may-play permission's plain cast is
	// planned (the "mayplay" cast mode; a MayPlayText$-typed permission, and
	// every alternative cost, are withheld).  Any other origin fails closed,
	// and the object must actually sit in its origin's zone: a mislabeled
	// origin would compose a cost (and, from ZCommand, a commander tax) that
	// beginCast would never charge. The hand's alternate-spell-face casts
	// (adventure_alt, omen_alt) ride the same body under the flip to the
	// spell face (plannedAltSpellFace, faceprobe.go).
	originZone := state.ZHand
	flashback := false
	switch planCastPaymentCheckedCodes.Code(string(cast.Origin)) {
	case planCastPaymentCheckedHand:
	case planCastPaymentCheckedCommandZone:
		originZone = state.ZCommand
	case planCastPaymentCheckedExile:
		originZone = state.ZExile
	case planCastPaymentCheckedGraveyard:
		originZone = state.ZGraveyard
	case planCastPaymentCheckedLibrary:
		originZone = state.ZLibrary
	case planCastPaymentCheckedFlashback:
		// The CR 702.34a flashback cast of a graveyard card for its primary
		// flashback cost (decision.OriginFlashback): the owner's own card,
		// cast in Mode "flashback", never a may-play grant.
		originZone, flashback = state.ZGraveyard, true
	default:
		return PaymentPlanOutcome{Reason: "unsupported"}
	}
	mayPlay := plannedMayPlayZone(originZone) && !flashback
	o := e.G.Obj(cast.Object)
	// A may-play permission can grant an opponent's card (Intellect
	// Devourer), so the owner check is relaxed for those origins only; the
	// offer's own controller check already gates who may cast it.
	if o == nil || o.Zone != originZone || (!mayPlay && o.Owner != p) || o.Face() == nil {
		return PaymentPlanOutcome{Reason: "unsupported"}
	}
	if originZone == state.ZLibrary {
		// A may-play library cast begins only from the top card (the granting
		// permission covers Card.TopLibrary): anything deeper fails closed.
		lib := e.G.Zone(state.ZLibrary, o.Controller)
		if len(lib) == 0 || lib[0] != cast.Object {
			return PaymentPlanOutcome{Reason: "unsupported"}
		}
	}
	// A hand alternate-spell-face cast (adventure_alt/omen_alt, CR 715.3a,
	// 720.3) is priced the way beginCast charges it: one FlipFace to the
	// spell face BEFORE the ordinary cast transaction, so the whole body
	// below reads the FLIPPED face (faceprobe.go) -- base cost, altAdd parts,
	// cost statics, and the FaceIdx check (FaceIdx == cast.Face == 1 under
	// the flip, so a cast whose face function names nothing still fails
	// closed at the body's own check). The shared cost statics are
	// re-collected under the flip when either face carries a cost-modifier
	// static of its own, exactly the rule offerCastableAsFace
	// (legal_walk.go) implements.
	if altFace := plannedAltSpellFace(cast, o); altFace != nil {
		// The candidate walk's own offer gates read the object at its LIVE
		// face (appendAlternateSpellFace's FaceIdx-0 guard), so the set is
		// filled here, before the probe: a membership query inside the flip
		// would run a fresh walk that cannot re-list the alternate-face
		// option the live-face walk offered.
		if !candidates.has(cast.Object, cast.Mode) {
			return PaymentPlanOutcome{Reason: "unsupported"}
		}
		liveFace := o.Face()
		var out PaymentPlanOutcome
		e.offerAsFace(cast.Object, altFace, func() bool {
			src := statics
			if faceHasCostStatics(liveFace) || faceHasCostStatics(altFace) {
				// A fresh lazy source: the caller's may already have been
				// collected at the live face, and a get() here collects
				// under the flip (probe-local, retired at both edges).
				src = &costStaticSource{e: e}
			}
			out = planCastPaymentCheckedBody(&planCastBody{e: e}, p, cast, src, candidates, originZone, flashback, mayPlay, altFace)
			return true
		})
		return out
	}
	if int(o.FaceIdx) != cast.Face {
		return PaymentPlanOutcome{Reason: "unsupported"}
	}
	return planCastPaymentCheckedBody(&planCastBody{e: e}, p, cast, statics, candidates, originZone, flashback, mayPlay, nil)
}

// planCastBody carries the Engine the planner's plain-cast body prices
// against, so the body is a free function over a feature struct: an Engine
// method, or a free function taking *Engine, would grow the
// engineMethodCount/engineSurface ratchets' measurement.
type planCastBody struct {
	e *Engine
}

// planCastPaymentCheckedBody is planCastPaymentChecked's body, from the
// object check through the witness. altFace non-nil runs under the flip to
// that alternate spell face (faceprobe.go), with the composition in the cast
// mode's scope.
func planCastPaymentCheckedBody(b *planCastBody, p state.PlayerID, cast decision.PlannedCast, statics *costStaticSource, candidates *paymentCastCandidates, originZone state.Zone, flashback, mayPlay bool, altFace *cards.Face) PaymentPlanOutcome {
	e := b.e
	targetPaid := false
	o := e.G.Obj(cast.Object)
	// A may-play permission can grant an opponent's card (Intellect
	// Devourer), so the owner check is relaxed for those origins only; the
	// offer's own controller check already gates who may cast it.
	if o == nil || o.Zone != originZone || (!mayPlay && o.Owner != p) || o.Face() == nil || int(o.FaceIdx) != cast.Face {
		return PaymentPlanOutcome{Reason: "unsupported"}
	}
	scope := plannedCastScope(o.Zone, flashback)
	if altFace != nil {
		// The alternate spell face's charge is composed in the cast mode's
		// scope, exactly beginCast's (pendingCastScope's spellScope(pc.mode),
		// cast_pricing.go) -- never plannedCastScope's empty hand scope.
		scope = spellScope(cast.Mode)
	}
	altAdd := len(altAddCostParts(o.Face())) != 0
	beholdAltAdd := false
	if altAdd {
		// The beholder-or-mana shape (Mudbutton Cursetosser's
		// Behold<1/Goblin>:2) is the one the planned witness prices: the
		// Behold part is paid by the in-flow ask, and the mana part by the
		// witness. Other either-or shapes stay with the offer gate.
		for _, part := range altAddCostParts(o.Face()) {
			if len(ParseCost(part).Behold) != 0 {
				beholdAltAdd = true
				break
			}
		}
	}
	planAltAdd := false
	detail := e.paymentPlanCastShapeDetailScoped(statics.get(), p, cast.Object, scope)
	if detail != "" {
		switch {
		case altFace != nil:
			// The alternate-spell-face body prices the flipped face's printed
			// cost alone, exactly the offer walk's base for the mode
			// (appendAlternateSpellFace's ParseCost(af.ManaCost)): a shape the
			// plain body special-cases -- a target-paid price, an either-or
			// additional cost -- stays with the manual window, as the
			// potential planner keeps it (potentialModeCastPlan composes a
			// Mode cast's printed cost alone).
			return PaymentPlanOutcome{Reason: "unsupported", Detail: detail}
		case targetPaidShape(detail) && !flashback && !mayPlay && e.targetPaidNeedsPlannedPayment(p, cast.Object):
			// A witness whose target pays for it is the target-paid path's
			// (see targetPaidNeedsPlannedPayment); keep it first.
			targetPaid = true
		case beholdAltAdd && detail == "shape:optional_cost" && !flashback &&
			e.paymentPlanCastShapeDetailAltAdd(statics.get(), p, cast.Object, scope, true) == "":
			// A mandatory either-or additional cost (CR 601.2b/h) is a
			// choice made at announcement, so the printed-cost witness
			// alone cannot price it: the cast flow asks altAddAsk and folds
			// the chosen part into the total. Plan that total here instead
			// of withholding the cast (the autopay surface only offers
			// plain casts the planner witnesses).
			planAltAdd = true
		default:
			return PaymentPlanOutcome{Reason: "unsupported", Detail: detail}
		}
	}
	switch {
	case flashback:
		if !candidates.hasFlashback(cast.Object) {
			return PaymentPlanOutcome{Reason: "unsupported"}
		}
	case mayPlay:
		if !candidates.has(cast.Object, "mayplay") {
			return PaymentPlanOutcome{Reason: "unsupported"}
		}
	case altFace != nil:
		if !candidates.has(cast.Object, cast.Mode) {
			return PaymentPlanOutcome{Reason: "unsupported"}
		}
	default:
		if !candidates.has(cast.Object) {
			return PaymentPlanOutcome{Reason: "unsupported"}
		}
	}
	if targetPaid && detail == "shape:target_dependent_cost" {
		// A ValidTarget$ cost reduction prices the cast for the targets it
		// will have (CR 601.2f), and a cast that is payable only by spending
		// one of those targets pays at CR 601.2g, after they are chosen
		// (targetPaidNeedsPlannedPayment). Plan the cheapest legal target
		// set's composed cost, the same enumeration the potential walk's
		// witness uses (potentialTargetPricedPlan), so the witness's price is
		// the one the chosen set will charge.
		base := pay.RawBaseCost(asPayer(e), p, cast.Object)
		base = pay.WithSpellAbilityExtras(o.Face(), base)
		rider := pay.ConvRider(asEval(e).Conv(p, cast.Object, pay.SASpell))
		if _, got := e.targetPricedPlan(statics.get(), p, cast.Object, base, spellScope(""), rider); got.Plan != nil && got.Reason == "" {
			return got
		}
	}
	// V1 has no way to carry a target-dependent reprice or a choice made at
	// announcement. Candidate discovery below owns timing, targets and
	// prohibitions; this method owns the exact cost/witness subset.
	// base is the cast's price BEFORE the cost statics (the composition the
	// either-or additional cost's part joins, CR 601.2f/h).
	var base, cost Cost
	var improviseTaps []state.ObjID
	if flashback {
		cost = e.plannedFlashbackCost(statics.get(), p, cast.Object)
	} else if mayPlay {
		// The untyped may-play cast's price, composed exactly as the offer
		// walk (legal_walk_alt.go) prices it and beginCast's "mayplay" case
		// charges it: the free grant empties the mana part, the granting
		// static's RaiseCost$ is added before the cost statics apply, and
		// the spell ability's extras follow in "mayplay" scope.
		base = pay.RawBaseCost(asPayer(e), p, cast.Object)
		if e.mayPlayFreeCast(p, cast.Object) {
			base = Cost{}
		}
		if raise, hasRaise, priced := e.mayPlayRaiseCost(p, cast.Object); hasRaise {
			if !priced {
				return PaymentPlanOutcome{Reason: "unsupported", Detail: "mayplay_raise"}
			}
			base = base.Plus(raise)
		}
		cost = pay.WithSpellAbilityExtras(o.Face(), e.offerCostForUsing(statics.get(), p, cast.Object, base, spellScope("mayplay")))
	} else {
		base = pay.RawBaseCost(asPayer(e), p, cast.Object)
		// CR 702.126a: the artifacts Improvise taps for generic mana are
		// credited before the statics, as castOfferBase prices the offer, and
		// stay out of the mana plan (one permanent pays one cost).
		base, improviseTaps = e.improviseCost(p, cast.Object, base, nil)
		base = pay.WithSpellAbilityExtras(o.Face(), base)
		cost = e.offerCostForUsing(statics.get(), p, cast.Object, base, scope)
	}
	if targetPaid {
		cost = pay.PlanTargetPaidManaHalf(cost)
	}
	if detail := pay.PlanNonManaAdmissible(cost); detail != "" {
		return PaymentPlanOutcome{Reason: "unsupported", Detail: detail}
	}
	if global, detail := e.paymentPlanGlobalManaEffect(p, cast.Object); global {
		return PaymentPlanOutcome{Reason: "unsupported", Detail: detail}
	}
	if planAltAdd {
		return e.planCastAltAdd(statics.get(), p, cast, base, mayPlay)
	}
	desc := pay.DescriptorFor(cast.Object, false, cost)
	mana := pay.PlanManaHalf(cost)
	// A creature spell under Vizier of the Menagerie's AnyType->AnyType
	// conversion funds its pips as generic, as the may-play rider does (CR 609.4b).
	rider := pay.ConvRider(asEval(e).Conv(p, cast.Object, pay.SASpell))
	if mayPlay {
		rider = rider.Or(asPayer(e).Eval().MayPlayRider(p, cast.Object))
	}
	if len(improviseTaps) != 0 {
		// The Improvise credit is composed only in the plain-cast branch above,
		// so this arm never runs under a may-play origin: its rider is the zero
		// rider or a ConvRider AnyType alone, and both folds are exact -- a zero
		// rider folds nothing, and "any type" lets even colourless mana pay
		// every pip (so the folded witness's production always settles the
		// pre-fold cost). No folded/re-plan shape or coloured-unit prune is
		// needed here: the arm cannot carry an AnyColor rider, whose fold
		// erases a pip's need for a coloured unit.
		mana = pay.PlanManaUnderRider(mana, rider)
		return planImprovise(asPayer(e), p, cast, mana, &desc, improviseTaps)
	}
	plan := func(c Cost, d *pay.Descriptor) PaymentPlanOutcome {
		if len(cost.Sac) == 0 && len(cost.TapPermanent) == 0 {
			return pay.PlanPaymentCost(asPayer(e), p, cast, c, d)
		}
		// The mana abilities run first (CR 601.2g-h), so a source that
		// sacrifices itself can use up what an additional cost must sacrifice.
		parts, _ := planCostParts(cost, cast.Object, false)
		return e.planReserving(p, cast.Object, cost, false, false, parts, func(tapR, keptR []state.ObjID) PaymentPlanOutcome {
			return pay.PlanPaymentCostWithout(asPayer(e), p, cast, c, d, tapR, nil, keptR)
		})
	}
	out := planManaUnderRider(asPayer(e), p, e.G.Players[p].Life, cast, mana, rider, plan)
	if len(cost.Phyrexian) != 0 {
		// The witness funds only the pip-free mana half (pay.PlanManaHalf),
		// so a Phyrexian pip the payer's life total cannot pay must take its
		// colour face out of the window. A plan whose window cannot settle it
		// strands the cast; it is not an executable payment (CR 107.4f,
		// 119.4).
		pc := &pendingCast{player: p, card: cast.Object, mode: cast.CastMode(), ability: -1, cost: cost}
		desc := paymentForCast(pc, cost)
		av := pay.AvailableFor(asPayer(e), p, desc)
		conv := asEval(e).Conv(p, cast.Object, desc.SAKind())
		half := out
		out = castPlanPipsOutcome(e, p, cost, pc, e.G.Players[p].Life, e.G.Players[p].Snow,
			av.Pool, av.Typed, conv, rider, out)
		if out.Plan == nil {
			out = castPlanPhyrexianReplan(asPayer(e), e, p, cast, cost, mana, rider, plan, half, out, pc, av, conv)
		}
	}
	return out
}

// castPlanPhyrexianReplans bounds the re-plans of castPlanPhyrexianReplan:
// each attempt holds at least one more of the previous witness's sources out
// of the plan, so a cost the pip's colour faces cannot survive without stops
// the loop long before it.
const castPlanPhyrexianReplans = 4

// castPlanReplan is the narrow surface the re-plan needs: the cast window's
// own read beside the reserving planner (planReserving). The interface keeps
// the re-plan off the engine's own surface (the engineSurface ratchet), the
// way castPlanPipsWindow does for castPlanPipsOutcome.
type castPlanReplan interface {
	castPlanPipsWindow
	planReserving(p state.PlayerID, id state.ObjID, cost Cost, ability, selfSource bool,
		parts []planCostPart, plan func(tapR, keptR []state.ObjID) PaymentPlanOutcome) PaymentPlanOutcome
}

// castPlanPhyrexianReplan re-plans a withheld Phyrexian cast's mana half with
// the sources its witness tapped held out of the plan, and re-runs the CR
// 601.2g window check on every re-plan (CR 107.4f, 119.4). The half's witness
// prices the pip-free mana half (pay.PlanManaHalf) before any pip is answered
// (CR 601.2b), and the window the pips then resolve in offers every source the
// witness did not tap (castPlanPipsOutcome): a witness that spent a source the
// pips' colour faces needed withholds a cast a different assignment pays --
// Fetid Heath's free "{T}: Add {C}" feeding the generic while its "{W/B},
// {T}: Add {W}{W}, {W}{B} or {B}{B}" fee ability was the only {B} the window
// could make (Vault Skirge's {1}{B/P} at one life beside an untapped Fetid
// Heath and a Forest). The re-plan prices the same half over the other
// sources, leaving the pip's colour producers to the window. half is the
// withheld attempt's own half plan; an attempt with no plan stops the loop:
// the sources excluded so far are ones every remaining assignment must do
// without, so the pips they were held out for cannot be paid beside the half.
func castPlanPhyrexianReplan(pe pay.Engine, w castPlanReplan, p state.PlayerID, cast decision.PlannedCast,
	cost, mana Cost, rider pipRider, plan func(Cost, *pay.Descriptor) PaymentPlanOutcome,
	half, out PaymentPlanOutcome, pc *pendingCast, av pay.Available, conv *manaConv) PaymentPlanOutcome {
	if half.Plan == nil {
		return out
	}
	life := w.Game().Players[p].Life
	replan := func(tapR []state.ObjID) func(Cost, *pay.Descriptor) PaymentPlanOutcome {
		return func(c Cost, d *pay.Descriptor) PaymentPlanOutcome {
			if len(cost.Sac) == 0 && len(cost.TapPermanent) == 0 {
				return pay.PlanPaymentCostWithout(pe, p, cast, c, d, tapR, nil, nil)
			}
			// The mana abilities run first (CR 601.2g-h), so a source that
			// sacrifices itself can use up what an additional cost must
			// sacrifice.
			parts, _ := planCostParts(cost, cast.Object, false)
			return w.planReserving(p, cast.Object, cost, false, false, parts, func(tr, kr []state.ObjID) PaymentPlanOutcome {
				all := tr
				if len(tapR) != 0 {
					all = append(append([]state.ObjID(nil), tr...), tapR...)
				}
				return pay.PlanPaymentCostWithout(pe, p, cast, c, d, all, nil, kr)
			})
		}
	}
	excluded := make([]state.ObjID, 0, len(half.Plan.Activations))
	for _, a := range half.Plan.Activations {
		excluded = append(excluded, a.Source)
	}
	for attempt := 0; attempt < castPlanPhyrexianReplans; attempt++ {
		got := planManaUnderRider(pe, p, life, cast, mana, rider, replan(excluded))
		if got.Plan == nil {
			return out // no assignment without the excluded sources pays the half
		}
		retry := castPlanPipsOutcome(w, p, cost, pc, life, w.Game().Players[p].Snow,
			av.Pool, av.Typed, conv, rider, got)
		if retry.Plan != nil {
			return retry
		}
		for _, a := range got.Plan.Activations {
			seen := false
			for _, id := range excluded {
				seen = seen || id == a.Source
			}
			if !seen {
				excluded = append(excluded, a.Source)
			}
		}
	}
	return out
}

// planManaUnderRider plans a cast's mana half under a may-play pip rider and
// keeps the rider's relaxed coloured pips funded from a coloured unit. The
// folded witness (pay.PlanManaUnderRider) prices every plain coloured pip
// generic, so the plan may tap a colourless-only ability; but "any color"
// never includes colourless (CR 107.4c), and the real payment then cannot
// settle the pip -- the cast reverts with the card still in its old zone and
// the pool holding the stranded {C}. If the folded plan's production cannot pay
// the pre-fold cost under the rider, re-plan the pinned alternatives
// (potentialPinnedColourPlan's shape) and return the first that does. A folded
// plan with no witness (insufficient) is returned unchanged: pins never make a
// cost cheaper, so there is nothing to re-plan. Otherwise the folded plan is
// never returned -- it would validate and then fail at execution -- so the
// verdict is unsupported/witness_not_offered, never insufficient, which
// provesUnpayable reads as a proof.
func planManaUnderRider(pe pay.Engine, p state.PlayerID, life int32, cast decision.PlannedCast, mana Cost, rider pipRider,
	plan func(Cost, *pay.Descriptor) PaymentPlanOutcome) PaymentPlanOutcome {
	real := pay.PlanManaHalf(mana)
	alts, n := pay.PlanManaUnderRiderAlternatives(real, rider)
	desc := pay.DescriptorFor(cast.Object, false, alts[0])
	got := plan(alts[0], &desc)
	// The folded plan's own search may have collected alternative witnesses
	// (pay.Session.PlanAlts, EnsurePaymentPlanAlternatives's readers) priced at
	// the same folded cost; the folded pip is generic to them too, so an
	// alternative that funds it from a colourless-only ability would validate
	// and then strand it -- prune those like plan 0's own verdict does.
	prunePlanAltsUnderRider(pe, p, life, real, rider, &desc)
	if got.Plan == nil {
		return got
	}
	if planPaysManaUnderRider(pe, p, life, *got.Plan, &desc, real, rider) {
		return got
	}
	for i := 1; i < n; i++ {
		desc := pay.DescriptorFor(cast.Object, false, alts[i])
		retry := plan(alts[i], &desc)
		if retry.Plan != nil && retry.Reason == "" &&
			planPaysManaUnderRider(pe, p, life, *retry.Plan, &desc, real, rider) {
			return retry
		}
	}
	// A board whose coloured units span several colours funds the rider's
	// pips with a coloured unit of ANY colour, so no single-colour pin has to
	// cover them all (four {W/U} pips under MayPlayIgnoreColor$ beside
	// U1 B3 R1: every pin asks four units of one colour, three is the
	// largest, yet Island U, two Swamps B and a Mountain R pay the cast).
	// One more alternative spreads the pips over the colours the payment
	// census can fund, largest supply first in WUBRG tie-break order; the
	// witness verifies the plan against the real rider cost exactly as it
	// verifies every pin, so a spread that over-asks one colour only costs a
	// failed attempt and the cast stays withheld (CR 107.4c: colourless mana
	// still pays no pip).
	if spread := pay.PlanManaUnderRiderSpread(pe, p, real, rider); spread != nil {
		desc := pay.DescriptorFor(cast.Object, false, *spread)
		retry := plan(*spread, &desc)
		if retry.Plan != nil && retry.Reason == "" &&
			planPaysManaUnderRider(pe, p, life, *retry.Plan, &desc, real, rider) {
			return retry
		}
	}
	return PaymentPlanOutcome{Reason: "unsupported", Detail: pay.DetailWitnessNotOffered}
}

// planPaysManaUnderRider reports whether plan's activations plus the starting
// pool can settle real under rider, exactly as ValidateCastPayment settles a
// matched alternative: the plan prices the folded (or pinned) cost, and the
// real payment spends the produced units under the rider.
func planPaysManaUnderRider(pe pay.Engine, p state.PlayerID, life int32, plan decision.PaymentPlan, desc *pay.Descriptor, real Cost, rider pipRider) bool {
	var produced state.Mana
	for _, a := range plan.Activations {
		for i := range produced {
			produced[i] += int32(a.Produces[i])
		}
	}
	pool := pay.ManaAdd(pay.PlanPool(pe, p, desc), produced)
	_, ok := resolveManaWith(real, pool, state.Mana{}, [8]state.Mana{}, life, false, rider, nil)
	return ok
}

// castPlanPipsWindow is the cast payment window's reach: the only engine
// surface castPlanPipsOutcome needs. The narrow interface keeps the check
// from widening *Engine (the engineSurface ratchet).
type castPlanPipsWindow interface {
	castWindowUnits(pc *pendingCast) []windowManaUnit
	castWindowCensusComplete(pc *pendingCast, units []windowManaUnit) bool
	castWindowReachableAfter(deadAfter int, feeExtra state.Mana, p state.PlayerID, cost Cost, spellPool, snow state.Mana,
		typed [8]state.Mana, life int32, conv *manaConv, rider pipRider, units []windowManaUnit) (bool, int)
	// The ceiling read's queries (existing *Engine methods): the interface
	// keeps the pips check off the engine's own surface.
	Game() *state.Game
	untappedManaSource(p state.PlayerID, id state.ObjID) bool
	paymentPlanManaInterference() bool
	manaProductionAltered(id state.ObjID, ma *cards.SA) bool
	convokeCommitted(pc *pendingCast, id state.ObjID) bool
	castWindowProbeAbilities(p state.PlayerID, id state.ObjID) []*cards.SA
	manaAbilityCC(p state.PlayerID, id state.ObjID, ma *cards.SA) *compiledCost
	paidCostShape(p state.PlayerID, id state.ObjID, cost Cost, life int32) (int32, bool)
	castWindowSourceExcluded(p state.PlayerID, id state.ObjID, desc pay.Descriptor) bool
	castWindowAmount(p state.PlayerID, source state.ObjID, o *state.Object, ma *cards.SA) (int32, bool)
	castWindowIdentityCounts(p state.PlayerID) (counts [6]int32)
	castWindowRestrictedAdmits(p state.PlayerID, id state.ObjID, desc pay.Descriptor, units []windowManaUnit) bool
}

// castPlanPipsOutcome withholds a cast plan whose Phyrexian pips the payer's
// life total cannot all pay. The witness funds only the pip-free mana half
// (pay.PlanManaHalf); each pip then takes its colour face or its two life in
// the CR 601.2g-h window (CR 107.4f, 119.4). At a life total below two per
// pip some pip must take its colour face, which needs mana the half's witness
// did not fund, so the window's own reach decides whether the plan can still
// settle it.
//
// The plan's activations tap their sources and float their mana, and the
// window then offers every remaining source (castWindowUnits). The check
// starts from the plan's production and omits the sources it already used
// (a source the activation sacrificed is gone; one that survives can still
// take its no-tap self-sacrifice second activation, castWindowUsedExtras),
// exactly as the lowering executes. An incomplete census cannot prove the
// play unpayable, so it keeps the plan (fail open) -- unless even the ceiling
// of what its unpriced sources could add falls short of the cost's floor at
// this life total (castWindowUnpricedReach, castPipsManaFloor).
func castPlanPipsOutcome(w castPlanPipsWindow, p state.PlayerID, cost Cost, pc *pendingCast,
	life int32, snow, pool state.Mana, typed [8]state.Mana, conv *manaConv, rider pipRider, out PaymentPlanOutcome) PaymentPlanOutcome {
	if out.Plan == nil || out.Reason != "" || len(cost.Phyrexian) == 0 ||
		life >= int32(decision.PhyrexianLife)*int32(len(cost.Phyrexian)) {
		return out
	}
	units := w.castWindowUnits(pc)
	complete := w.castWindowCensusComplete(pc, units)
	used := make(map[state.ObjID]bool, len(out.Plan.Activations))
	sacrificed := make(map[state.ObjID]bool, len(out.Plan.Activations))
	var produced state.Mana
	for _, a := range out.Plan.Activations {
		used[a.Source] = true
		if a.Consequence != nil && a.Consequence.Sacrifice {
			sacrificed[a.Source] = true
		}
		for i := range produced {
			produced[i] += int32(a.Produces[i])
		}
	}
	var unpriced int32
	var bounded bool
	if !complete {
		unpriced, bounded = castWindowUnpricedReach(w, pc, units, used, sacrificed)
	}
	kept := make([]windowManaUnit, 0, len(units))
	for _, u := range units {
		if !used[u.ID] {
			kept = append(kept, u)
		}
	}
	if !complete {
		if bounded {
			if floor := castPipsManaFloor(&cost, life); floor >= 0 {
				// Snow and typed are PROVENANCE, not additional mana: they
				// partition the same floating pool Pool already carries.
				have := int64(pool.Total()+produced.Total()) + int64(castWindowUnitsReach(kept)) +
					int64(castWindowUsedExtras(units, used, sacrificed)) + int64(unpriced)
				if have < floor {
					return PaymentPlanOutcome{Reason: "insufficient", Detail: "phyrexian_window"}
				}
			}
		}
		return out
	}
	// The plan's production floats in the payer's pool before this window
	// opens (the witness is lowered at priority), so it funds an activation
	// fee beside the payer's real pool: a Selesnya Signet's "{1}: Add {W}{G}"
	// is payable from it (CR 601.2g-h), and a pip's colour face the fee's
	// production then settles keeps the cast offered (dr-mbc-msd pair 55,
	// Moltensteel Dragon's two {R/P} at 3 life).
	if ok, _ := w.castWindowReachableAfter(cwDeadAfter, produced, p, cost, pay.ManaAdd(pool, produced), snow, typed, life, conv, rider, kept); ok {
		return out
	}
	return PaymentPlanOutcome{Reason: "insufficient", Detail: "phyrexian_window"}
}

// castWindowUsedExtras is the ceiling's second-activation term for a source
// the plan already activated: one the activation left on the battlefield can
// still take the no-tap self-sacrifice activation the window records as an
// Extra (CR 602.2b, 605.1a; castWindowUnitsReach counts a kept unit's Extras).
// Omitting them under-counts a window the plan's own sources can still pay,
// which withholds a payable cast; one the activation sacrificed is gone.
func castWindowUsedExtras(units []windowManaUnit, used, sacrificed map[state.ObjID]bool) int32 {
	total := int32(0)
	for _, u := range units {
		if !used[u.ID] || sacrificed[u.ID] {
			continue
		}
		for _, a := range u.Extras {
			total += max(0, a.Mana().Total())
		}
	}
	return total
}

// castPipsManaFloor is the least mana a payment of cost can spend at life:
// its pool-unit floor (generic and strict coloured pips, black excluded
// because PayLifeInsteadOf:B may pay it), plus one unit for each Phyrexian pip
// the life total cannot pay with two life (CR 107.4f, 119.4). A
// hybrid-Phyrexian pip stays out: pay.PlanManaHalf keeps it in the witness
// (only cost.Phyrexian is stripped), so the plan already funds either colour
// face and never leaves it for this window. Snow, hybrid and twobrid pips also
// stay out, as in the pool-unit floor: each would add at least one unit, so
// leaving them out keeps this a floor, never an over-estimate. A cost whose
// {X}, repeated (NumTimes) or Unknown parts could add mana has no floor (-1).
func castPipsManaFloor(c *Cost, life int32) int64 {
	if c.X != 0 || c.NumTimes || len(c.Unknown) != 0 {
		return -1
	}
	phy := int64(len(c.Phyrexian))
	return c.PoolUnitsFloor() + max(0, phy-int64(life/2))
}

// prunePlanAltsUnderRider drops the alternative witnesses the folded plan's
// search collected (pay.Session.PlanAlts, priced at the folded cost and
// appended by EnsurePaymentPlanAlternatives) whose production cannot settle the
// pre-fold cost under the rider. ValidateCastPayment settles a folded-cost
// witness against the folded cost with plain pipRider{} -- the folded pips are
// generic to it -- so an unpruned alternative that funds the folded pip from a
// colourless-only ability validates and then strands the pip's {C} in the pool:
// the same defect the folded/re-plan verdict above keeps off plan 0. A pinned
// alternative is priced at a pinned cost whose coloured pips settle directly,
// identical under both settles, so the same rule keeps it. Nothing to do
// without a rider (the fold is then the identity, and every alternative
// settles its own cost, which is real's) or without collected alternatives
// (PlanAltsWant is one outside EnsurePaymentPlanAlternatives).
func prunePlanAltsUnderRider(pe pay.Engine, p state.PlayerID, life int32, real Cost, rider pipRider, desc *pay.Descriptor) {
	if rider == (pipRider{}) {
		return
	}
	s := pe.Session()
	if s == nil || len(s.PlanAlts) == 0 {
		return
	}
	kept := s.PlanAlts[:0:0]
	for _, alt := range s.PlanAlts {
		if planPaysManaUnderRider(pe, p, life, alt, desc, real, rider) {
			kept = append(kept, alt)
		}
	}
	s.PlanAlts = kept
}

// planCastAltAdd prices a plain cast's mandatory either-or additional cost
// (AlternateAdditionalCost, CR 601.2b/h) into the payment witness. altAddAsk
// runs at announcement, before this witness's mana window, and folds ONE
// chosen part into the total; the witness must therefore fund the printed
// cost plus the part the flow will ask for. altAddAsk offers the payable
// parts first in script order, so the witness is built for the first part
// that is payable: a mana part plans its total, a pure non-mana part (Behold,
// Blight) leaves the printed cost and just has to be payable now. The plan's
// Cost carries the selected part, which ValidateCastPayment re-derives.
func (e *Engine) planCastAltAdd(statics costStaticViews, p state.PlayerID, cast decision.PlannedCast, base Cost, mayPlay bool) PaymentPlanOutcome {
	o := e.G.Obj(cast.Object)
	if o == nil || o.Face() == nil {
		return PaymentPlanOutcome{Reason: "unsupported"}
	}
	scope := spellScope("")
	if mayPlay {
		scope = spellScope("mayplay")
	}
	rider := pay.ConvRider(asEval(e).Conv(p, cast.Object, pay.SASpell))
	if mayPlay {
		rider = rider.Or(asPayer(e).Eval().MayPlayRider(p, cast.Object))
	}
	opt := decision.Option{Kind: "cast", Obj: cast.Object, Mode: cast.CastMode()}
	for _, part := range altAddCostParts(o.Face()) {
		c := ParseCost(part)
		nonMana := !c.HasManaPayment()
		total := base.Plus(c)
		if nonMana && !pay.NonManaCastable(asPayer(e), p, cast.Object, total, false, "") {
			continue
		}
		// The part joins the price BEFORE the cost statics (CR 601.2f/h);
		// the spell ability's extras follow the composition in "mayplay"
		// scope, exactly as the offer walk and beginCast charge them.
		composed := e.offerCostForUsing(statics, p, cast.Object, total, scope)
		if mayPlay {
			composed = pay.WithSpellAbilityExtras(o.Face(), composed)
		}
		got := planManaUnderRider(asPayer(e), p, e.G.Players[p].Life, cast, composed, rider, func(c Cost, _ *pay.Descriptor) PaymentPlanOutcome {
			return e.planComposedCostFor(p, cast.Object, cast, c, false, false)
		})
		// A part with no mana of its own is paid by the cast flow's in-flow
		// ask (altAddAsk), not by the offer walk, so the walk cannot price it
		// on its own: a Behold part makes offerCastable decline the cast
		// however payable the beholding is. Requiring potentialWitnessOffers
		// here therefore dropped the cast whenever this free part was the
		// first payable one (pair 73: Silvergill Mentor's behold part was
		// payable while the {2} part was not, so neither the pool's option
		// list nor the planner carried the cast). NonManaCastable above is the
		// free part's own payability gate; the parts with mana still take the
		// offer gate. CR 601.2b/h.
		//
		// A relaxed proof run (PotentialPaymentPlans' unpayability proof) reads
		// only whether the part is payable with its uncovered sources taken as
		// free units, and its plans are never executed: a relaxed unit produces
		// nothing in the plan's activations, so the offer gate below would
		// price the witness short of mana the unit stands for and the proof
		// would call a Springleaf Drum-funded cast unpayable, never searching
		// the prefix that pays it. The gate is skipped there as the single-cost
		// arm's is (potentialModeCastPlan).
		if got.Plan != nil && got.Reason == "" && (nonMana || len(e.PaymentPlanRelaxed) != 0 || e.potentialWitnessOffers(p, opt, *got.Plan)) {
			return got
		}
	}
	return PaymentPlanOutcome{Reason: "insufficient"}
}

// planCastAltAddSettleCost folds the either-or part the witness planned into
// ValidateCastPayment's settle price. planCastAltAdd builds the witness's
// Cost from the chosen part's composed total, so the settle price is that
// Cost, rebuilt as the same mana vector.
func (e *Engine) planCastAltAddSettleCost(p state.PlayerID, cast decision.PlannedCast, cost Cost, plan decision.PaymentPlan) Cost {
	o := e.G.Obj(cast.Object)
	if o == nil || o.Face() == nil || len(altAddCostParts(o.Face())) == 0 {
		return cost
	}
	var colored state.Mana
	for i := range colored {
		colored[i] = int32(plan.Cost.Mana[i])
	}
	return Cost{Generic: int32(plan.Cost.Generic), Colored: colored}
}

// paymentPlanCastCandidate deliberately delegates timing, mandatory-target
// feasibility and CantBeCast to the normal hypothetical cast walk.  The
// hypothetical aggregate only discovers candidates; exact admission still
// happens through the source-exclusive plan below.
func (e *Engine) paymentPlanCastCandidate(p state.PlayerID, id state.ObjID) bool {
	c := paymentCastCandidates{e: e, p: p}
	return c.has(id)
}

// paymentCastCandidates is one player's plain-cast candidate set from ONE
// hypothetical legal-action walk, run on the first membership query.
// Candidate legality is intentionally independent of present mana: a large
// local pool lets the shared walk retain a cast which the planner will later
// classify as insufficient, while its non-mana gates (timing, mandatory
// targets, CantBeCast) remain authoritative and live. A member is a
// "cast" option with no Mode and no AltCostIndex -- exactly the option
// paymentPlanCastCandidate has always matched -- so membership answers that
// per-cast walk for every candidate of a build at the cost of one walk.
//
// priced marks a set whose every query names a plain cast the offer
// builder's OWN priced walk (legalActionsPriced at PotentialMana) listed.
// That walk and the huge-pool walk run the identical body; the one place
// the pool enters a plain cast's admission is offerCastable's mana
// feasibility, which only ever asks whether the pool COVERS a composed
// cost, and the huge pool covers every cost the PotentialMana bound does.
// (The two retry arms behind a failed first pass differ only for a
// ValidTarget$ cost static or an announced Sac<X>, and planCastPaymentChecked
// has already declined both shapes -- shape:target_dependent_cost,
// shape:additional_cost -- before it asks.) So membership is certain and
// the huge walk is skipped; pricedCandidatesVerify runs it anyway and
// panics on a miss.
type paymentCastCandidates struct {
	e      *Engine
	p      state.PlayerID
	ids    []state.ObjID
	ready  bool
	priced bool
	// mayIDs are the untyped may-play casts (Mode "mayplay", no
	// MayPlayPerm) the same walk listed, for the exile/graveyard origins.
	mayIDs []state.ObjID
	// fbIDs are the primary-instance flashback casts (Mode "flashback", no
	// Option.Cost) the same walk listed, for decision.OriginFlashback.
	fbIDs []state.ObjID
	// altIDs are the hand's alternate-spell-face casts (Mode
	// "adventure_alt"/"omen_alt") the same walk listed, for the planner's
	// alternate-face arm.
	altIDs []altCastCandidate
}

// altCastCandidate is one alternate-spell-face cast the candidate walk
// listed: the object and the mode it is cast in.
type altCastCandidate struct {
	id   state.ObjID
	mode string
}

// pricedCandidatesVerify: see derivedMemoVerify. Set by the rules test binary.
var pricedCandidatesVerify = derivedMemoVerifyFlag != ""

// has is the membership query for one planned cast shape the same walk
// listed: no mode names the plain casts (ids), "mayplay" the untyped may-play
// casts (mayIDs), and a hand alternate-spell-face mode (adventure_alt,
// omen_alt) the (id, mode) alternate-face pairs (altIDs). A priced set
// answers true, exactly as before, and verify mode checks it.
func (c *paymentCastCandidates) has(id state.ObjID, mode ...string) bool {
	if c.priced && !pricedCandidatesVerify {
		return true
	}
	c.fill()
	m := ""
	if len(mode) != 0 {
		m = mode[0]
	}
	kind := "plain cast"
	var found bool
	switch {
	case m == "":
		found = slices.Contains(c.ids, id)
	case planCastPaymentCheckedCodes.Code(m) == planCastPaymentCheckedMayPlayMode:
		kind, found = "may-play cast", slices.Contains(c.mayIDs, id)
	case plannedAltFaceMode(m):
		kind = "alternate-face cast"
		for _, cand := range c.altIDs {
			if cand.id == id && cand.mode == m {
				found = true
				break
			}
		}
	default:
		kind = "unknown-mode cast"
	}
	if c.priced && !found {
		panic(fmt.Sprintf("payment plan: priced-walk %s %d missing from the huge-pool walk", kind, id))
	}
	return found
}

// fill runs the set's one huge-pool walk on first use.
func (c *paymentCastCandidates) fill() {
	if c.ready {
		return
	}
	hyp := state.Mana{1 << 28, 1 << 28, 1 << 28, 1 << 28, 1 << 28, 1 << 28}
	// Only casts are read, so the walk skips the non-cast sections
	// (legalActionsWalk's castsOnly).
	opts := c.e.legalActionsWalkTemp(c.p, &hyp, true)
	for _, opt := range opts {
		if opt.Kind != "cast" || opt.AltCostIndex != 0 {
			continue
		}
		switch {
		case opt.Mode == "":
			c.ids = append(c.ids, opt.Obj)
		case planCastPaymentCheckedCodes.Code(opt.Mode) == planCastPaymentCheckedMayPlayMode && opt.MayPlayPerm == "":
			c.mayIDs = append(c.mayIDs, opt.Obj)
		case planCastFlashbackOption(opt):
			c.fbIDs = append(c.fbIDs, opt.Obj)
		case plannedAltFaceMode(opt.Mode):
			c.altIDs = append(c.altIDs, altCastCandidate{id: opt.Obj, mode: opt.Mode})
		}
	}
	c.e.optRelease(opts)
	c.ready = true
}

// hasFlashback is has for a planned flashback cast: membership of the
// primary-instance flashback casts the same walk listed (fbIDs). A priced
// set answers true, exactly as has does, and verify mode checks it.
func (c *paymentCastCandidates) hasFlashback(id state.ObjID) bool {
	if c.priced && !pricedCandidatesVerify {
		return true
	}
	c.fill()
	found := slices.Contains(c.fbIDs, id)
	if c.priced && !found {
		panic(fmt.Sprintf("payment plan: priced-walk flashback cast %d missing from the huge-pool walk", id))
	}
	return found
}

// planCastFlashbackOption reports whether opt is the cast option a planned
// flashback cast (decision.OriginFlashback) stands for: the primary flashback
// instance (no Option.Cost, which names a further instance), no alternative
// additional cost, no may-play permission.
func planCastFlashbackOption(opt decision.Option) bool {
	return opt.Kind == optCast && opt.Mode == "flashback" && opt.Cost == "" && opt.AltCostIndex == 0 && opt.MayPlayPerm == ""
}

// paymentPlanCastShapeOK excludes plain casts whose announced cost or result
// depends on a choice V1 cannot bind into its witness.  The ordinary priority
// option remains available; this only withholds the additive automatic offer.
func (e *Engine) paymentPlanCastShapeOK(p state.PlayerID, id state.ObjID) bool {
	return e.paymentPlanCastShapeDetail(p, id) == ""
}

func (e *Engine) paymentPlanCastShapeDetail(p state.PlayerID, id state.ObjID) string {
	return e.paymentPlanCastShapeDetailUsing(e.collectCostStatics(), p, id)
}

// paymentPlanCastShapeDetailUsing is paymentPlanCastShapeDetail over one
// already-collected cost-static set (the offer builder's, shared by every
// candidate of one decision).
func (e *Engine) paymentPlanCastShapeDetailUsing(statics costStaticViews, p state.PlayerID, id state.ObjID) string {
	scope := spellScope("")
	if o := e.G.Obj(id); o != nil && plannedMayPlayZone(o.Zone) {
		scope = spellScope("mayplay")
	}
	return e.paymentPlanCastShapeDetailScoped(statics, p, id, scope)
}

// paymentPlanCastShapeDetailScoped is paymentPlanCastShapeDetailUsing for a
// cast priced in scope: the may-play origins' "mayplay", a planned
// flashback's "flashback" (plannedCastScope), the plain casts' "".
func (e *Engine) paymentPlanCastShapeDetailScoped(statics costStaticViews, p state.PlayerID, id state.ObjID, scope costScope) string {
	return e.paymentPlanCastShapeDetailAltAdd(statics, p, id, scope, false)
}

// paymentPlanCastShapeDetailAltAdd is paymentPlanCastShapeDetailScoped with
// the mandatory either-or additional cost (AlternateAdditionalCost) either
// withheld as before or treated as plannable. planCastPaymentChecked plans
// the chosen part into the witness (planCastAltAdd), so the alternative's
// presence stops being a reason to withhold the cast; every other shape gate
// still answers.
func (e *Engine) paymentPlanCastShapeDetailAltAdd(statics costStaticViews, p state.PlayerID, id state.ObjID, scope costScope, altAddPlannable bool) string {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return "shape:additional_cost"
	}
	f := o.Face()
	if len(altAddCostParts(f)) != 0 && !altAddPlannable {
		return "shape:optional_cost"
	}
	if len(e.optionalCostViews(statics, p, id)) != 0 || len(e.reduceColorOptionalViews(statics, p, id)) != 0 {
		return "shape:optional_cost"
	}
	// Cost$ on the spell ability and the supported cost-static extra are both
	// additional costs; neither is represented by a mana-only plan witness.
	mods := e.costModifiersWithTargetsUsing(statics, p, id, scope, nil, false)
	spellCost := Cost{}
	if sa := f.SpellAbility(); sa != nil {
		spellCost = e.abilityCompiledCost(sa).Cost
	}
	// A cost static's own non-mana extra (Soul Immolation's Blight<X>) stays
	// withheld entirely: the mana-only witness cannot describe it.
	if pay.PlanCostDetail(mods.Extra) != "" {
		return "shape:additional_cost"
	}
	// The spell ability's own Cost$ admits exactly two non-mana shapes: a
	// fixed-count mandatory sacrifice or discard. The mana half of that same
	// cost still has to be V1-clean, which pay.PlanNonManaAdmissible checks
	// after removing the Sac and Discard parts. Every other non-mana part
	// (PayLife, Exile, tapXType, RevealOrChoose, Exert, ...) and every
	// variable-count Sac<X/...>/Sac<All/...> or Discard<X/...> still declines.
	if pay.PlanNonManaAdmissible(spellCost) != "" || pay.PlanNonManaAdmissible(pay.WithSpellAbilityExtras(f, Cost{})) != "" {
		return "shape:additional_cost"
	}
	if e.hasCastConvoke(id) || e.hasCastDelve(id, false) {
		return "shape:contribution"
	}
	// Improvise alone (CR 702.126a) is planned at the credited price with its
	// artifacts reserved (planImprovise); a mana remainder the lands pay is
	// still a witness (CR 601.2h). The shapes that credit does not price
	// stay with the manual window.
	if e.hasCastImprovise(id) && (scope.Mode != "" || len(spellCost.Sac) != 0 || len(spellCost.TapPermanent) != 0 ||
		costAnnouncesX(pay.RawBaseCost(asPayer(e), p, id))) {
		return "shape:contribution"
	}
	// Waterbend (a RaiseCost static's Waterbend<N>, Benevolent River
	// Spirit, or the spell's own Cost$) is the same tap-to-help credit as
	// convoke and improvise: the cast poses its helper choice before the
	// mana is paid, and any helper tapped there reprices the generic the
	// witness funded (and may tap a source the witness spends), so the
	// planned cast falls back cost_changed. potentialModeCastPlan declines it
	// for the same reason.
	if mods.Waterbend != 0 || mods.WaterbendX || mods.WaterbendPartX != 0 ||
		spellCost.Waterbend != 0 || spellCost.WaterbendX {
		return "shape:contribution"
	}
	// faceHasModeCost, memoised per face (face_scan_memo.go).
	if e.faceScanHas(f, faceScanModalCost) {
		return "shape:modal_cost"
	}
	for _, key := range []string{"Replicate", "Multikicker", "Squad"} {
		if _, ok := f.KeywordParam(key); ok {
			return "shape:optional_cost"
		}
	}
	if e.faceScanHas(f, faceScanReadsManaSpent) || faceWantsConverge(f) || faceWantsCastSpend(f) || e.paymentPlanBoardSpendReaderOut() {
		return "shape:mana_spent_reader"
	}
	if e.paymentPlanHasTargetDependentModifierUsing(statics, p, id) {
		return "shape:target_dependent_cost"
	}
	if _, ok := f.KeywordParam("Escalate"); ok {
		return "shape:optional_cost"
	}
	if _, ok := f.KeywordParam("Strive"); ok {
		return "shape:optional_cost"
	}
	return ""
}

// paymentPlanHasTargetDependentModifier finds a live cost static that would
// apply to this ordinary spell except for ValidTarget$, or one that applies
// and reads the announced targets in its amount, ValidSpell$ or CheckSVar$
// (costStaticReadsTargets: Battlefield Thaumaturge's "{1} less for each
// creature it targets").  Its actual amount is unknowable until CR 601.2c,
// after the plan has been selected, so V1 leaves that cast to the normal
// target/payment flow.  A spell that announces no
// target cannot meet any ValidTarget$ clause, so it is never taxed and is not
// declined (paymentPlanSpellTargets).  Copying the parameter map is
// important: static views share compiled-card maps.
func (e *Engine) paymentPlanHasTargetDependentModifier(p state.PlayerID, id state.ObjID) bool {
	if o := e.G.Obj(id); o == nil || !pay.PaymentPlanSpellTargets(o.Face()) {
		return false
	}
	return e.paymentPlanHasTargetDependentModifierUsing(e.collectCostStatics(), p, id)
}

// paymentPlanHasTargetDependentModifierUsing is
// paymentPlanHasTargetDependentModifier over one already-collected
// cost-static set.
func (e *Engine) paymentPlanHasTargetDependentModifierUsing(statics costStaticViews, p state.PlayerID, id state.ObjID) bool {
	return e.paymentPlanTargetDependentFor(statics, p, id, spellScope(""))
}

// paymentPlanTargetDependentFor is paymentPlanHasTargetDependentModifierUsing
// for a cast in scope (a flashback or bestowed cast's own scope).
func (e *Engine) paymentPlanTargetDependentFor(statics costStaticViews, p state.PlayerID, id state.ObjID, scope costScope) bool {
	if o := e.G.Obj(id); o == nil || !pay.PaymentPlanSpellTargets(o.Face()) {
		return false
	}
	for _, group := range []struct {
		mode  string
		views []staticView
	}{
		{"RaiseCost", statics.raise},
		{"ReduceCost", statics.reduce},
		{"SetCost", statics.set},
	} {
		for _, sv := range group.views {
			if strings.TrimSpace(sv.ParamStr(cards.PKValidTarget)) == "" {
				// No ValidTarget$, but its amount, ValidSpell$ or CheckSVar$
				// may still read the announced targets (Battlefield
				// Thaumaturge's "{1} less for each creature it targets"):
				// such a static prices the cast per announcement unless a
				// target-INDEPENDENT gate already denies it (costStaticGate's
				// indepFail), so it declines the witness too.
				// A self static (affinity, Card.Self) of another card never
				// prices this one: skip it before the text census.
				if id != sv.Source && sv.ParamStr(cards.PKValidCard) == "Card.Self" {
					continue
				}
				if costStaticReadsTargets(sv) {
					if ok, indepFail := e.costStaticGate(sv, group.mode, p, id, scope, nil, false); ok || !indepFail {
						return true
					}
				}
				continue
			}
			params := make(map[string]string, len(sv.Params)-1)
			for k, v := range sv.Params {
				if k != "ValidTarget" {
					params[k] = v
				}
			}
			sv.Params = params
			if e.costStaticApplies(sv, group.mode, p, id, scope, nil, false) {
				return true
			}
		}
	}
	return false
}

// costStaticReadsTargets reports whether a cost-modifier static WITHOUT
// ValidTarget$ can still price a cast differently per target announcement:
// a ValidSpell$ IsTargeting alternative (validSpellHasTargeting), or an
// Amount$ / CheckSVar$ whose expression -- through the static face's SVar
// table -- names a target reference (Targeted$, TargetedObjects$,
// TargetedObjectsDistinct$, TargetedController$, TargetedByTarget$,
// ParentTargeted$, AllTargeted$, SpellTargeted$, the targetedBy filter
// family ...). Every target-reading count head and filter property spells
// "target" (effects' ref switches), so the case-insensitive match over the
// transitive body is the census; a plain integer literal never reads one.
// The offer gate's markCostValidTarget is deliberately wider (any computed
// amount arms its cheap potential-target retry); the planner, which would
// decline the whole witness, keeps target-independent computed amounts
// (affinity, Count$Valid ... YouCtrl) plannable.
func costStaticReadsTargets(sv staticView) bool {
	if validSpellHasTargeting(sv.ParamStr(cards.PKValidSpell)) {
		return true
	}
	for _, v := range [...]string{sv.ParamStr(cards.PKAmount), sv.ParamStr(cards.PKCheckSVar)} {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, literal := parseInt10(v); literal {
			continue
		}
		if bodyReadsRef(v, sv.SVars, 0, pay.MentionsTarget) {
			return true
		}
	}
	return false
}

// PaymentActionsForPriority builds the additive extension for one concrete
// priority decision. ask publishes its result after fixing the decision Seq;
// callers may also inspect this pure builder without changing an ask.
// Without the decision in hand it derives BaseOptionIndex from a fresh
// legalActions walk; EnsurePaymentActions passes the pending Options instead.
func (e *Engine) PaymentActionsForPriority(p state.PlayerID, seq uint64) []decision.PaymentAction {
	// The builder's plans may be shared with the decision's cast-plan memo
	// (planCastPaymentMemo); hand the caller its own copy, as
	// EnsurePaymentActions does.
	Out, _, _, _, _ := e.paymentActionsForPriority(p, p, seq, nil, nil, nil)
	if Out == nil {
		return nil
	}
	return (&decision.Decision{PaymentActions: Out}).Clone().PaymentActions
}

// plannedCastKind is what one candidate walk option plans: the plain
// hand/command-zone cast, the untyped may-play cast, the primary flashback
// cast, or the hand alternate-spell-face cast.
type plannedCastKind uint8

const (
	planCastPlain plannedCastKind = iota + 1
	planCastMayPlay
	planCastFlashback
	planCastAltFace
)

// plannedPriorityCast classifies one candidate walk option the offer builder
// plans, and builds its PlannedCast. A V1 PlannedCast records the ordinary
// printed-cost cast only: an AlternativeCost has no Mode marker, but
// AltCostIndex identifies it, and accepting that option would build the same
// PlannedCast and canonical action ID as the ordinary cast -- apart from
// presenting the wrong cost, that duplicates IDs on the wire. The one other
// plain shape is the untyped may-play cast of an exile/graveyard card
// (Mode "mayplay"): a typed MayPlayText$ permission carries a MayPlayPerm and
// stays withheld. And the hand's alternate-spell-face casts
// (adventure_alt/omen_alt, CR 715.3a, 720.3) are the one Mode family beyond
// those the builder plans: they ride the plain-cast body under the flip to
// the spell face (planCastPaymentChecked's alternate-face arm). Every other
// Mode -- the alternative-cost and additional-cost families, split_alt,
// room_alt, modal_spell -- stays with the manual walk.
//
// The origin is derived from the object's actual zone (only the hand and the
// command zone ever offer a plain cast, and only exile and the graveyard a
// may-play one), and an alternate-spell-face option further requires the card
// in its owner's hand at face 0 with the face function naming Faces[1]
// (plannedAltSpellFace); anything else fails closed.
// A free function over the game, not an Engine method, so it adds nothing to
// the engine's method surface (the engineMethodCount ratchet).
func plannedPriorityCast(g *state.Game, opt decision.Option) (cast decision.PlannedCast, altFace *cards.Face, kind plannedCastKind, ok bool) {
	if opt.Kind != "cast" || opt.AltCostIndex != 0 {
		return cast, nil, 0, false
	}
	switch {
	case opt.Mode == "":
	case planCastPaymentCheckedCodes.Code(opt.Mode) == planCastPaymentCheckedMayPlayMode && opt.MayPlayPerm == "":
		kind = planCastMayPlay
	case planCastFlashbackOption(opt):
		kind = planCastFlashback
	case plannedAltFaceMode(opt.Mode):
		kind = planCastAltFace
	default:
		return cast, nil, 0, false
	}
	o := g.Obj(opt.Obj)
	if o == nil {
		return cast, nil, 0, false
	}
	switch kind {
	case planCastMayPlay:
		if !plannedMayPlayZone(o.Zone) {
			return cast, nil, 0, false
		}
		if o.Zone == state.ZLibrary {
			// The granting permission covers Card.TopLibrary: only the top
			// card of the library it sits in is castable, so the planned
			// cast names it and the origin check at plan and submit time
			// re-reads the top.
			lib := g.Zone(state.ZLibrary, o.Controller)
			if len(lib) == 0 || lib[0] != opt.Obj {
				return cast, nil, 0, false
			}
		}
		return decision.PlannedCast{Object: opt.Obj, Face: plannedCastFace(o), Origin: plannedMayPlayOrigin(o.Zone)}, nil, kind, true
	case planCastFlashback:
		if o.Zone != state.ZGraveyard {
			return cast, nil, 0, false
		}
		return decision.PlannedCast{Object: opt.Obj, Face: plannedCastFace(o), Origin: decision.OriginFlashback}, nil, kind, true
	case planCastAltFace:
		if o.Zone != state.ZHand || int(o.FaceIdx) != 0 {
			return cast, nil, 0, false
		}
		// The witness pays Face 1 (the spell face) in the cast's mode.
		cast = decision.PlannedCast{Object: opt.Obj, Face: 1, Origin: "hand", Mode: opt.Mode}
		af := plannedAltSpellFace(cast, o)
		return cast, af, kind, af != nil
	default:
		switch o.Zone {
		case state.ZHand:
			return decision.PlannedCast{Object: opt.Obj, Face: plannedCastFace(o), Origin: "hand"}, nil, planCastPlain, true
		case state.ZCommand:
			return decision.PlannedCast{Object: opt.Obj, Face: plannedCastFace(o), Origin: "command_zone"}, nil, planCastPlain, true
		}
		return cast, nil, 0, false
	}
}

// paymentActionsForPriority is the one-pass offer builder (spec 5 as amended:
// the pending decision's Options and one hypothetical candidate walk, not a
// full legal-action walk per candidate). options is the priority decision's
// own Options, which BaseOptionIndex indexes; nil derives them with one
// legalActions walk, and only once some action needs them.
//
// p is the seat the casts are planned FOR (the acting seat: its hand, mana
// base and pool); idSeat is the seat the action and plan IDs bind, the
// decision's ANSWERING seat, because Decision.Validate (a wire contract a
// rules-ignorant client also runs) re-derives them from d.Player. The two
// differ only under a CR 722 control redirect (Decision.Acting).
//
// Per build it runs ONE legal-action walk: the PotentialMana walk, which
// discovers the candidates and their order and labels, casts only
// (legalActionsWalk's castsOnly). The huge-pool walk PlanCastPayment
// requires of each candidate is implied by it (paymentCastCandidates'
// priced), and the PotentialMana walk also withholds a cast its potential
// sources cannot afford, which the huge-pool walk admits, so the planner
// only ever sees the candidates it saw before. Cost statics are collected
// once and shared by every candidate.
//
// potential is the caller's buffer for the build's cast census
// (decision.PotentialCasts): it is truncated and refilled with every object
// the walk offers a cast of, and known reports that the walk ran.
func (e *Engine) paymentActionsForPriority(p, idSeat state.PlayerID, seq uint64, options []decision.Option, potential, shapes []state.ObjID) (out []decision.PaymentAction, unpayable []state.ObjID, potentialCasts []state.ObjID, shapeCasts []state.ObjID, known bool) {
	potential = potential[:0]
	shapes = shapes[:0]
	if e.G.Over {
		return nil, nil, potential, shapes, false
	}
	e.PaymentStats.RecordBuild()
	// The builder's potential walk is what the priority walk's block record
	// serves (walk_block_reuse.go): record from now on.
	e.WalkRecDemand = true
	// The build is a pure read: one memo scope makes every nested walk
	// (PotentialMana, the candidate walk, each candidate's window-unit and
	// legality reads) share one generation and one board-static scan.
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	// One query scope (source census, alternatives) serves every
	// candidate's planner query, and is kept for the decision's other pure
	// payment readers (paymentPlanQueryResumeBegin).
	defer pay.PaymentPlanQueryEnd(asPayer(e), pay.PaymentPlanQueryBegin(asPayer(e)))
	defer e.paymentPlanQueryKeep(p)
	// legalActionsPriced is the authoritative candidate walk.  Its hypothetical
	// pool is only a superset gate; every admission below still has an exact
	// source-exclusive witness.
	// Only plain casts are read below, so the walk skips the sections that
	// append non-cast options (legalActionsWalk's castsOnly): the same cast
	// options in the same order, without pricing every battlefield ability.
	// The walk is shared with the decision's other potential readers
	// (potential_walk_cache.go), which may hand back the full walk.
	hyp, candidates := e.potentialWalkOf(p, false)
	for i := range candidates {
		if candidates[i].Kind == optCast && !slices.Contains(potential, candidates[i].Obj) {
			potential = append(potential, candidates[i].Obj)
		}
	}
	statics := costStaticSource{e: e}
	legal := paymentCastCandidates{e: e, p: p, priced: true}
	// The planner's "insufficient" is a PROOF only when the source census
	// prices every mana ability p could activate, or when a relaxation of the
	// abilities it misses (paymentPlanRelaxProof) still cannot pay. The cast
	// planner's raw verdict (planCastPaymentMemo) does no such post-processing,
	// so a cast payable only through an uncovered source's scripted prefix
	// (Saruli Caretaker, Wall of Roots, Heap Gate's filter) reaches the builder
	// as "insufficient" while PotentialPaymentPlans proves it payable. Marking
	// that a proof would hide a genuinely playable cast's "tap other mana
	// first" row, so the census gate below is the same proof the planner
	// reports (PotentialPaymentPlans), minus its script search: a
	// census-incomplete, non-relaxable insufficiency is not appended.
	var (
		census   paymentPlanCensus
		censused bool
	)
	provesUnpayable := func(cast decision.PlannedCast) bool {
		verdict := func() PaymentPlanOutcome {
			return e.planCastPaymentMemo(p, cast, &statics, &legal)
		}
		if got := verdict(); got.Reason != pay.ReasonInsufficient {
			return false
		}
		if !censused {
			census, censused = e.paymentPlanCensusOf(p, &hyp), true
		}
		if census.complete {
			return true
		}
		return census.relaxable && e.paymentPlanRelaxProof(census, verdict).Reason == pay.ReasonInsufficient
	}
	for _, opt := range candidates {
		cast, altFace, kind, ok := plannedPriorityCast(e.G, opt)
		if !ok {
			continue
		}
		got := e.planCastPaymentMemo(p, cast, &statics, &legal)
		e.PaymentStats.RecordOutcome(got)
		if got.Plan == nil {
			// The unpayable census names plain casts only
			// (decision.PotentialUnpayableCasts); a flashback proof is not
			// recorded there, nor an alternate-face one.
			if kind != planCastFlashback && kind != planCastAltFace && got.Reason == pay.ReasonInsufficient && provesUnpayable(cast) {
				unpayable = append(unpayable, opt.Obj)
			}
			// The plain casts declined for their cost's shape
			// (decision.PotentialShapeCasts).
			if kind != planCastFlashback && kind != planCastMayPlay && kind != planCastAltFace &&
				announcedShape(got) && !slices.Contains(shapes, opt.Obj) {
				shapes = append(shapes, opt.Obj)
			}
			continue
		}
		// The candidate walk's target census ran with the card in hand; the
		// planned cast asks for targets with it on the stack (CR 601.2a
		// before 601.2c). A one-click plan whose mandatory targets vanish
		// when the card leaves the hand would only reverse (CR 733.1), so
		// it is not offered (castprobe.go).
		if !castTargetsAvailableOnStackFace(e, p, opt.Obj, altFace) {
			continue
		}
		// Likewise the plan executes in the CR 601.2g window, after the card
		// left the hand: a planned source whose activation restriction or
		// amount reads the hand must still hold there, or the step falls
		// back source_changed and the cast reverses (castprobe.go). For an
		// alternate-spell-face cast the reads are judged under the flip to
		// the face beginCast flips to.
		if !paymentPlanHoldsOnStackFace(e, p, opt.Obj, altFace, *got.Plan) {
			continue
		}
		plan := *got.Plan
		pid, err := decision.PaymentPlanID(seq, idSeat, cast, plan)
		if err != nil {
			continue // impossible for a rules-built V1 witness; fail closed.
		}
		plan.ID = pid
		aid, err := decision.PaymentActionID(decision.PaymentPlanV1, seq, idSeat, cast)
		if err != nil {
			continue
		}
		a := decision.PaymentAction{ID: aid, Cast: cast, Label: opt.Label, Plans: []decision.PaymentPlan{plan}}
		if options == nil {
			options = e.legalActions(p)
		}
		for i := range options {
			if options[i].Kind == "cast" && options[i].Obj == opt.Obj && options[i].Mode == opt.Mode && options[i].MayPlayPerm == "" && options[i].AltCostIndex == 0 && options[i].Cost == "" {
				idx := options[i].Index
				a.BaseOptionIndex = &idx
				break
			}
		}
		out = append(out, a)
		e.PaymentStats.RecordOffered(len(a.Plans))
	}
	return out, unpayable, potential, shapes, true
}

// announcedShape reports whether a cast verdict declines the cast for its
// cost's shape -- a choice made as the spell is announced or a price its
// target sets (CR 601.2b, 601.2f: {X}, a hybrid face, an optional or
// additional cost, a target-dependent reduction), a Detail "cost:..." or
// "shape:..." -- rather than for its mana sources ("source:...") or its
// candidacy (no Detail).
func announcedShape(got PaymentPlanOutcome) bool {
	d := got.Detail
	return got.Reason == "unsupported" && (len(d) > 5 && d[:5] == "cost:" || len(d) > 6 && d[:6] == "shape:")
}

// ValidateCastPayment independently re-derives the eligible sources and cost
// then proves that exactly the submitted witness pays it.  It is deliberately
// usable by the executor without trusting an offer cache or an ID.
func (e *Engine) ValidateCastPayment(p state.PlayerID, cast decision.PlannedCast, plan decision.PaymentPlan) error {
	// A pure read: one Derived memo scope (derivedmemo.go) serves the
	// candidate walk, the planner and the census below, and one query scope
	// serves the planner's census to the rebuild.
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	defer pay.PaymentPlanQueryEnd(asPayer(e), e.paymentPlanQueryResumeBegin(p))
	got := e.planCastPaymentAtDecision(p, cast)
	// A Sac-bearing additional cost is answered by the ordinary in-flow
	// ask AFTER this validation, so the distinct-candidate assignment must
	// still hold at submit time. Re-run the offer gate's own sacrifice
	// feasibility check (nonManaCastable) on the composed cost and reject with
	// a clear error when a candidate left the battlefield between offer and
	// submit; the seat then falls back to the manual window instead of
	// committing a cast whose additional cost can no longer be paid.
	if o := e.G.Obj(cast.Object); o != nil && o.Face() != nil {
		composed := e.plannedCastComposedCost(p, cast, o)
		if (len(composed.Sac) != 0 || len(composed.Discard) != 0) && !pay.NonManaCastable(asPayer(e), p, cast.Object, composed, false, "") {
			return fmt.Errorf("payment plan sacrifice or discard cost no longer payable")
		}
	}
	if got.Reason == "unsupported" {
		return fmt.Errorf("payment plan unsupported")
	}
	if got.Plan == nil {
		return fmt.Errorf("payment plan %s", got.Reason)
	}
	if plan.Version != decision.PaymentPlanV1 || plan.Cost != got.Plan.Cost {
		return fmt.Errorf("payment plan cost or version changed")
	}
	if len(plan.Activations) > decision.MaxPaymentActivations {
		return fmt.Errorf("payment plan has too many activations")
	}
	// Rebuild only the explicitly named alternatives.  This is independent of
	// planner ranking: a valid non-preferred witness remains legal.
	units := pay.PaymentPlanQueryUnits(asPayer(e), p)
	produced := state.Mana{}
	var pain int64
	lastResort := false
	seen := make(map[state.ObjID]bool, len(plan.Activations))
	for _, pa := range plan.Activations {
		if seen[pa.Source] {
			return fmt.Errorf("payment source %d reused", pa.Source)
		}
		if pa.SourceZoneSeq != pay.PaymentSourceZoneSeq(asPayer(e), pa.Source) {
			return fmt.Errorf("payment source %d changed zone incarnation", pa.Source)
		}
		seen[pa.Source] = true
		step, ok := pay.PaymentPlanStepAlternative(asPayer(e), units, pa)
		if !ok {
			return fmt.Errorf("payment activation is no longer eligible")
		}
		pain += pay.ConsequencePain(step.Consequence)
		lastResort = lastResort || step.Tier == pay.TierLastResort
		produced = pay.ManaAdd(produced, step.Mana)
	}
	// The lethal guard and the phase rule (spec 5): a witness never kills its
	// caster, and it uses a last-resort source only when no plan from normal
	// sources exists -- exactly when the planner's own best plan uses one.
	if pain > 0 && pain >= int64(e.G.Players[p].Life) {
		return fmt.Errorf("payment plan life and damage would be lethal")
	}
	if lastResort && !pay.PlanUsesLastResort(*got.Plan) {
		return fmt.Errorf("payment plan uses a last-resort source while a normal plan exists")
	}
	// The settle price is the planner's: for a may-play origin the free grant
	// empties the mana part, the permission's RaiseCost$ surcharge is added
	// before the cost statics, and the spell-ability extras follow in
	// "mayplay" scope -- exactly planCastPaymentChecked and beginCast. Pricing
	// it here in plain scope would under-charge a raised cast and reject the
	// witness the planner just built.
	altFace := (*cards.Face)(nil)
	if o := e.G.Obj(cast.Object); o != nil {
		altFace = plannedAltSpellFace(cast, o)
	}
	settleBase := pay.RawBaseCost(asPayer(e), p, cast.Object)
	settleScope := spellScope("")
	if cast.Origin == decision.OriginFlashback {
		// A planned flashback settles at its primary flashback cost in
		// "flashback" scope (plannedFlashbackCost: extras, then statics).
		settleBase, settleScope = e.flashbackCost(cast.Object), spellScope("flashback")
	} else if o := e.G.Obj(cast.Object); o != nil && plannedMayPlayZone(o.Zone) {
		settleScope = spellScope("mayplay")
		if e.mayPlayFreeCast(p, cast.Object) {
			settleBase = Cost{}
		}
		if raise, hasRaise, priced := e.mayPlayRaiseCost(p, cast.Object); hasRaise {
			if !priced {
				return fmt.Errorf("payment plan may-play raise cost no longer priceable")
			}
			settleBase = settleBase.Plus(raise)
		}
	} else if altFace == nil {
		// A plain cast's Improvise credit, as planCastPaymentChecked prices it.
		settleBase, _ = e.improviseCost(p, cast.Object, settleBase, nil)
	}
	var cost Cost
	rider := pay.ConvRider(asEval(e).Conv(p, cast.Object, pay.SASpell))
	if altFace != nil {
		// An alternate-spell-face cast settles the flipped face's composed
		// charge in the cast mode's scope, exactly planCastPaymentChecked's
		// plain arm and beginCast charge it: the improvise credit and the
		// printed cost read the flip, the CR 601.2f statics price under it
		// (faceprobe.go), and the pip rider is the flip's own conversion.
		e.offerAsFace(cast.Object, altFace, func() bool {
			settleBase = pay.RawBaseCost(asPayer(e), p, cast.Object)
			settleBase, _ = e.improviseCost(p, cast.Object, settleBase, nil)
			settleScope = spellScope(cast.Mode)
			cost = pay.PlanManaHalf(e.offerCostFor(p, cast.Object, pay.WithSpellAbilityExtras(e.G.Obj(cast.Object).Face(), settleBase), settleScope))
			rider = pay.ConvRider(asEval(e).Conv(p, cast.Object, pay.SASpell))
			return true
		})
	} else {
		cost = pay.PlanManaHalf(e.offerCostFor(p, cast.Object, pay.WithSpellAbilityExtras(e.G.Obj(cast.Object).Face(), settleBase), settleScope))
	}
	if got.Plan != nil {
		// The witness planned the mandatory either-or additional cost's
		// chosen part (planCastAltAdd); settle the same total or the exact
		// witness re-check at the end rejects a plan that pays.
		cost = e.planCastAltAddSettleCost(p, cast, cost, *got.Plan)
	}
	if o := e.G.Obj(cast.Object); o != nil && plannedMayPlayZone(o.Zone) && cast.Origin != decision.OriginFlashback {
		rider = rider.Or(asPayer(e).Eval().MayPlayRider(p, cast.Object))
	}
	// The witness's Cost names the alternative the plan was built for: the
	// folded cost, or (under an AnyColor rider) one of the pinned costs that
	// keep a relaxed coloured pip funded from a coloured unit. Matching the
	// plan's wire cost picks exactly that alternative, so desc, the pool and
	// the settle below read the cost the executor will actually spend.
	cost, ok := pay.PlanManaUnderRiderMatching(cost, rider, got.Plan.Cost)
	if !ok {
		// A planned target-paid cast witnesses the cheapest legal target
		// set's composed price (planCastPaymentChecked's targetPaid arm), not
		// the printed composition: re-derive that enumeration so the settle
		// proves the price the target ask will charge. An
		// alternate-spell-face cast's composition reads the flip too.
		retry := func() {
			base := pay.WithSpellAbilityExtras(e.G.Obj(cast.Object).Face(), settleBase)
			if priced, alt := e.targetPricedPlan(e.collectCostStatics(), p, cast.Object, base, settleScope, rider); alt.Plan != nil && alt.Reason == "" {
				cost, ok = pay.PlanManaUnderRiderMatching(pay.PlanManaHalf(priced), rider, got.Plan.Cost)
			}
		}
		if altFace != nil {
			e.offerAsFace(cast.Object, altFace, func() bool { retry(); return true })
		} else {
			retry()
		}
	}
	if !ok {
		return fmt.Errorf("payment plan cost or version changed")
	}
	// The spend pool is read at the face being cast: a restricted batch
	// ("Spend this mana only to cast instant and sorcery spells") admits an
	// Adventure's Sorcery face but not its Artifact front, and the planner
	// built the witness under the flip (planCastPaymentChecked).
	var initial state.Mana
	poolFor := func() bool {
		desc := pay.DescriptorFor(cast.Object, false, cost)
		initial = pay.PlanPool(asPayer(e), p, &desc)
		return true
	}
	if altFace != nil {
		e.offerAsFace(cast.Object, altFace, poolFor)
	} else {
		poolFor()
	}
	payment, ok := resolveManaWith(cost, pay.ManaAdd(initial, produced), state.Mana{}, [8]state.Mana{}, e.G.Players[p].Life, false, pipRider{}, nil)
	expected := pay.Witness(cost, initial, produced, nil, payment.Pool)
	if !ok || pay.ManaAmount(payment.Pool) != plan.PoolAfter || expected.PoolSpend != plan.PoolSpend {
		return fmt.Errorf("payment witness does not settle")
	}
	return nil
}

// paymentPlanManaUnits extends the shared fixed-production payment census
// with the choice shapes a V1 witness can make concrete (Produced$ Any with a
// fixed amount, an amount-1 Combo/Chosen/ColorIdentity) and with the
// fixed-production abilities whose cost is not a bare tap (the census lists
// free-cost abilities only), so the tier gate sees every last-resort
// candidate (spec §3.2). The shared census must keep withholding the choice
// shapes for attack/unless windows, which cannot answer a colour choice; this
// planner records its selected W/U/B/R/G output and executes that exact
// rewrite.
//
// It additionally prices a source the shared census withholds because its
// Amount$ is not statically literal, when the engine's own evaluator resolves
// it to a value provably invariant to the payment's own activations
// (paymentPlanStableAmount): an Urza land's SVar-indirected
// Count$UrzaLands.3.1. The shared census must keep withholding those too,
// for the attack/unless windows, which can neither make a colour choice
// concrete nor re-verify a changing amount at activation.
//
// The census is a pure read, so it runs in one Derived memo scope
// (derivedmemo.go): outside a walk (the executor's per-step revalidation,
// ValidateCastPayment) every source's availability check otherwise
// re-derived the board. The two layers below ask each untapped source for
// the same payment-window membership, which reads the board only, so the
// first layer's list is kept (by zone position) for the second.
func (e *Engine) paymentPlanManaUnits(p state.PlayerID) []windowManaUnit {
	return e.paymentPlanManaUnitsOnly(p, nil)
}

// paymentPlanManaUnitsOnly is paymentPlanManaUnits restricted to the
// sources in only (nil: every source). Every layer below computes a
// source's unit from that source alone (the evaluated-amount probe is the
// whole board tapped, whichever source asks), so the restricted census is
// exactly the full census's units for those sources, in the same order;
// verify mode (walkCacheVerify) compares the two.
func (e *Engine) paymentPlanManaUnitsOnly(p state.PlayerID, only []state.ObjID) []windowManaUnit {
	units := e.paymentPlanManaUnitsOnlyCompute(p, only)
	if walkCacheVerify && only != nil {
		var want []windowManaUnit
		for _, u := range e.paymentPlanManaUnitsOnlyCompute(p, nil) {
			if slices.Contains(only, u.ID) {
				want = append(want, u)
			}
		}
		if !pay.SameUnits(units, want) {
			panic(fmt.Sprintf("payment plan census: restricted census for %v is not the full census's", only))
		}
	}
	return units
}

func (e *Engine) paymentPlanManaUnitsOnlyCompute(p state.PlayerID, only []state.ObjID) []windowManaUnit {
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	zone := e.G.Zone(state.ZBattlefield, p)
	// Each of p's sources' payment-window abilities, read once here for
	// both the shared census (windowManaUnitsWith) and the layers below:
	// the shared census reads every source with a face, tapped or not.
	windowMas := make([][]*cards.SA, len(zone))
	for zi, id := range zone {
		if only != nil && !slices.Contains(only, id) {
			continue
		}
		if o := e.G.Obj(id); o != nil && o.Face() != nil {
			windowMas[zi] = e.availableManaAbilitiesForWindow(p, id, false)
		}
	}
	units := e.windowManaUnitsWith(p, only, windowMas)
	for zi, id := range zone {
		if only != nil && !slices.Contains(only, id) {
			continue
		}
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		idx := -1
		for i := range units {
			if units[i].ID == id {
				idx = i
				break
			}
		}
		for _, ma := range windowMas[zi] {
			mf := e.manaStaticFor(p, id, ma)
			raw := mf.Produced
			amt := mf.Amount
			if amt <= 0 {
				continue
			}
			// Only a {T} cost needs the source untapped (CR 602.2b, 118.3):
			// a tapped Eldrazi Spawn still sacrifices for {C}.
			if (mf.Tap && o.Tapped) || (mf.Untap && !o.Tapped) {
				continue
			}
			counts, any := mf.Counts, mf.Any
			if !any {
				// Fixed production whose cost is not a bare tap (Eldrazi
				// Spawn's Sac<1/CARDNAME>: Add {C}) is outside the shared
				// census, which lists free-cost abilities only. Such an
				// ability is never normal (paymentPlanTapOnlyCost); the tier
				// gate in paymentPlanUnitAlternatives keeps it only when it is
				// a last-resort shape. A RestrictValid$ free ability is the
				// shared census's blind spot (windowManaUnitsWith keeps its
				// conservatism): it joins here, and the planner admits it per
				// payment (pay.PlanPaymentCostWithout's descriptor).
				if mf.FreeCost && !mf.RestrictValid {
					continue
				}
				total := int32(0)
				for _, n := range counts {
					total += n
				}
				if total <= 0 {
					continue
				}
				if idx < 0 {
					units = append(units, windowManaUnit{ID: id})
					idx = len(units) - 1
				}
				units[idx].Alts = append(units[idx].Alts, windowManaAlt{Ma: ma, Counts: counts, Amt: amt})
				continue
			}
			// The shared census keeps only deterministic production; extend it
			// with the finite choice shapes V1 can make concrete: Produced$ Any
			// (one alternative per colour, any literal amount) and an amount-1
			// Combo/Chosen/ColorIdentity (one alternative per producible
			// colour). Combo Any and every allocation (amount > 1) stay deferred.
			if alt, ok := paymentPlanReflectedAlt(e, p, id, ma, amt); ok {
				units, idx = setPaymentPlanUnitAlt(units, idx, id, alt)
				continue
			}
			if pay.PaymentPlanReflectedShape(ma) {
				// The shared census listed this reflected ability as its
				// blank colourless stand-in. The reflection names nothing
				// right now, so the ability produces nothing: drop the
				// stand-in, or the choice expansion would read the stand-in's
				// {C} as a real reflected colour.
				units = dropPaymentPlanUnitAlt(units, idx, id, ma)
				continue
			}
			if raw != "Any" && !pay.PaymentPlanChoiceShape(raw) {
				continue
			}
			if raw != "Any" && amt != 1 {
				continue
			}
			if idx < 0 {
				units = append(units, windowManaUnit{ID: id})
				idx = len(units) - 1
			}
			units[idx].Alts = append(units[idx].Alts, windowManaAlt{Ma: ma, Counts: counts, Amt: amt, Any: true})
		}
	}
	// Evaluated-amount layer: an ability windowManaUnits skipped because
	// availableAmount could not statically price its Amount$ is offered here
	// whenever paymentPlanStableAmount resolves it with the engine's own
	// evaluator AND proves the value cannot move when the payment taps its
	// sources. The probe is built lazily: a board with no evaluated-amount
	// source (the overwhelming majority) pays no clone at all.
	var probe *Engine
	for zi, id := range zone {
		if only != nil && !slices.Contains(only, id) {
			continue
		}
		o := e.G.Obj(id)
		if o == nil || o.Tapped || o.Face() == nil {
			continue
		}
		if walkCacheVerify {
			if fresh := e.availableManaAbilitiesForWindow(p, id, false); !slices.EqualFunc(fresh, windowMas[zi], pay.SameManaAbility) {
				panic(fmt.Sprintf("payment plan census: window membership for %d moved inside the census", id))
			}
		}
		for _, ma := range windowMas[zi] {
			mf := e.manaStaticFor(p, id, ma)
			if mf.Amount > 0 {
				continue // windowManaUnits' static path already priced it.
			}
			// Only a V1 source contract (a bare tap) can be executed from a
			// witness, so never build the probe for an ability the plan could
			// not activate anyway.
			if !mf.TapOnly {
				continue
			}
			if probe == nil {
				probe = e.paymentPlanTappedProbe(p)
			}
			amt, ok := e.paymentPlanStableAmount(probe, p, id, o, ma)
			if !ok {
				continue
			}
			mp := effects.ManaOf(ma)
			counts, any := mp.Counts, mp.CountsAny
			if any {
				units = pay.AppendUnitAlt(units, id, windowManaAlt{Ma: ma, Counts: counts, Amt: amt, Any: true})
				continue
			}
			total := int32(0)
			for _, n := range counts {
				total += n
			}
			if total <= 0 {
				continue
			}
			units = pay.AppendUnitAlt(units, id, windowManaAlt{Ma: ma, Counts: counts, Amt: amt})
		}
	}
	return units
}

// paymentPlanTappedProbe clones the engine and taps every battlefield
// permanent p controls. The clone is e.Clone(), whose zeroed layer caches
// (staticEpoch/activeEpoch/continuousVersion) force any Derived read to
// rebuild against the cloned, all-tapped board rather than serving the live
// engine's cache. It is a pure throwaway: the clone never emits and is
// discarded after the two evaluations in paymentPlanStableAmount.
func (e *Engine) paymentPlanTappedProbe(p state.PlayerID) *Engine {
	probe := e.Clone()
	for _, id := range probe.G.Zone(state.ZBattlefield, p) {
		if o := probe.G.Obj(id); o != nil {
			o.Tapped = true
		}
	}
	return probe
}

// paymentPlanStableAmount prices a mana ability's Amount$ with the engine's
// own evaluator (castWindowAmount: the source face's SVar table and
// effects.Num's grammar) and admits the value only when it is provably
// invariant to the state a V1 payment changes for its sources.
//
// A V1 plan admits only normal-tier, tap-only alternatives
// (paymentPlanAbilityShapeTier / paymentPlanTapOnlyCost): activating one taps
// its source, adds mana, and changes nothing else -- no sacrifice, no return,
// no life. So the only per-object field an amount could read that the payment
// moves is the tapped bit. This proves invariance constructively, not by
// enumerating count heads: it re-evaluates the same expression on a clone
// whose battlefield permanents are all tapped, and requires the two values to
// agree. A count of untapped permanents therefore refuses the plan, while the
// Urza lands' presence/subtype count is unchanged.
//
// The check can only be too strict, never too lax, and even then it is not the
// last word: the executor re-derives this alternative at every activation and
// compares the mana actually added with the witness
// (paymentPlanStepReady / paymentPlanProducedExactly), so a value that moved
// between the plan and the activation is a plan failure, never a silent
// overpay.
func (e *Engine) paymentPlanStableAmount(probe *Engine, p state.PlayerID, source state.ObjID, o *state.Object, ma *cards.SA) (int32, bool) {
	amt, ok := e.castWindowAmount(p, source, o, ma)
	if !ok || amt <= 0 {
		return 0, false
	}
	if probeAmt, ok := probe.castWindowAmount(p, source, o, ma); !ok || probeAmt != amt {
		return 0, false
	}
	return amt, true
}

// paymentPlanManaInterference is the zero-argument global gate the planned
// executor re-runs before each later step: whether a mana effect whose scope
// the planner cannot prove (paymentPlanGlobalManaEffect) reaches the acting
// payer -- the cast in progress when there is one, else any alive player.
// Everything the planner CAN scope -- a trigger or replacement that can match
// one source's tap or production -- is a per-source tier question
// (paymentPlanSourceInterference), which the executor re-reads through
// paymentPlanUnitAlternatives.
func (e *Engine) paymentPlanManaInterference() bool {
	if pc := e.cast; pc != nil {
		global, _ := e.paymentPlanGlobalManaEffect(pc.player, pc.card)
		return global
	}
	for _, p := range e.G.AliveFrom(0) {
		if global, _ := e.paymentPlanGlobalManaEffect(p, 0); global {
			return true
		}
	}
	return false
}

type planCastPaymentCheckedCode uint16

const (
	planCastPaymentCheckedHand planCastPaymentCheckedCode = iota + 1
	planCastPaymentCheckedCommandZone
	planCastPaymentCheckedExile
	planCastPaymentCheckedGraveyard
	planCastPaymentCheckedMayPlayMode
	planCastPaymentCheckedFlashback
	planCastPaymentCheckedLibrary
)

var planCastPaymentCheckedCodes = state.NewStrCodes(
	state.StrEntry[planCastPaymentCheckedCode]{Key: "hand", Val: planCastPaymentCheckedHand},
	state.StrEntry[planCastPaymentCheckedCode]{Key: "command_zone", Val: planCastPaymentCheckedCommandZone},
	state.StrEntry[planCastPaymentCheckedCode]{Key: "exile", Val: planCastPaymentCheckedExile},
	state.StrEntry[planCastPaymentCheckedCode]{Key: "graveyard", Val: planCastPaymentCheckedGraveyard},
	state.StrEntry[planCastPaymentCheckedCode]{Key: "mayplay", Val: planCastPaymentCheckedMayPlayMode},
	state.StrEntry[planCastPaymentCheckedCode]{Key: decision.OriginFlashback, Val: planCastPaymentCheckedFlashback},
	state.StrEntry[planCastPaymentCheckedCode]{Key: decision.OriginLibrary, Val: planCastPaymentCheckedLibrary},
)

// plannedCastScope is the cost-static scope a planned cast from zone z is
// priced in: "flashback" for a planned flashback cast, "mayplay" for a
// may-play origin (exile, graveyard), "" for the hand and command zone --
// the scope beginCast charges each one's option Mode in. A hand
// alternate-spell-face cast composes in its own Mode's scope instead
// (planCastPaymentChecked's alternate-face arm).
func plannedCastScope(z state.Zone, flashback bool) costScope {
	switch {
	case flashback:
		return spellScope("flashback")
	case plannedMayPlayZone(z):
		return spellScope("mayplay")
	}
	return spellScope("")
}

// plannedAltFaceMode reports whether mode is one of the two hand
// alternate-spell-face cast modes the payment planner plans (CR 715.3a
// adventure_alt, CR 720.3 omen_alt): the same word dispatch cast_targets.go's
// modeFlagsCodes owns. Every other Mode -- the alternative-cost and
// additional-cost families, split_alt, room_alt, modal_spell -- is planned
// nowhere and stays withheld.
func plannedAltFaceMode(mode string) bool {
	switch modeFlagsCodes.Code(mode) {
	case modeFlagsAdventureAlt, modeFlagsOmenAlt:
		return true
	}
	return false
}

// plannedAltSpellFace is the alternate spell face (adventure.go's
// adventureSpellFace/omenSpellFace, face 1 -- ALTERNATE starts face 1) a
// planned hand cast of o in one of the planner's two alternate-face modes
// flips to, or nil when cast is not such a cast: the origin is the hand, the
// object shows face 0, and the face function names Faces[cast.Face].
func plannedAltSpellFace(cast decision.PlannedCast, o *state.Object) *cards.Face {
	if planCastPaymentCheckedCodes.Code(string(cast.Origin)) != planCastPaymentCheckedHand ||
		cast.Face != 1 || o == nil || o.Card == nil || int(o.FaceIdx) != 0 {
		return nil
	}
	switch modeFlagsCodes.Code(cast.Mode) {
	case modeFlagsAdventureAlt:
		return adventureSpellFace(o)
	case modeFlagsOmenAlt:
		return omenSpellFace(o)
	}
	return nil
}

// plannedAltGate is the slice of the Engine the alternate-spell-face cast's
// two offer gates read, a narrow interface so the free functions below add
// nothing to the Engine surface (the engineMethodCount/engineSurface
// ratchets; castprobe.go's castOfferProbe shape).
type plannedAltGate interface {
	offerAsFace(id state.ObjID, face *cards.Face, fn func() bool) bool
	castTargetsAvailableOnStack(p state.PlayerID, id state.ObjID) bool
	paymentPlanHoldsOnStack(p state.PlayerID, id state.ObjID, plan decision.PaymentPlan) bool
}

// castTargetsAvailableOnStackFace is castTargetsAvailableOnStack for a
// planned alternate-spell-face cast: the census the CR 601.2c target ask
// runs with the card on the stack AND flipped to the face beginCast flips to
// (faceprobe.go -- the same probe offerCastableAsFace prices under). The
// stack probe nests inside the face probe; each restores its own reads.
// altFace nil answers castTargetsAvailableOnStack unchanged.
func castTargetsAvailableOnStackFace(w plannedAltGate, p state.PlayerID, id state.ObjID, altFace *cards.Face) bool {
	if altFace == nil {
		return w.castTargetsAvailableOnStack(p, id)
	}
	return w.offerAsFace(id, altFace, func() bool {
		return w.castTargetsAvailableOnStack(p, id)
	})
}

// paymentPlanHoldsOnStackFace is paymentPlanHoldsOnStack for a planned
// alternate-spell-face cast: the plan executes in the CR 601.2g window with
// the card on the stack at the face beginCast flips to, so an activation
// restriction or amount reading the caster's hand card's characteristics is
// judged under the flip too (faceprobe.go). altFace nil answers
// paymentPlanHoldsOnStack unchanged.
func paymentPlanHoldsOnStackFace(w plannedAltGate, p state.PlayerID, id state.ObjID, altFace *cards.Face, plan decision.PaymentPlan) bool {
	if altFace == nil {
		return w.paymentPlanHoldsOnStack(p, id, plan)
	}
	return w.offerAsFace(id, altFace, func() bool {
		return w.paymentPlanHoldsOnStack(p, id, plan)
	})
}

// plannedFlashbackCost is the cost a planned flashback cast of id locks in
// (CR 601.2f), composed exactly as the offer walk (legal_walk_alt.go's
// flashback arm) and beginCast's "flashback" mode charge it: the primary
// flashback cost (flashbackCost: the printed parameter, or the card's mana
// cost for a parameterless grant) with the spell ability's own additional
// costs folded in (CR 702.34a replaces only the mana cost), then re-priced by
// every live cost static in "flashback" scope.
func (e *Engine) plannedFlashbackCost(statics costStaticViews, p state.PlayerID, id state.ObjID) Cost {
	f := e.G.Obj(id).Face()
	return e.offerCostForUsing(statics, p, id, pay.WithSpellAbilityExtras(f, e.flashbackCost(id)), spellScope("flashback"))
}

// plannedCastComposedCost is the cost a planned cast locks in, read for the
// submit-time checks (ValidateCastPayment's sacrifice feasibility): the
// primary flashback cost in "flashback" scope for decision.OriginFlashback,
// the alternate spell face's composed cost under the flip in the cast mode's
// scope for a hand alternate-spell-face cast, else the printed cost with its
// extras in the origin zone's scope.
func (e *Engine) plannedCastComposedCost(p state.PlayerID, cast decision.PlannedCast, o *state.Object) Cost {
	if cast.Origin == decision.OriginFlashback {
		return e.plannedFlashbackCost(e.collectCostStatics(), p, cast.Object)
	}
	if af := plannedAltSpellFace(cast, o); af != nil {
		var composed Cost
		e.offerAsFace(cast.Object, af, func() bool {
			composed = e.offerCostFor(p, cast.Object, pay.WithSpellAbilityExtras(af, pay.RawBaseCost(asPayer(e), p, cast.Object)), spellScope(cast.Mode))
			return true
		})
		return composed
	}
	return e.offerCostFor(p, cast.Object, pay.WithSpellAbilityExtras(o.Face(), pay.RawBaseCost(asPayer(e), p, cast.Object)), plannedCastScope(o.Zone, false))
}

// plannedMayPlayZone reports whether z is an origin whose planned cast is an
// untyped may-play cast (the "mayplay" mode): exile, the graveyard and the
// library top.
func plannedMayPlayZone(z state.Zone) bool {
	return z == state.ZExile || z == state.ZGraveyard || z == state.ZLibrary
}

// plannedMayPlayOrigin is the PlannedCast.Origin code of a may-play zone.
func plannedMayPlayOrigin(z state.Zone) string {
	if z == state.ZGraveyard {
		return "graveyard"
	}
	if z == state.ZLibrary {
		return decision.OriginLibrary
	}
	return "exile"
}

// plannedCastOption is the priority option an announced or planned cast of
// cast begins as. A may-play origin (exile, graveyard, library top) begins
// as Mode
// "mayplay" so beginCast prices the permission's cost and records the hosts
// while the card is still in the granted zone; a planned flashback
// (decision.OriginFlashback) begins as Mode "flashback", the primary
// instance (no Option.Cost); the hand and command-zone plain casts carry no
// Mode, exactly as before (decision.PlannedCast.CastMode).
func plannedCastOption(cast decision.PlannedCast) decision.Option {
	return decision.Option{Kind: "cast", Obj: cast.Object, Mode: cast.CastMode()}
}

// plannedCastFace is the Face of a planned plain cast of o: the face the
// object shows (CR 709.3). It is 0 for every ordinary card; a Room that left
// the battlefield as its other door keeps that door, so its plain cast is the
// shown half's and hard-coding 0 withheld it from the planner's offers (the
// autopay surface and the potential walk) while the manual walk offered it.
func plannedCastFace(o *state.Object) int {
	if o != nil {
		return int(o.FaceIdx)
	}
	return 0
}

// targetPaidShape reports the declined cast shapes whose plan can still be
// admitted when the cast needs it (targetPaidNeedsPlannedPayment): each leaves
// the lowering unable to cast a spell whose only target its payment consumes.
func targetPaidShape(detail string) bool {
	switch detail {
	case "shape:modal_cost", "shape:additional_cost", "shape:optional_cost", "shape:contribution",
		"shape:mana_spent_reader", "shape:target_dependent_cost":
		return true
	}
	return false
}

// targetPaidNeedsPlannedPayment reports whether a cast of a shape the planner
// withholds (Spree or Tiered modes, an additional or optional cost, a
// convoke-like credit, a mana-spent reader, a target-dependent price) is
// payable only by paying with a permanent that the cast's own announcements
// need. Its targets and modes are chosen before any mana ability is
// activated (CR 601.2b-c), and mana abilities only then (601.2g), so the cast
// may sacrifice its own target to pay (the spell then resolves with an illegal
// target, CR 608.2b). The potential walk's lowering floats that mana before
// the cast and so cannot reach it (potentialWitnessReaches); the planned cast
// pays at 601.2g, and its witness at the printed cost falls back to the
// ordinary mana window once the chosen modes change the cost. A witness whose
// sacrifice sets off a trigger that ends the game in p's favour reaches the play
// (witnessReachesMode), as the potential walk counts it.
func (e *Engine) targetPaidNeedsPlannedPayment(p state.PlayerID, id state.ObjID) bool {
	if e.Pending() == nil {
		return false
	}
	// The plan is at the printed cost; a credit (convoke, improvise, delve,
	// waterbend) is chosen in the cast itself and reprices it, which the
	// planned cast answers by falling back to the manual window.
	e.planIgnoreCredit = true
	defer func() { e.planIgnoreCredit = false }()
	o := decision.Option{Kind: "cast", Obj: id}
	// An either-or additional cost (AlternateAdditionalCost) is folded into
	// the total only by the cast's own announcement (altAddAsk), after this
	// query's plan: potentialModeCastPlan completes the retained mana plan
	// with the parts (potentialAltAddSacrificePlan, potentialAltAddPlan), and
	// a part can consume a permanent the mana plan leaves alone -- even the
	// very target the cast announces. The pre-gates below read only the
	// search's mana plan and the census's mana alternatives, neither of
	// which carries a part, so they cannot prove such a cast needs nothing:
	// it is planned, never pre-gated.
	altAdd := false
	if obj := e.G.Obj(id); obj != nil && obj.Face() != nil {
		altAdd = len(altAddCostParts(obj.Face())) != 0
	}
	if !altAdd && e.planPoolCoversPrintedCost(p, o) {
		// The floating pool alone pays the composed printed cost, so a
		// payment that consumes nothing exists and the cast is not payable
		// only by paying with a permanent its own announcements need
		// (planPoolCoversPrintedCost) -- without the census pass or the plan.
		return false
	}
	if !altAdd && e.planBoardConsumesNothing(p) {
		// No census alternative consumes its source (planCoverConsumable from
		// the printed-face scan), so no plan this query can read consumes
		// either (planCoverConsumable): the cast is not payable only by
		// paying with a needed permanent, whatever its targets and modes
		// price to -- without the census pass or the plan.
		return false
	}
	e.planConsumptionOnly = true
	e.planCoverBoundOn = true
	if _, consumable := e.planCoverBoundOf(p); !altAdd && !consumable {
		// No census alternative consumes its source, so no plan this query
		// can read consumes either (planCoverConsumable): the cast is not
		// payable only by paying with a needed permanent, whatever its
		// targets and modes price to.
		e.planCoverBoundOn = false
		e.planConsumptionOnly = false
		return false
	}
	got := e.potentialModeCastPlan(p, o)
	e.planCoverBoundOn = false
	e.planConsumptionOnly = false
	if got.Plan == nil || got.Reason != "" || !paymentPlanConsumes(*got.Plan) || e.witnessReachesMode(e, p, o, *got.Plan, true, false) {
		return false
	}
	return e.potentialWitnessKeeping(p, o).Reason != ""
}
