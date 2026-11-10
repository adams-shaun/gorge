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
	// V1 plans four ordinary-cast origins: the hand (the original shape),
	// the command zone (the CR 903.8 plain taxed cast; alternate
	// command-zone casts -- dash, evoke, bestow, ... -- carry a Mode and are
	// withheld, as everywhere else), and the exile zone and graveyard, where
	// only an untyped may-play permission's plain cast is planned (the
	// "mayplay" cast mode; a MayPlayText$-typed permission, and every
	// alternative cost, are withheld).  Any other origin fails closed, and
	// the object must actually sit in its origin's zone: a mislabeled origin
	// would compose a cost (and, from ZCommand, a commander tax) that
	// beginCast would never charge.
	originZone := state.ZHand
	flashback := false
	targetPaid := false
	switch planCastPaymentCheckedCodes.Code(string(cast.Origin)) {
	case planCastPaymentCheckedHand:
	case planCastPaymentCheckedCommandZone:
		originZone = state.ZCommand
	case planCastPaymentCheckedExile:
		originZone = state.ZExile
	case planCastPaymentCheckedGraveyard:
		originZone = state.ZGraveyard
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
	if o == nil || o.Zone != originZone || (!mayPlay && o.Owner != p) || o.Face() == nil || int(o.FaceIdx) != cast.Face {
		return PaymentPlanOutcome{Reason: "unsupported"}
	}
	scope := plannedCastScope(o.Zone, flashback)
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
		if !candidates.has(cast.Object, true) {
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
		base = pay.WithSpellAbilityExtras(o.Face(), base)
		cost = e.offerCostForUsing(statics.get(), p, cast.Object, base, spellScope(""))
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
	mana = pay.PlanManaUnderRider(mana, rider)
	if len(cost.Sac) == 0 && len(cost.TapPermanent) == 0 {
		return pay.PlanPaymentCost(asPayer(e), p, cast, mana, &desc)
	}
	// The mana abilities run first (CR 601.2g-h), so a source that sacrifices
	// itself can use up what an additional cost must sacrifice.
	parts, _ := planCostParts(cost, cast.Object, false)
	return e.planReserving(p, cast.Object, cost, false, false, parts, func(tapR, keptR []state.ObjID) PaymentPlanOutcome {
		return pay.PlanPaymentCostWithout(asPayer(e), p, cast, mana, &desc, tapR, nil, keptR)
	})
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
		composed = pay.PlanManaUnderRider(composed, rider)
		got := e.planComposedCostFor(p, cast.Object, cast, composed, false, false)
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
		if got.Plan != nil && got.Reason == "" && (nonMana || e.potentialWitnessOffers(p, opt, *got.Plan)) {
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
}

// pricedCandidatesVerify: see derivedMemoVerify. Set by the rules test binary.
var pricedCandidatesVerify = derivedMemoVerifyFlag != ""

func (c *paymentCastCandidates) has(id state.ObjID, mayPlay ...bool) bool {
	if c.priced && !pricedCandidatesVerify {
		return true
	}
	c.fill()
	wantMayPlay := len(mayPlay) > 0 && mayPlay[0]
	found := false
	if wantMayPlay {
		found = slices.Contains(c.mayIDs, id)
	} else {
		found = slices.Contains(c.ids, id)
	}
	if c.priced && !found {
		kind := "plain cast"
		if wantMayPlay {
			kind = "may-play cast"
		}
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
	if e.hasCastConvoke(id) || e.hasCastImprovise(id) || e.hasCastDelve(id, false) {
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
		// A V1 PlannedCast records the ordinary printed-cost cast only.  An
		// AlternativeCost has no Mode marker, but AltCostIndex identifies it;
		// accepting that option would build the same PlannedCast and canonical
		// action ID as the ordinary cast.  Apart from presenting the wrong
		// cost, that produces duplicate IDs on the wire.
		// The one other plain shape is the untyped may-play cast of an
		// exile/graveyard card (Mode "mayplay"): a typed MayPlayText$
		// permission carries a MayPlayPerm and stays withheld.
		mayPlay := planCastPaymentCheckedCodes.Code(opt.Mode) == planCastPaymentCheckedMayPlayMode && opt.MayPlayPerm == ""
		flashback := planCastFlashbackOption(opt)
		if opt.Kind != "cast" || (opt.Mode != "" && !mayPlay && !flashback) || opt.AltCostIndex != 0 {
			continue
		}
		// Derive the origin from the object's actual zone.  Only the hand and
		// the command zone (Commander format) ever offer a plain cast
		// (Mode "" AltCostIndex 0), and only exile and the graveyard a
		// may-play one; anything else fails closed.
		originObj := e.G.Obj(opt.Obj)
		if originObj == nil {
			continue
		}
		var origin string
		switch {
		case flashback && originObj.Zone == state.ZGraveyard:
			origin = decision.OriginFlashback
		case flashback:
			continue
		case mayPlay && plannedMayPlayZone(originObj.Zone):
			origin = plannedMayPlayOrigin(originObj.Zone)
		case mayPlay:
			continue
		case originObj.Zone == state.ZHand:
			origin = "hand"
		case originObj.Zone == state.ZCommand:
			origin = "command_zone"
		default:
			continue
		}
		cast := decision.PlannedCast{Object: opt.Obj, Face: plannedCastFace(originObj), Origin: origin}
		got := e.planCastPaymentMemo(p, cast, &statics, &legal)
		e.PaymentStats.RecordOutcome(got)
		if got.Plan == nil {
			// The unpayable census names plain casts only
			// (decision.PotentialUnpayableCasts); a flashback proof is not
			// recorded there.
			if !flashback && got.Reason == pay.ReasonInsufficient && provesUnpayable(cast) {
				unpayable = append(unpayable, opt.Obj)
			}
			// The plain casts declined for their cost's shape
			// (decision.PotentialShapeCasts).
			if !flashback && !mayPlay && announcedShape(got) && !slices.Contains(shapes, opt.Obj) {
				shapes = append(shapes, opt.Obj)
			}
			continue
		}
		// The candidate walk's target census ran with the card in hand; the
		// planned cast asks for targets with it on the stack (CR 601.2a
		// before 601.2c). A one-click plan whose mandatory targets vanish
		// when the card leaves the hand would only reverse (CR 733.1), so
		// it is not offered (castprobe.go).
		if !e.castTargetsAvailableOnStack(p, opt.Obj) {
			continue
		}
		// Likewise the plan executes in the CR 601.2g window, after the card
		// left the hand: a planned source whose activation restriction or
		// amount reads the hand must still hold there, or the step falls
		// back source_changed and the cast reverses (castprobe.go).
		if !e.paymentPlanHoldsOnStack(p, opt.Obj, *got.Plan) {
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
	}
	cost := pay.PlanManaHalf(e.offerCostFor(p, cast.Object, pay.WithSpellAbilityExtras(e.G.Obj(cast.Object).Face(), settleBase), settleScope))
	if got.Plan != nil {
		// The witness planned the mandatory either-or additional cost's
		// chosen part (planCastAltAdd); settle the same total or the exact
		// witness re-check at the end rejects a plan that pays.
		cost = e.planCastAltAddSettleCost(p, cast, cost, *got.Plan)
	}
	rider := pay.ConvRider(asEval(e).Conv(p, cast.Object, pay.SASpell))
	if o := e.G.Obj(cast.Object); o != nil && plannedMayPlayZone(o.Zone) && cast.Origin != decision.OriginFlashback {
		rider = rider.Or(asPayer(e).Eval().MayPlayRider(p, cast.Object))
	}
	cost = pay.PlanManaUnderRider(cost, rider)
	if got.Plan != nil && pay.WireCost(cost) != got.Plan.Cost {
		// A planned target-paid cast witnesses the cheapest legal target
		// set's composed price (planCastPaymentChecked's targetPaid arm), not
		// the printed composition: re-derive that enumeration so the settle
		// proves the price the target ask will charge.
		base := pay.WithSpellAbilityExtras(e.G.Obj(cast.Object).Face(), settleBase)
		if priced, alt := e.targetPricedPlan(e.collectCostStatics(), p, cast.Object, base, settleScope, rider); alt.Plan != nil && alt.Reason == "" {
			if altCost := pay.PlanManaUnderRider(pay.PlanManaHalf(priced), rider); pay.WireCost(altCost) == got.Plan.Cost {
				cost = altCost
			}
		}
	}
	desc := pay.DescriptorFor(cast.Object, false, cost)
	initial := pay.PlanPool(asPayer(e), p, &desc)
	payment, ok := resolveManaWith(cost, pay.ManaAdd(initial, produced), state.Mana{}, [7]state.Mana{}, e.G.Players[p].Life, false, pipRider{}, nil)
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
)

var planCastPaymentCheckedCodes = state.NewStrCodes(
	state.StrEntry[planCastPaymentCheckedCode]{Key: "hand", Val: planCastPaymentCheckedHand},
	state.StrEntry[planCastPaymentCheckedCode]{Key: "command_zone", Val: planCastPaymentCheckedCommandZone},
	state.StrEntry[planCastPaymentCheckedCode]{Key: "exile", Val: planCastPaymentCheckedExile},
	state.StrEntry[planCastPaymentCheckedCode]{Key: "graveyard", Val: planCastPaymentCheckedGraveyard},
	state.StrEntry[planCastPaymentCheckedCode]{Key: "mayplay", Val: planCastPaymentCheckedMayPlayMode},
	state.StrEntry[planCastPaymentCheckedCode]{Key: decision.OriginFlashback, Val: planCastPaymentCheckedFlashback},
)

// plannedCastScope is the cost-static scope a planned cast from zone z is
// priced in: "flashback" for a planned flashback cast, "mayplay" for a
// may-play origin (exile, graveyard), "" for the hand and command zone --
// the scope beginCast charges each one's option Mode in.
func plannedCastScope(z state.Zone, flashback bool) costScope {
	switch {
	case flashback:
		return spellScope("flashback")
	case plannedMayPlayZone(z):
		return spellScope("mayplay")
	}
	return spellScope("")
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
// else the printed cost with its extras in the origin zone's scope.
func (e *Engine) plannedCastComposedCost(p state.PlayerID, cast decision.PlannedCast, o *state.Object) Cost {
	if cast.Origin == decision.OriginFlashback {
		return e.plannedFlashbackCost(e.collectCostStatics(), p, cast.Object)
	}
	return e.offerCostFor(p, cast.Object, pay.WithSpellAbilityExtras(o.Face(), pay.RawBaseCost(asPayer(e), p, cast.Object)), plannedCastScope(o.Zone, false))
}

// plannedMayPlayZone reports whether z is an origin whose planned cast is an
// untyped may-play cast (the "mayplay" mode): exile and the graveyard.
func plannedMayPlayZone(z state.Zone) bool {
	return z == state.ZExile || z == state.ZGraveyard
}

// plannedMayPlayOrigin is the PlannedCast.Origin code of a may-play zone.
func plannedMayPlayOrigin(z state.Zone) string {
	if z == state.ZGraveyard {
		return "graveyard"
	}
	return "exile"
}

// plannedCastOption is the priority option an announced or planned cast of
// cast begins as. A may-play origin (exile, graveyard) begins as Mode
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
	if got.Plan == nil || got.Reason != "" || !paymentPlanConsumes(*got.Plan) || e.witnessReachesMode(e, p, o, *got.Plan, true) {
		return false
	}
	return e.potentialWitnessKeeping(p, o).Reason != ""
}
