package rules

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/rules/pay"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// PotentialPlan is the payment planner's verdict on one potential play
// (PotentialActions): whether the seat's floating pool plus its mana
// sources can pay the play's mana, and the exact witness (or scripted
// prefix) that does.
//
// It serves a seat playing a surface where mana is tapped before the play
// is offered (a manual-mana engine): the priority offer carries payment
// plans for ordinary casts only (Decision.PaymentActions), so an activated
// ability with a mana cost ({2}: Heap Gate's draw) had no witness, and the
// seat could only tap sources one by one and hope. With Plan it taps exactly
// the witness's sources and then activates.
type PotentialPlan struct {
	// Action is the play as PotentialActions projects it (raw label).
	Action decision.PotentialAction
	// Plan is the witness paying the play's mana part, set for an ordinary
	// cast (the same witness Decision.PaymentActions offers while the pool
	// is plain), a printed activated ability (Kind "ability" with a pile
	// index), a cast in an alternative or additional-cost mode (Mode
	// flashback, bestowed, kicked ...), an ordinary {X} cast (the largest X
	// the sources pay) and a hybrid one.
	// Its Activations are what to tap; the play's non-mana costs (a tap of
	// the source, a sacrifice, a discard) and its X are the play's own asks.
	Plan *decision.PaymentPlan
	// Reason is the planner verdict: "" (payable: Plan or Script says
	// how; an ordinary cast whose witness rides Decision.PaymentActions
	// may carry neither), "insufficient" (PROVEN unpayable: the census
	// covers every mana ability p could activate, or a relaxation of the
	// ones it misses, and no plan exists), "unsupported" (a shape the
	// planner does not price: a granted ability, a mode it does not
	// compose, a play only an uncovered source could pay that the prefix
	// search did not reach ...; PotentialPlayScript is the exact fallback),
	// "search_limit" or "ambiguous" (another potential play shares this
	// one's Kind/Obj/Ability/Mode, so a seat keyed on those could not tell
	// them apart).
	Reason string
	Detail string
	// Script, when set (Reason ""), is a PREFIX the seat plays first: its
	// answers activate mana sources the planner census cannot price (Saruli
	// Caretaker's "{T}, tap an untapped creature", Wall of Roots' counter),
	// found by an exact search on engine clones. After the last step the
	// same query prices the play from the floating pool (a witness, or the
	// cast's payment action), and the seat pays it as usual.
	Script []ScriptStep
}

// PotentialPaymentPlans prices every potential play of p (the same walk and
// order as PotentialActions) with the exact source-exclusive planner:
//
//   - an ordinary cast (Mode "", no alternative cost) from the hand or the
//     command zone gets PlanCastPayment's verdict and witness (the one
//     Decision.PaymentActions offers); a shape it declines ({X}, hybrid,
//     an additional cost) is planned from its composed cost instead;
//   - a printed activated ability gets a witness for its offer-time mana
//     cost (the printed Cost$ with its own ReduceCost$ and the live
//     RaiseCost/ReduceCost statics, exactly abilityOfferCost's
//     composition), planned so that the ability's own tap, tapXType and
//     sacrifice parts stay payable (planComposedCost);
//   - a cast in a mode the cast planner does not witness (flashback,
//     bestow, kicker, may-play under its rider or at its permission's alternative
//     cost, plot ...) gets a witness from potentialModeCastPlan, confirmed by the
//     offer gate itself;
//   - every other play is "unsupported".
//
// A witness that sacrifices or returns a permanent is replayed on a clone
// (potentialWitnessReaches) before it is offered.
//
// An "insufficient" verdict is only reported as a proof: when the planner's
// source census covers every mana ability p could activate, or when a
// relaxation of the abilities it misses (paymentPlanRelaxProof) still
// cannot pay. Otherwise a play only those abilities could pay gets a
// scripted prefix when potentialPrefixScript finds one, and is left
// "unsupported" (census_incomplete) when not.
//
// A pure read: no event, no state write, no RNG draw. Determinism: zone-order
// walks only; the maps below are lookups.
func (e *Engine) PotentialPaymentPlans(p state.PlayerID) []PotentialPlan {
	return e.potentialPlans(p, potentialScriptPlays)
}

// EnsurePotentialProofs records on the pending priority decision which of its
// potential plays the planner proves unpayable, for PotentialActions to project
// (PotentialAction.Payable). A host calls it for a seat that reads
// potential_actions as what it can do: the walk prices every play, which a bot
// reading its own plans has no use for. Like EnsurePaymentActions it is a pure
// derived read that emits nothing, and it runs no prefix search: a play only a
// script could pay is not a proof either way.
func (e *Engine) EnsurePotentialProofs() {
	d := e.pending
	if d == nil || d.Kind != decision.KPriority || d.PotentialProofsBuilt {
		return
	}
	d.PotentialUnpayablePlays = d.PotentialUnpayablePlays[:0]
	for _, pp := range e.potentialPlans(d.Acting(), 0) {
		if pp.Reason == pay.ReasonInsufficient {
			d.PotentialUnpayablePlays = append(d.PotentialUnpayablePlays, pp.Action)
		}
	}
	d.PotentialProofsBuilt = true
}

// potentialPlans is PotentialPaymentPlans with at most scriptPlays prefix searches.
func (e *Engine) potentialPlans(p state.PlayerID, scriptPlays int) []PotentialPlan {
	if e.G.Over || int(p) < 0 || int(p) >= len(e.G.Players) {
		return nil
	}
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	defer pay.PaymentPlanQueryEnd(asPayer(e), e.paymentPlanQueryResumeBegin(p))
	hyp, opts := e.potentialWalkOf(p, true)
	// ambiguous[i]: another potential play shares opts[i]'s (kind, object,
	// ability, mode) identity (the default cast is not ambiguous with the
	// card's own alternative-cost cast, only that cast with it; nor is the
	// printed flashback instance with one granted beside it, which names its
	// cost). A granted or gained ability's Ability is 0 too,
	// and its SVar or gained source tells it from the printed ability it would
	// otherwise take off the surface. A pairwise scan over the walk's few plays
	// spares the per-query map.
	var ambiguousBuf [128]bool
	ambiguous := ambiguousBuf[:0]
	if len(opts) <= len(ambiguousBuf) {
		ambiguous = ambiguousBuf[:len(opts)]
	} else {
		ambiguous = make([]bool, len(opts))
	}
	for i := range opts {
		a := &opts[i]
		if !potentialPlayKind(a.Kind) {
			continue
		}
		for j := i + 1; j < len(opts); j++ {
			b := &opts[j]
			if a.Obj == b.Obj && a.Ability == b.Ability && a.Kind == b.Kind && a.Mode == b.Mode &&
				a.SVar == b.SVar && a.GainedSource == b.GainedSource {
				// The plain cast is priced on its own beside a card's own alternative
				// cost (Land Grant's reveal, Fireblast's sacrifice): it is
				// the cast of the printed mana cost, a different play with its
				// own plan, where the alternative-cost play shares its Action
				// with it and stays unpriced.
				if a.AltCostIndex == 0 && b.AltCostIndex > 0 {
					ambiguous[j] = true
				} else if a.AltCostIndex > 0 && b.AltCostIndex == 0 {
					ambiguous[i] = true
				} else if a.Cost == "" && b.Cost != "" {
					// The printed flashback instance is the default cast the
					// seat's option matcher selects; a further instance granted
					// beside it (CR 702.34a) names its own cost and stays
					// unpriced, exactly as an alternative cost does above.
					ambiguous[j] = true
				} else if a.Cost != "" && b.Cost == "" {
					ambiguous[i] = true
				} else {
					ambiguous[i], ambiguous[j] = true, true
				}
			}
		}
	}
	// Every ordinary cast verdict below names a plain cast the PotentialMana
	// walk above listed, so the planner shares one cost-static collection
	// and a priced candidate set across them (castPlanShare).
	share := castPlanShare{statics: costStaticSource{e: e}, candidates: paymentCastCandidates{e: e, p: p, priced: true}}
	censused := false // the census (and its relaxed alternatives) was read
	scripts := 0      // prefix searches run (potentialScriptPlays bounds them)
	var census paymentPlanCensus
	var out []PotentialPlan
	for oi, o := range opts {
		if !potentialPlayKind(o.Kind) {
			continue
		}
		pp := PotentialPlan{Action: decision.PotentialAction{Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode, Label: o.Label}}
		witness := false // the verdict's Plan is the play's witness (not Decision.PaymentActions')
		verdict := func() PaymentPlanOutcome {
			if ambiguous[oi] {
				return PaymentPlanOutcome{Reason: "ambiguous"}
			}
			got, w := e.potentialPlayVerdictShared(p, o, &share)
			witness = w
			return got
		}
		got := verdict()
		if got.Reason == "insufficient" {
			if !censused {
				census, censused = e.paymentPlanCensusOf(p, &hyp), true
			}
			if !census.complete {
				got = PaymentPlanOutcome{Reason: "unsupported", Detail: "census_incomplete"}
				if census.relaxable {
					// The census misses a source the planner cannot execute
					// (Saruli Caretaker's tapXType, Wall of Roots' counter,
					// Heap Gate's {1} fee). Re-ask with each such ability as
					// a free, relaxed alternative: a play no relaxed plan
					// pays is unpayable by any real sequence.
					if r := e.paymentPlanRelaxProof(census, verdict); r.Reason == "insufficient" {
						got = PaymentPlanOutcome{Reason: "insufficient", Detail: "relaxed_census"}
					}
				}
				if got.Reason != "insufficient" && scripts < scriptPlays && !e.potentialTargetPriced(p, o) {
					// Payable, if at all, only through an uncovered source:
					// search the activations of those sources on clones
					// for a prefix after which the planner pays the play.
					// The search slot is counted first, whether or not the
					// search runs below: dirac mirrors this search and its
					// slot accounting node for node, so skipping a search
					// must not move a later play into or out of the bound.
					scripts++
					// The bounded relaxation is a NECESSARY condition: it
					// over-produces every real activation sequence of the
					// census's uncovered sources (each unit is free, and a
					// repeatable tap source contributes as many as its
					// untapped candidates can pay for), so a play no bounded
					// plan pays cannot be paid by any prefix and the search
					// could only run to a dead end. It is used ONLY here, to
					// skip such a search: it is never reported as
					// "insufficient", nor does it feed the relaxed proof
					// above, because over-producing is not a proof of
					// unpayability. got stays unsupported/census_incomplete
					// exactly as before.
					skip := !census.relaxable && census.bounded &&
						e.paymentPlanRelaxProof(census.boundedProof(), verdict).Reason == "insufficient"
					if !skip {
						if script, ok := e.potentialPrefixScript(p, o, census.gaps, census.fees()); ok {
							pp.Script = script
							got = PaymentPlanOutcome{}
						}
					}
				}
			}
		}
		if witness && got.Plan != nil && got.Reason == "" && paymentPlanConsumes(*got.Plan) && !e.potentialWitnessReaches(p, o, *got.Plan) {
			// The witness sacrifices or returns a permanent the play
			// itself needed (its only target): not a way to pay it, though a
			// smaller X may pay without that permanent.
			planned := got
			rejectedX := e.planLastX
			got = e.potentialWitnessKeeping(p, o)
			if got.Plan == nil {
				if alt, ok := e.potentialWitnessSmallerX(p, o, rejectedX); ok {
					got = alt
				} else if e.witnessReachesMode(e, p, o, *planned.Plan, true) {
					// No colouring keeps every source, but the one that
					// pays by consuming one ends the game in p's favour.
					got = planned
				}
			}
		}
		pp.Reason, pp.Detail = got.Reason, got.Detail
		if witness && got.Plan != nil && got.Reason == "" {
			plan := decision.ClonePaymentPlan(*got.Plan)
			pp.Plan = &plan
		}
		out = append(out, pp)
	}
	return out
}

// potentialWitnessSmallerX is the witness for a plain {X} cast whose
// largest-X plan consumed a permanent the play itself needed (potentialPlans'
// guard): the next smaller X's plan, largest first, whose witness the offer
// walk still admits. x is the X the rejected witness paid; the X
// announcement offers every value up to a witness, so a smaller X loses
// nothing the witness's own payment does not consume.
func (e *Engine) potentialWitnessSmallerX(p state.PlayerID, o decision.Option, x int32) (PaymentPlanOutcome, bool) {
	if o.Kind != "cast" || o.Mode != "" || o.AltCostIndex != 0 || x <= 0 {
		return PaymentPlanOutcome{}, false
	}
	for c := x - 1; c >= 0; c-- {
		e.planXCap, e.planXCapOn = c, true
		got := e.potentialModeCastPlan(p, o)
		e.planXCapOn = false
		if got.Plan == nil || got.Reason != "" {
			continue
		}
		if !paymentPlanConsumes(*got.Plan) || e.potentialWitnessReaches(p, o, *got.Plan) {
			return got, true
		}
	}
	return PaymentPlanOutcome{}, false
}

// potentialWitnessKeeping is the witness for a cast whose planned witness ate
// what the cast needed: the largest X that keeps every permanent, or
// witness_unreachable when no plan does.
func (e *Engine) potentialWitnessKeeping(p state.PlayerID, o decision.Option) PaymentPlanOutcome {
	if o.Kind == "cast" {
		e.planKeepSources = true
		got := e.potentialModeCastPlan(p, o)
		e.planKeepSources = false
		if got.Plan != nil && got.Reason == "" && !paymentPlanConsumes(*got.Plan) {
			return got
		}
	}
	return PaymentPlanOutcome{Reason: "unsupported", Detail: "witness_unreachable"}
}

// paymentPlanConsumes reports whether some step of plan sacrifices or
// returns its source.
func paymentPlanConsumes(plan decision.PaymentPlan) bool {
	for _, a := range plan.Activations {
		if a.Consequence != nil && (a.Consequence.Sacrifice || a.Consequence.ReturnToHand) {
			return true
		}
	}
	return false
}

// potentialPlayVerdict is the planner's verdict on potential play o of p,
// before the census check: witness reports whether the outcome's Plan is
// a witness for the play (always, except an unsupported shape).
func (e *Engine) potentialPlayVerdict(p state.PlayerID, o decision.Option) (got PaymentPlanOutcome, witness bool) {
	return e.potentialPlayVerdictShared(p, o, nil)
}

// castPlanShare is the cast planner's per-query inputs shared across one
// PotentialPaymentPlans query's ordinary-cast verdicts: one lazy cost-static
// collection and a candidate set marked priced. Every query names a plain
// cast the query's own PotentialMana walk listed -- exactly the offer
// builder's priced set (paymentCastCandidates' priced argument), so
// membership is certain and the per-cast huge-pool walk PlanCastPayment
// would run is skipped (pricedCandidatesVerify runs it and panics on a
// miss). The statics are a pure read of the unchanged board, shared exactly
// as the offer builder shares them.
type castPlanShare struct {
	statics    costStaticSource
	candidates paymentCastCandidates
}

// potentialPlayVerdictShared is potentialPlayVerdict with the ordinary-cast
// verdict planned over sh (nil: PlanCastPayment's own fresh inputs). sh is
// only valid for an o listed by the PotentialMana walk at this state.
func (e *Engine) potentialPlayVerdictShared(p state.PlayerID, o decision.Option, sh *castPlanShare) (got PaymentPlanOutcome, witness bool) {
	switch {
	case o.Kind == "cast" && o.Mode == "" && o.AltCostIndex == 0:
		got := e.potentialCastVerdict(p, o.Obj, sh)
		if got.Reason == "unsupported" {
			// A shape the cast planner does not witness -- an {X} spell
			// (the largest X the sources pay), a hybrid one (each pip
			// resolved to a colour), an additional discard or announced
			// sacrifice, a target-dependent price -- planned from its
			// composed cost and confirmed by the offer gate.
			alt := e.potentialModeCastPlan(p, o)
			// A modal cast is planned at its printed cost, which is the
			// exact total only when a legal mode is free; a costlier
			// cheapest mode would be over-offered (potentialModalCastFreeMode).
			modalFree := got.Detail == "shape:modal_cost" && e.potentialModalCastFreeMode(p, o.Obj)
			if alt.Reason == "" || got.Detail == "cost:x" || got.Detail == "cost:hybrid" || e.manaSpentReaderComposed(o.Obj, got.Detail) ||
				got.Detail == "shape:target_dependent_cost" || modalFree || e.G.Obj(o.Obj) != nil && (len(altAddCostParts(e.G.Obj(o.Obj).Face())) != 0 || spellExtrasAreFixedTaps(e.G.Obj(o.Obj).Face()) || e.spellExtrasAreAnnouncedLife(p, o.Obj)) {
				// A witness always; a proof only for the shapes the
				// composition models exactly (the planner declined the
				// others for a reason the composition may not see).
				// A Spree or Tiered face's mode costs only add to the printed
				// cost (CR 702.172a/702.183a), so the composition's "insufficient" at
				// the printed cost is a proof only when some mode adds
				// nothing: otherwise the cast's cheapest legal total is higher
				// and a prefix that pays only the printed cost would offer a
				// cast no mode can pay. With a free mode the census and prefix
				// search reach a source the planner cannot price (a granted
				// any-colour mana ability, CR 601.2g).
				return alt, true
			}
		}
		// The cast planner's witness also rides Decision.PaymentActions.
		return got, true
	case o.Kind == "ability" && o.SVar == "" && o.Keyword == "" && o.GainedSource == 0 && o.AltCostIndex == 0:
		return e.planAbilityPayment(p, o.Obj, o.Ability), true
	case o.Kind == "cast" && (o.AltCostIndex == 0 || o.Mode == "mayplay"):
		return e.potentialModeCastPlan(p, o), true
	}
	return PaymentPlanOutcome{Reason: "unsupported", Detail: "shape"}, false
}

// spellExtrasAreFixedTaps reports a face whose spell ability adds nothing to the
// printed mana cost but fixed-count tapXType parts (Guardian of the Great Door's
// "tap four untapped artifacts, creatures, and/or lands"). planComposedCost models
// those exactly: it withholds each assignment of candidates from the mana plan
// and requires the plan to leave them untapped, so its verdict, proof included,
// stands where the cast planner declines the shape (CR 601.2g-h).
func spellExtrasAreFixedTaps(f *cards.Face) bool {
	if f == nil {
		return false
	}
	extras := pay.WithSpellAbilityExtras(f, Cost{})
	if len(extras.TapPermanent) == 0 {
		return false
	}
	for _, part := range extras.TapPermanent {
		if part.Dyn != "" || part.N <= 0 || part.MinPower > 0 {
			return false
		}
	}
	rest := extras
	rest.TapPermanent = nil
	return rest.IsZero()
}

// spellExtrasAreAnnouncedLife reports a cast whose face's spell ability adds nothing to
// the printed mana cost but an announced PayLife<X> (Toxic Deluge's "As an
// additional cost, pay X life"). X is the caster's announcement (CR 601.2b,
// 107.3), so X = 0 is always payable and the cast costs exactly its mana: the
// composition's verdict, proof included, stands where the cast planner declines
// the shape, and an insufficient one goes on to the prefix search like any other
// cast's when the census misses a source (a filter land, CR 605.1a).
func (e *Engine) spellExtrasAreAnnouncedLife(p state.PlayerID, id state.ObjID) bool {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false
	}
	// an X a SVar fixes is converted to fixed life (FixLifeXCost); only the
	// announced one keeps its LifeX part
	extras, ok := pay.FixLifeXCost(asPayer(e), p, id, pay.WithSpellAbilityExtras(o.Face(), Cost{}))
	if !ok || len(extras.LifeX) == 0 {
		return false
	}
	rest := extras
	rest.LifeX = nil
	return rest.IsZero()
}

// manaSpentReaderComposed reports a spell the cast planner declined only
// because the mana it spends is read (its own Count$CastTotalManaSpent or
// Converge, or a reader on any battlefield): reading the spend changes no
// price, so the composition prices its mana exactly as it does any other spell
// -- {X} (Mockingbird) or not (Deadly Embrace beside the other player's Tellah).
// An insufficient verdict then stands as a proof and, through a source the
// census cannot price (a filter land, a granted any-colour ability), goes on to
// the prefix search. The shape gate tests Escalate and Strive after the
// reader, so a spell with either is still left to the cast planner's verdict.
func (e *Engine) manaSpentReaderComposed(id state.ObjID, detail string) bool {
	if detail != "shape:mana_spent_reader" {
		return false
	}
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false
	}
	_, escalate := o.Face().KeywordParam("Escalate")
	_, strive := o.Face().KeywordParam("Strive")
	return !escalate && !strive
}

// faceModalFloorZero reports a Spree or Tiered face with a mode that adds
// nothing, so the cast's cheapest legal total is its printed cost (CR
// 702.172a/702.183a). A mode whose ModeCost$ is printed but unparseable is withheld by
// the mode gate, so it is no free mode here.
func faceModalFloorZero(f *cards.Face) bool {
	if f == nil || !faceHasModeCost(f) {
		return false
	}
	sa := f.SpellAbility()
	if sa == nil {
		return false
	}
	for _, name := range effects.CharmOf(sa).Modes {
		c, present, ok := modeCost(f, name)
		if present && !ok {
			continue
		}
		if !present || c.CMC() == 0 && !c.HasNonMana() {
			return true
		}
	}
	return false
}

// relaxedPaid is a missed paid ability's relaxed alternatives and fee.
type relaxedPaid struct {
	alts []pay.Alt
	fee  int32
}

// relaxedPaidLimit bounds the paid missed abilities a relaxed proof
// enumerates subsets of (2^n proof runs).
const relaxedPaidLimit = 3

// paymentPlanRelaxProof runs verdict with the census's relaxed alternatives
// appended to every planner search: the free ones always, and the paid ones
// (filters) for each subset in turn, with that subset's fees charged as
// generic on top of the play's cost. It returns "insufficient" only when
// every run is: whatever set of paid abilities a real payment uses, the run
// for that set already covers it. A relaxed alternative is never
// executable, so no Plan is returned.
func (e *Engine) paymentPlanRelaxProof(c paymentPlanCensus, verdict func() PaymentPlanOutcome) PaymentPlanOutcome {
	prev, prevFee := e.PaymentPlanRelaxed, e.PaymentPlanRelaxedFee
	defer func() { e.PaymentPlanRelaxed, e.PaymentPlanRelaxedFee = prev, prevFee }()
	for mask := 0; mask < 1<<len(c.paid); mask++ {
		relaxed := slices.Clone(c.relaxed)
		fee := int32(0)
		for i, pd := range c.paid {
			if mask>>i&1 == 1 {
				relaxed = append(relaxed, pd.alts)
				fee += pd.fee
			}
		}
		if len(relaxed) == 0 {
			relaxed = [][]pay.Alt{nil} // the census alone, still a relaxed run
		}
		e.PaymentPlanRelaxed, e.PaymentPlanRelaxedFee = relaxed, fee
		if got := verdict(); got.Reason != "insufficient" {
			return PaymentPlanOutcome{Reason: got.Reason, Detail: got.Detail}
		}
	}
	return PaymentPlanOutcome{Reason: "insufficient"}
}

// potentialModeCastPlan prices a cast the cast planner does not witness --
// an alternative or additional-cost mode (flashback, bestow, kicker,
// buyback, entwine, surge, evoke, dash, overload, warp, madness, miracle)
// or an {X} spell -- and returns a witness for its mana part.
//
// The cost is composed exactly as beginCast charges it for the mode (the
// same per-mode helper, fixLifeXCost, then offerCostFor's CR 601.2f
// modifiers and commander tax), with an {X} folded in before the modifiers
// as manaToPay does. An {X} spell's witness pays the LARGEST X its sources
// can fund (the X announcement then offers every value up to it, so no X is
// lost); an X the sources cannot pay even at its minimum is insufficient.
// Convoke, improvise and delve are OPTIONAL credits (CR 702.51a, 702.66a)
// announced after the mode's cost is chosen. An alternative-cost keyword
// mode (altcast_modes.go: evoke, dash, warp, impending, ...) replaces the
// printed cost with a keyword cost whose offer gate prices no credit, so it
// is planned at that raw cost WITHOUT the credit: a cast that lands payable
// that way is offered, and the commitment stays with convokeAsk. When even
// the no-credit payment is insufficient the old non-proof verdict
// ("unsupported", "cost:credit") stands: the credit may still pay it.
//
// Every witness is then checked against the offer gate itself: the play
// must be offered by the potential walk priced with exactly the floating
// pool plus the witness's production. A composition that disagreed with the
// engine's gate is therefore "unsupported", never a wrong witness.
func (e *Engine) potentialModeCastPlan(p state.PlayerID, o decision.Option) (out PaymentPlanOutcome) {
	xCap, xCapOn := e.planXCap, e.planXCapOn
	e.planXCapOn = false
	e.planLastX = 0
	id := o.Obj
	obj := e.G.Obj(id)
	if obj == nil || obj.Face() == nil {
		return PaymentPlanOutcome{Reason: "unsupported"}
	}
	f := obj.Face()
	// A convoke/improvise/delve credit is an optional announcement that
	// reprices the cast. A printed-cost cast keeps the cast planner's own
	// answer: its offer base commits the credit (castOfferBase) and that
	// planner declines the credit-bearing shape (shape:contribution), so the
	// mode witness must not second-guess it. An alternative-cost keyword
	// mode charges the cost its offer gate prices WITHOUT the credit, so
	// plan that no-credit payment and keep the old unknown verdict -- never
	// a proof -- when it fails.
	credit := !e.planIgnoreCredit && (e.hasCastConvoke(id) || e.hasCastImprovise(id) || e.hasCastDelve(id, false))
	if credit {
		if m := altCastFor(o.Mode); m == nil || !m.potential {
			return PaymentPlanOutcome{Reason: "unsupported", Detail: "cost:credit"}
		}
		defer func() {
			if out.Plan == nil && out.Reason == "insufficient" {
				out = PaymentPlanOutcome{Reason: "unsupported", Detail: "cost:credit"}
			}
		}()
	}
	base, ok := e.potentialModeBaseCost(p, id, f, o)
	if !ok {
		return PaymentPlanOutcome{Reason: "unsupported", Detail: "shape"}
	}
	if base, ok = pay.FixLifeXCost(asPayer(e), p, id, base); !ok {
		return PaymentPlanOutcome{Reason: "unsupported", Detail: "cost:life_x"}
	}
	scope := spellScope(o.Mode)
	if pay.CostAnnouncesSacX(base) || !e.planIgnoreCredit && e.costModifiers(p, id, scope).Waterbend > 0 {
		// An announced Sac<X> or a waterbend credit reprices the cast by
		// what the payment taps or sacrifices: not composed here.
		return PaymentPlanOutcome{Reason: "unsupported", Detail: "cost:credit"}
	}
	// An alternate-face mode casts with the object flipped to that face, so
	// its CR 601.2f modifiers must be composed there too: a face-dependent
	// tax (Thalia's ValidCard$ Card.nonCreature) otherwise prices the live
	// face's cost, the offer gate prices the cast face's, and the witness is
	// rejected -- the cast is never offered from untapped sources.
	castFace := potentialModeCastFace(obj, o.Mode)
	compose := func(c Cost) (Cost, bool) {
		if castFace == nil {
			return e.offerCostFor(p, id, c, scope), true
		}
		var composed Cost
		ok := e.offerAsFace(id, castFace, func() bool {
			composed = e.offerCostFor(p, id, c, scope)
			return true
		})
		return composed, ok
	}
	var rider pipRider
	plan := func(x int32) PaymentPlanOutcome {
		c := base
		if base.X != 0 {
			c = base.WithX(x)
		}
		composed, ok := compose(c)
		if !ok {
			return PaymentPlanOutcome{Reason: "unsupported", Detail: "shape"}
		}
		// a RestrictValid$ source admits the payment by the cast face's
		// characteristics (The Ringhart Crest's "legendary creature
		// spell" mana cannot pay its own back face), the rest of the plan
		// reads the board as it is
		prev := e.planFace
		e.planFace = castFace
		defer func() { e.planFace = prev }()
		got := e.planComposedCost(p, id, pay.PlanManaUnderRider(composed, rider), false, false)
		if e.planKeepSources && got.Plan != nil && paymentPlanConsumes(*got.Plan) {
			return PaymentPlanOutcome{Reason: "insufficient"}
		}
		return got
	}
	lo := base.XMin
	best := plan(lo)
	if best.Reason == "insufficient" && (o.Mode == "mayplay" || castModeCodes.Code(o.Mode) == castModeBargained) {
		// The grant may let any mana pay what the printed pips cannot
		// (MayPlayIgnoreColor$, MayPlayIgnoreType$), as the cast's payment reads it.
		rider = asEval(e).MayPlayRider(p, id)
		best = plan(lo)
	}
	if conv := pay.ConvRider(asEval(e).Conv(p, id, pay.SASpell)); best.Reason == "insufficient" && conv != (pipRider{}) {
		// So may an AnyType->AnyType conversion (Vizier of the Menagerie, CR 609.4b).
		rider = rider.Or(conv)
		best = plan(lo)
	}
	if best.Reason == "insufficient" {
		statics := e.collectCostStatics()
		if e.paymentPlanTargetDependentFor(statics, p, id, scope) {
			// A ValidTarget$ cost static may price this cast lower for some
			// target set (CR 601.2f reads the chosen targets; offerCastable's
			// second pass). The target ask tests each candidate as the sole
			// selection, which does not price a mode cast choosing several, so
			// one of those stays unknown; a plain cast's cheapest set is
			// priced (potentialTargetPricedPlan), one own permanent enough
			// for a flat "if it targets ..." reduction on a TargetMax$ 2
			// spell exactly as on a single-target one.
			if o.Mode != "" || o.AltCostIndex != 0 {
				if sa := pay.CostTargetingSA(e.G, id, scope); sa != nil {
					if _, max := targetBounds(sa); max > 1 {
						return PaymentPlanOutcome{Reason: "unsupported", Detail: "shape:target_dependent_cost"}
					}
				}
			}
			return e.potentialTargetPricedPlan(statics, p, o, base, scope, rider)
		}
	}
	if best.Plan == nil || best.Reason != "" || len(e.PaymentPlanRelaxed) != 0 {
		// No witness, or a relaxed proof run (which reads only whether the
		// minimum is payable, and whose plans are never executed).
		return best
	}
	if base.X != 0 {
		// Payability is monotone in X: binary-search the largest payable X
		// up to what the floating pool and every census source together
		// could fund (the census bounds a witness; PotentialMana's bound may
		// be unbounded).
		hi := e.paymentPlanCensusTotal(p) / int32(base.X)
		if xCapOn {
			if xCap < lo {
				return PaymentPlanOutcome{Reason: "unsupported", Detail: "cost:x"}
			}
			if xCap < hi {
				hi = xCap
			}
		}
		bestX := lo
		for lo < hi {
			mid := lo + (hi-lo+1)/2
			got := plan(mid)
			if got.Plan != nil && got.Reason == "" {
				best, lo, bestX = got, mid, mid
			} else {
				hi = mid - 1
			}
		}
		if best.Plan != nil && best.Reason == "" {
			e.planLastX = bestX
		}
	}
	if parts := altAddCostParts(f); len(parts) != 0 && o.Mode == "" && best.Plan != nil && best.Reason == "" {
		if got, sacrificeOnly := e.potentialAltAddSacrificePlan(p, o, base, parts); sacrificeOnly {
			return got
		}
	}
	if e.planWitnessOffersNeeded(f, *best.Plan) && !e.potentialWitnessOffers(p, o, *best.Plan) {
		// An either-or additional cost (AlternateAdditionalCost) rides a
		// may-play cast too (CR 118.9d: the permission replaces the mana
		// cost only), so the may-play arm is re-priced beside its parts
		// exactly as the plain cast's is below; the offer gate (mayPlayAffordable)
		// proves at least one part payable, and beginCast's may-play ask
		// folds the chosen one (CR 601.2b). planWitnessOffersNeeded keeps the
		// offer walk on every cast with parts, so this arm stays reachable.
		if parts := altAddCostParts(f); len(parts) != 0 && (o.Mode == "" || o.Mode == "mayplay") {
			return e.potentialAltAddPlan(p, o, base, parts)
		}
		return PaymentPlanOutcome{Reason: "unsupported", Detail: "witness_not_offered"}
	}
	return best
}

// potentialTargetPriced reports whether o is a cast a ValidTarget$ cost static may
// price lower for some target. No prefix is searched for one: the search replays the
// play's price, and a target's is not known until the targets are.
func (e *Engine) potentialTargetPriced(p state.PlayerID, o decision.Option) bool {
	return o.Kind == "cast" && e.paymentPlanTargetDependentFor(e.collectCostStatics(), p, o.Obj, spellScope(o.Mode))
}

// potentialTargetPricedPlan is a witness for a cast whose ValidTarget$ cost
// statics price it lower for some legal target set, tried one assignment at a
// time (potentialCostModsUsing), or "insufficient" when no set's price is
// payable. The target ask then keeps the targets the mana floated can pay for.
func (e *Engine) potentialTargetPricedPlan(statics costStaticViews, p state.PlayerID, o decision.Option, base Cost, scope costScope, rider pipRider) PaymentPlanOutcome {
	// A consumption-only query reads the plans only for whether one pays
	// and what it consumes, so when even the cheapest price a target
	// assignment can announce is beyond the option set's cover, no target's
	// plan pays and the enumeration is skipped (planTargetCostFloor,
	// planCoverInsufficient). The floor subtracts a matching reduction's
	// literal amount ONCE, which bounds a single-target declaration; a spell
	// that may choose several targets can apply a per-target reduction more
	// than once (or the cheapest legal set may announce fewer than the
	// maximum), so its floor is not a lower bound and the enumeration runs.
	if e.planConsumptionOnly {
		sa := pay.CostTargetingSA(e.G, o.Obj, scope)
		singleTarget := sa == nil
		if !singleTarget {
			_, max := targetBounds(sa)
			singleTarget = max <= 1
		}
		if singleTarget {
			if floor, ok := e.planTargetCostFloor(statics, p, o.Obj, scope, base); ok &&
				e.planCoverInsufficient(p, pay.PlanManaUnderRider(floor, rider)) {
				return PaymentPlanOutcome{Reason: "insufficient"}
			}
		}
	}
	got := PaymentPlanOutcome{Reason: "insufficient"}
	_, got = e.targetPricedPlan(statics, p, o.Obj, base, scope, rider)
	if got.Plan != nil && got.Reason == "" && len(e.PaymentPlanRelaxed) == 0 &&
		e.planWitnessOffersNeeded(e.G.Obj(o.Obj).Face(), *got.Plan) && !e.potentialWitnessOffers(p, o, *got.Plan) {
		return PaymentPlanOutcome{Reason: "unsupported", Detail: "witness_not_offered"}
	}
	return got
}

// targetPricedPlan composes the cheapest payable legal target assignment of a
// cast whose ValidTarget$ cost statics price it per target set (CR 601.2f),
// returning the composed cost and its mana plan. It is the enumeration both
// the potential walk's witness (potentialTargetPricedPlan) and the planned
// target-paid cast (planCastPaymentChecked, ValidateCastPayment) read. The
// caller decides whether the plan is offered: the potential walk lowers it
// before the cast, while a planned cast pays it at CR 601.2g, after the
// targets are chosen.
func (e *Engine) targetPricedPlan(statics costStaticViews, p state.PlayerID, id state.ObjID, base Cost, scope costScope, rider pipRider) (Cost, PaymentPlanOutcome) {
	var composed Cost
	got := PaymentPlanOutcome{Reason: "insufficient"}
	e.potentialCostModsUsing(statics, p, id, scope, e.costPotentialTargets(p, id, scope), 0, func(m costMods) bool {
		var c Cost
		e.composedOfferCostInto(&c, p, id, &base, &m, scope)
		out := e.planComposedCost(p, id, pay.PlanManaUnderRider(c, rider), false, false)
		if out.Plan != nil && out.Reason == "" {
			composed, got = c, out
			return true
		}
		got = out
		return false
	})
	return composed, got
}

// planPoolCoversPrintedCost reports whether the floating pool alone pays the
// cast's composed printed cost, mirrored from potentialModeCastPlan's own
// composition up to its first planComposedCost: the same base, the same
// declines it answers unsupported, the same offerCostFor compose (the pip
// rider is zero there) and the same plain-mana reading. Such a cost is
// payable without activating a single source: the search is handed the pool
// before any census choice, so its zero-source plan is the rank-minimal one
// (no irreversible cost, no creatures, no sources -- keys 1-3 -- and the
// lowest surplus, key 4), paymentPlanConsumes reads nothing on it, and the
// consumption-only arm's verdict is false without the plan or the census.
// Anything the mirror cannot cheaply decide returns false and the planner
// plans (fail closed).
func (e *Engine) planPoolCoversPrintedCost(p state.PlayerID, o decision.Option) bool {
	id := o.Obj
	obj := e.G.Obj(id)
	if obj == nil || obj.Face() == nil {
		return false
	}
	base, ok := e.potentialModeBaseCost(p, id, obj.Face(), o)
	if !ok || base.X != 0 {
		return false
	}
	if base, ok = pay.FixLifeXCost(asPayer(e), p, id, base); !ok || base.X != 0 {
		return false
	}
	if pay.CostAnnouncesSacX(base) {
		return false // the planner declines the shape; let it answer
	}
	castFace := potentialModeCastFace(obj, o.Mode)
	var composed Cost
	if castFace == nil {
		composed = e.offerCostFor(p, id, base, spellScope(o.Mode))
	} else if faceOK := e.offerAsFace(id, castFace, func() bool {
		composed = e.offerCostFor(p, id, base, spellScope(o.Mode))
		return true
	}); !faceOK {
		return false
	}
	if len(composed.Sac) != 0 || len(composed.Discard) != 0 || len(composed.TapPermanent) != 0 {
		return false
	}
	if pay.PlanCostDetail(pay.PlanManaHalf(composed)) != "" {
		return false
	}
	pl := &e.G.Players[p]
	if len(pl.RestrictedMana) != 0 {
		return false // a restricted unit's spendability is the payment's own ask
	}
	pool := pl.Pool
	need := int32(0)
	for c := range pool {
		if composed.Colored[c] > pool[c] {
			return false
		}
		need += composed.Colored[c]
	}
	return pool.Total()-need >= composed.Generic
}

// planTargetCostFloor is the cheapest mana cost one target assignment of id
// can announce: base less every target-dependent ReduceCost static's literal
// generic amount, the reduction one target's match applies once (the
// potential-walk path only reaches here for a cast announcing at most one
// target). ok is false, and the caller plans, whenever a target-dependent
// part is not such a subtraction -- a count-reading amount, a SetCost -- or
// when a floor would charge a reduction no target can apply; target
// raises and optional parts only ever add, so the floor may ignore them.
func (e *Engine) planTargetCostFloor(statics costStaticViews, p state.PlayerID, id state.ObjID, scope costScope, base Cost) (Cost, bool) {
	floor := base
	for _, sv := range statics.reduce {
		if id != sv.Source && sv.ParamStr(cards.PKValidCard) == "Card.Self" {
			continue // another cast's self static never prices this one
		}
		if sv.ParamStr(cards.PKValidTarget) == "" {
			if costStaticReadsTargets(sv) {
				// A target-reading amount evaluated at nil targets reads
				// zero, so base does NOT already carry its reduction when
				// the gate passes: only a target-independent denial proves
				// the static never applies. Refuse otherwise, exactly as
				// paymentPlanTargetDependentFor declines the cast
				// (ok || !indepFail); continuing on ok would floor the price
				// above a real per-target price and could miss a consuming
				// plan.
				if _, indepFail := e.costStaticGate(sv, "ReduceCost", p, id, scope, nil, false); indepFail {
					continue // never applies
				}
				return Cost{}, false // a target-reading amount's floor is no subtraction
			}
			continue // priced into base already (or another card's static)
		}
		if ok, indepFail := e.costStaticGate(sv, "ReduceCost", p, id, scope, nil, false); ok || indepFail {
			continue // priced into base already, or never applies
		}
		amt := parseAmount(sv.ParamStr(cards.PKAmount), -1)
		if amt < 0 {
			return Cost{}, false
		}
		floor.Generic -= amt
	}
	for _, sv := range statics.set {
		if sv.ParamStr(cards.PKValidTarget) != "" || costStaticReadsTargets(sv) {
			return Cost{}, false
		}
	}
	return floor, true
}

// planWitnessOffersNeeded reports whether plan's offer gate
// (potentialWitnessOffers) must run. A consumption-only query
// (planConsumptionOnly, targetPaidNeedsPlannedPayment) reads its plan only
// for whether it consumes something: a plan that consumes nothing is
// declined by the reader itself, so the gate cannot change the answer --
// except on a cast with either-or additional cost parts, whose witness
// fallback (potentialAltAddPlan) the gate's failure arms, and never for the
// keep-sources plan (potentialWitnessKeeping), whose own consumption filter
// the gate's failure would flip.
func (e *Engine) planWitnessOffersNeeded(f *cards.Face, plan decision.PaymentPlan) bool {
	if !e.planConsumptionOnly || e.planKeepSources || paymentPlanConsumes(plan) {
		return true
	}
	if f == nil {
		return true // fail open: an unknown face keeps the gate
	}
	return len(altAddCostParts(f)) != 0
}

// potentialAltAddSacrificePlan prices an either-or additional cost none of
// whose mana-free parts is free of a sacrifice (Dusk Rose Reliquary's artifact
// or creature; Stir Up Trouble's, beside its {4}) with the sacrifice beside the
// spell's mana, as an ordinary sacrifice cost is priced: the permanent given up
// is not also a source of the mana that pays the spell (CR 601.2b, 601.2g,
// 601.2h). decided is false when a part that costs neither mana nor a
// sacrifice exists, so the mana witness alone stands for that part.
func (e *Engine) potentialAltAddSacrificePlan(p state.PlayerID, o decision.Option, base Cost, parts []string) (PaymentPlanOutcome, bool) {
	var sacs []Cost
	for _, part := range parts {
		c := ParseCost(part)
		if c.HasManaPayment() {
			continue
		}
		if len(c.Sac) == 0 {
			return PaymentPlanOutcome{}, false
		}
		sacs = append(sacs, c)
	}
	for _, c := range sacs {
		got := e.planComposedCost(p, o.Obj, e.offerCostFor(p, o.Obj, base.Plus(c), spellScope(o.Mode)), false, false)
		if got.Plan != nil && got.Reason == "" && e.potentialWitnessOffers(p, o, *got.Plan) {
			return got, true
		}
	}
	return e.potentialAltAddPlan(p, o, base, parts), true
}

// potentialAltAddPlan is a witness for an either-or additional cost (CR 601.2f)
// when the spell's own mana floated pays no part in full: its mana with a part
// that has mana of its own. "insufficient" when no part is payable.
func (e *Engine) potentialAltAddPlan(p state.PlayerID, o decision.Option, base Cost, parts []string) PaymentPlanOutcome {
	for _, part := range parts {
		c := ParseCost(part)
		if !c.HasManaPayment() {
			continue
		}
		got := e.planComposedCost(p, o.Obj, e.offerCostFor(p, o.Obj, base.Plus(c), spellScope(o.Mode)), false, false)
		if got.Plan != nil && got.Reason == "" && e.potentialWitnessOffers(p, o, *got.Plan) {
			return got
		}
	}
	return PaymentPlanOutcome{Reason: "insufficient"}
}

// paymentPlanCensusTotal is the most mana a witness can hold: p's floating
// pool plus, per census source, its largest alternative.
func (e *Engine) paymentPlanCensusTotal(p state.PlayerID) int32 {
	total := e.G.Players[p].Pool.Total()
	for _, u := range pay.PaymentPlanQueryUnits(asPayer(e), p) {
		best := int32(0)
		for _, a := range pay.PaymentPlanQueryAlternatives(asPayer(e), u) {
			best = max(best, a.Mana.Total())
		}
		total += best
	}
	return total
}

// planCoverInsufficient reports whether the planner's option set provably
// cannot fund mana: a plan spends the floating pool plus, per census source,
// one of its query alternatives (a plan activates each census source once,
// the bound paymentPlanCensusTotal reports a witness by), so a per-colour
// bound over pool and alternatives that cannot cover the pips covers no
// plan. What narrows the planner never widens it -- a RestrictValid$
// alternative the payment refuses, a restricted pool batch, a ManaConvert
// static the ordinary solver does not apply -- so the bound stays a
// superset of what any plan may spend. Only a plain Colored/Generic cost is
// read: any shape the planner declines (pay.PlanCostDetail) returns false
// and the planner plans.
func (e *Engine) planCoverInsufficient(p state.PlayerID, mana Cost) bool {
	if pay.PlanCostDetail(mana) != "" {
		return false
	}
	bound, _ := e.planCoverBoundOf(p)
	need := int32(0)
	for c := range bound {
		if mana.Colored[c] > bound[c] {
			return true
		}
		need += mana.Colored[c]
	}
	return bound.Total()-need < mana.Generic
}

// planBoardConsumesNothing reports, without the census walk, that no
// alternative the payment census could list for p consumes its source
// (planCoverConsumable), so the consumption-only arm's verdict is false
// whatever the cast's cost: with no consumable alternative, no plan the
// search can return passes paymentPlanConsumes. The census's alternatives
// carry Sacrifice/ReturnToHand only from the ability's own shape
// (paymentPlanAbilityShapeTier; the interference merge adds damage, life and
// no-untap, never a consume), so the scan reads the same per-ability verdict
// the census's alternative derivation reads, over a superset of the census's
// enumeration: every printed mana or ManaReflected ability on every
// permanent p controls, all faces of a mutated pile included, with none of
// the walk's own gates (zone, loss, tap-sick, payability) applied. The
// gates and the census's other arms only ever REMOVE abilities or
// alternatives -- the CR 305.6 intrinsics (facedown type words and
// layer-4-set basic land types) are plain taps whose production never
// consumes -- so the superset scan can only over-report a consuming ability
// and fail open. It fails open, too, whenever a granted ability could exist
// (active summary hasGrants: AddAbilities and GainedFaces entries, which is
// also every runtime grant the walk's grantedAbilities arm lists), whenever
// a raise/reduce/set static may reach a mana ability (anyManaReaching: the
// memoised shape facts are the unmodified costs', while the census then
// classifies the repriced cost per call), and for any ability whose shape
// the configured facts do not already know (shapeKnown false, or facts nil:
// the census's rider arms can disclose a self-return), which is the census's
// own unconfigured path. planCoverBoundOf computes the same verdict from the
// census; this scan is the pre-gate that answers the common no-consumption
// board without the census pass the census-only path pays once per
// consumption-only query.
func (e *Engine) planBoardConsumesNothing(p state.PlayerID) bool {
	if e.anyManaReachingCostStatic() {
		return false
	}
	if e.activeSummaryOf(e.active()).hasGrants {
		return false
	}
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		// A lost-abilities or face-down permanent's printed abilities are
		// not live (CR 613.1f, CR 708.5); scanning them can only
		// over-report, so the ordinary face loop needs no loss read.
		for i := 0; i < o.PileFaceCount(); i++ {
			pf, ok := o.PileFaceAt(i)
			if !ok {
				continue
			}
			for _, ma := range pf.Face.Abilities {
				if ma.API != "Mana" && ma.API != "ManaReflected" {
					continue
				}
				if f := e.manaFactsOf(ma); f != nil && f.shapeKnown {
					if f.shapeCons.Sacrifice || f.shapeCons.ReturnToHand {
						return false
					}
					continue
				}
				// The census classifies this ability per call, including
				// the rider arms a self-return rider takes; this scan
				// cannot answer it without the census's reads, so it
				// defers to the census.
				tier, c, _, rider := pay.PaymentPlanShapeTierOf(ma, e.abilityCompiledCost(ma).Cost)
				if rider || tier == pay.TierLastResort &&
					(c.Sacrifice || c.ReturnToHand) {
					return false
				}
			}
		}
	}
	return true
}

// planCoverBoundOf is the bound planCoverInsufficient reads, with whether a
// census alternative consumes its source (planCoverConsumable: sacrifices or
// returns it). The plan the consumption-only query reads is the mana part
// alone -- the fixed-count sacrifice parts and the either-or modes are the
// play's own asks, outside the plan -- and a relaxed proof's alternatives
// are never returned as a plan, so the plan's search draws its steps only
// from this census's alternatives (PlanPaymentCostWithout's choices). With
// no consumable alternative no plan paymentPlanConsumes reads can consume,
// whatever the cost, so the arm's verdict is false without the search.
// The bound itself is p's floating pool plus,
// per census source, its largest alternative per colour. Inside the arm's
// consumption-only query (planCoverBoundOn) both are computed once per
// event-log head and shared by the query's nested planComposedCost calls:
// every state change is an appended event, so an unchanged log length is the
// same unchanging board and pool.
func (e *Engine) planCoverBoundOf(p state.PlayerID) (state.Mana, bool) {
	pool := e.G.Players[p].Pool
	if e.planCoverBoundOn && e.planCoverLogLen == len(e.L.Events) &&
		e.planCoverBoundP == p && e.planCoverPool == pool {
		return e.planCoverBound, e.planCoverConsumable
	}
	// The census read is a pure reader of p's posed decision
	// (paymentPlanQueryResumeBegin): when the offer builder's own query
	// scope for this state was kept (paymentPlanQueryKeep), its census and
	// alternatives are served instead of recomputed, and a census the arm's
	// bound fills first is shared with the builder's ordinary-cast plans.
	defer pay.PaymentPlanQueryEnd(asPayer(e), e.paymentPlanQueryResumeBegin(p))
	bound := pool
	consumable := false
	for _, u := range pay.PaymentPlanQueryUnits(asPayer(e), p) {
		var best state.Mana
		for _, a := range pay.PaymentPlanQueryAlternatives(asPayer(e), u) {
			consumable = consumable || a.Consequence.Sacrifice || a.Consequence.ReturnToHand
			for c := range best {
				if a.Mana[c] > best[c] {
					best[c] = a.Mana[c]
				}
			}
		}
		for c := range bound {
			bound[c] += best[c]
		}
	}
	if e.planCoverBoundOn {
		e.planCoverLogLen = len(e.L.Events)
		e.planCoverBoundP = p
		e.planCoverPool = pool
		e.planCoverBound = bound
		e.planCoverConsumable = consumable
	}
	return bound, consumable
}

// potentialModalCastFreeMode reports whether a plain Spree/Tiered cast of id
// has a legal mode whose ModeCost$ is zero. Only then is the printed-cost
// plan an exact witness for a cast the shape gate declines: the cheapest
// mode adds nothing, so the plan's total is the cast's minimum total. A
// modal cast whose cheapest legal mode costs extra must stay unpriced -- a
// plan at the printed cost alone would price a cast no mode can pay (the OTJ
// Explosive Derailment divergence, where both modes cost {2}).
//
// The target check mirrors castModeAsk's first filter (modeTargetsAvailable
// with X pending, since X is announced after the modes), so a mode the cast
// could never select does not count. The first free legal mode wins; mode
// order is deterministic.
func (e *Engine) potentialModalCastFreeMode(p state.PlayerID, id state.ObjID) bool {
	obj := e.G.Obj(id)
	if obj == nil {
		return false
	}
	f := obj.Face()
	if f == nil || !e.faceScanHas(f, faceScanModalCost) {
		return false
	}
	sa := f.SpellAbility()
	cp := effects.CharmOf(sa)
	if !cp.HasChoices {
		return false
	}
	for _, name := range cp.Modes {
		if name == "" {
			continue
		}
		if sub := cards.ResolveSVar(f.SVars, name); sub == nil || !modeTargetsAvailable(e, p, id, id, sub, 0, true) {
			continue
		}
		mc, present, ok := modeCost(f, name)
		if present && !ok {
			continue // an unparseable mandatory cost is never free
		}
		if !present || mc.CMC() == 0 {
			return true
		}
	}
	return false
}

// potentialModeBaseCost is the raw (pre-modifier) cost beginCast charges a
// cast of id in mode, for the modes potentialModeCastPlan prices; ok is
// false for any other mode.
func (e *Engine) potentialModeBaseCost(p state.PlayerID, id state.ObjID, f *cards.Face, o decision.Option) (Cost, bool) {
	cost := pay.RawBaseCost(asPayer(e), p, id)
	mode := o.Mode
	code := potentialModeBaseCostCodes.Code(string(mode))
	if castModeCodes.Code(string(mode)) == castModeBargained {
		// A bargained cast prices like the may-play cast it may ride: the
		// printed cost plus any grant raise (Bargain adds only a sacrifice).
		code = potentialModeBaseCostMayplay
	}
	if strings.HasPrefix(mode, "sneaked_grant_") {
		// A granted Sneak instance is its own offer mode. beginCast keeps the
		// ORIGINAL mode in selectedMode while it canonicalises only opt.Mode
		// for dispatch, and its castModeSneak arm charges the sneakCosts entry
		// whose mode is selectedMode -- so the witness must read that same
		// instance, not the first (printed) one.
		code = potentialModeBaseCostSneak
	}
	switch code {
	case potentialModeBaseCostEmpty:
		return pay.WithSpellAbilityExtras(f, cost), true
	case potentialModeBaseCostMayplay:
		// A may-play grant (an impulse draw's exile): the printed cost, or
		// none, plus the grant's own raise -- the same read beginCast makes.
		free, raise, hasRaise, priced := e.mayPlayPermFreeRaise(p, id, o.MayPlayPerm)
		if free {
			cost = Cost{}
		}
		if o.AltCostIndex > 0 {
			// The permission's own alternative (MayPlayAltManaCost$), as beginCast
			// reads it: it replaces the cost, and the raise still applies.
			alts := e.mayPlayCastAlts(p, id, e.mayPlayPlainGrant(p, id))
			if o.AltCostIndex > len(alts) {
				return Cost{}, false
			}
			cost = alts[o.AltCostIndex-1].cost
		}
		if hasRaise {
			if !priced {
				return Cost{}, false
			}
			cost = cost.Plus(raise)
		}
		return pay.WithSpellAbilityExtras(f, cost), true
	case potentialModeBaseCostPlot:
		// The plot special action pays the K:Plot parameter.
		raw, ok := f.KeywordCostParam("Plot")
		if !ok {
			return Cost{}, false
		}
		return ParseCost(raw), true
	case potentialModeBaseCostPrototype:
		// CR 718.3a: the prototyped cast pays the K:Prototype inset cost,
		// with the spell's own additional costs folded exactly as beginCast
		// folds them.
		raw, _, _, ok := state.PrototypeParams(f)
		if !ok {
			return Cost{}, false
		}
		return pay.WithSpellAbilityExtras(f, ParseCost(raw)), true
	case potentialModeBaseCostFlashback:
		return pay.WithSpellAbilityExtras(f, e.flashbackCostFor(id, o)), true
	case potentialModeBaseCostKicked:
		kc, ok := kickerCost(f)
		return cost.Plus(kc), ok
	case potentialModeBaseCostMultikick:
		// CR 702.43: the offer prices the base plus ONE payment; the count
		// ask (multikickAsk) settles how many afterwards, exactly the kicked
		// offer's shape.
		mc, ok := multikickerCost(f)
		return cost.Plus(mc), ok
	case potentialModeBaseCostBuyback:
		bc, ok := buybackCost(f)
		return cost.Plus(bc), ok
	case potentialModeBaseCostOffspring:
		oc, ok := e.offspringCost(id)
		return cost.Plus(oc), ok
	case potentialModeBaseCostEntwined:
		ec, ok := entwineCost(f)
		return cost.Plus(ec), ok
	case potentialModeBaseCostSurged:
		return surgeCost(f)
	case potentialModeBaseCostMiracle:
		return e.miracleCost(id)
	case potentialModeBaseCostOmenAlt:
		// CR 720.3: the offer flips to the Omen face and prices that face's
		// printed cost (appendAlternateSpellFace's base), not the front's.
		af := omenSpellFace(e.G.Obj(id))
		if af == nil {
			return Cost{}, false
		}
		return pay.WithSpellAbilityExtras(af, ParseCost(af.ManaCost)), true
	case potentialModeBaseCostAdventureAlt:
		// CR 715.3a: the same flip for the Adventure spell face.
		af := adventureSpellFace(e.G.Obj(id))
		if af == nil {
			return Cost{}, false
		}
		return pay.WithSpellAbilityExtras(af, ParseCost(af.ManaCost)), true
	case potentialModeBaseCostModalSpell:
		// CR 712.8: the nonland back face's own printed cost.
		af := modalSpellBack(e.G.Obj(id))
		if af == nil {
			return Cost{}, false
		}
		return pay.WithSpellAbilityExtras(af, e.faceCost(af)), true
	case potentialModeBaseCostSplitAlt:
		// CR 709.4: the alternate half's own printed cost.
		af := splitAlternateCastFace(e.G.Obj(id))
		if af == nil {
			return Cost{}, false
		}
		return pay.WithSpellAbilityExtras(af, e.faceCost(af)), true
	case potentialModeBaseCostRoomAlt:
		// CR 309.4b: the selected door's own printed cost.
		af := roomAlternateCastFace(e.G.Obj(id))
		if af == nil {
			return Cost{}, false
		}
		return pay.WithSpellAbilityExtras(af, e.faceCost(af)), true
	case potentialModeBaseCostAftermath:
		// CR 702.85a: the Aftermath half's own printed cost.
		af := aftermathAlternateFace(e.G.Obj(id))
		if af == nil {
			return Cost{}, false
		}
		return pay.WithSpellAbilityExtras(af, ParseCost(af.ManaCost)), true
	case potentialModeBaseCostAdventureRecast:
		// CR 714.3a: the main face is already the object's face off the
		// stack, and the offer prices it without spell-ability extras.
		return cost, true
	case potentialModeBaseCostPreparedCopy:
		// CR 722.3c: the exile copy is cast as a copy of the prepare spell and
		// pays that spell's own printed cost (the offer's RawBaseCost).
		o := e.G.Obj(id)
		if o == nil || !o.IsCopy || o.PreparedSource == 0 {
			return Cost{}, false
		}
		return cost, true
	case potentialModeBaseCostSneak:
		// CR 702.190a: the sneak cost, including its mandatory Return part,
		// through the ONE reader the offer and beginCast use. The mode names
		// the exact instance beginCast charges (a granted one is
		// "sneaked_grant_N", never canonicalised in selectedMode).
		for _, sc := range e.sneakCosts(p, id) {
			if sc.mode == mode {
				return sc.cost, true
			}
		}
		return Cost{}, false
	}
	if m := altCastFor(string(mode)); m != nil && m.potential {
		// The alternative-cost keyword family (engine/rules/altcast_modes.go): the
		// same reader the offer and beginCast use.
		return altRowCost(e, m, id, f, o.AltCostIndex)
	}
	return Cost{}, false
}

// potentialModeCastFace is the face beginCast flips to for the modes that
// cast an alternate face, or nil for a mode that stays on the live face.
// potentialModeBaseCost already reads the same face's printed cost; this
// names it for the CR 601.2f composition, which must run with the object
// showing that face (faceprobe.go).
func potentialModeCastFace(o *state.Object, mode string) *cards.Face {
	switch potentialModeBaseCostCodes.Code(string(mode)) {
	case potentialModeBaseCostOmenAlt:
		return omenSpellFace(o)
	case potentialModeBaseCostAdventureAlt:
		return adventureSpellFace(o)
	case potentialModeBaseCostModalSpell:
		return modalSpellBack(o)
	case potentialModeBaseCostSplitAlt:
		return splitAlternateCastFace(o)
	case potentialModeBaseCostRoomAlt:
		return roomAlternateCastFace(o)
	case potentialModeBaseCostAftermath:
		return aftermathAlternateFace(o)
	}
	return nil
}

// potentialWitnessOffers reports whether the potential walk, priced with
// p's floating pool plus plan's production, offers the play o: the engine's
// own offer gate confirming a witness (non-mana parts are read for real).
func (e *Engine) potentialWitnessOffers(p state.PlayerID, o decision.Option, plan decision.PaymentPlan) bool {
	pool := e.G.Players[p].Pool
	for _, a := range plan.Activations {
		for c, n := range a.Produces {
			pool[c] += int32(n)
		}
	}
	// A cast play is looked up among the walk's cast options only, which
	// the casts-only walk lists exactly as the full walk does
	// (castsOnlyWalkVerify), without pricing every battlefield ability.
	e.potentialWitness = true
	opts := e.legalActionsWalkTemp(p, &pool, o.Kind == "cast")
	e.potentialWitness = false
	defer e.optRelease(opts)
	for _, opt := range opts {
		if opt.Kind == o.Kind && opt.Obj == o.Obj && opt.Ability == o.Ability && opt.Mode == o.Mode && opt.AltCostIndex == o.AltCostIndex {
			return true
		}
	}
	return false
}

// potentialCastVerdict is PlanCastPayment's verdict for the ordinary cast of
// id, whose origin is read from its zone.
func (e *Engine) potentialCastVerdict(p state.PlayerID, id state.ObjID, sh *castPlanShare) PaymentPlanOutcome {
	o := e.G.Obj(id)
	if o == nil {
		return PaymentPlanOutcome{Reason: "unsupported"}
	}
	origin := ""
	switch o.Zone {
	case state.ZHand:
		origin = "hand"
	case state.ZCommand:
		origin = "command_zone"
	default:
		return PaymentPlanOutcome{Reason: "unsupported", Detail: "origin"}
	}
	cast := decision.PlannedCast{Object: id, Face: plannedCastFace(o), Origin: origin}
	var got PaymentPlanOutcome
	if sh != nil {
		got = e.planCastPaymentMemo(p, cast, &sh.statics, &sh.candidates)
	} else {
		got = e.PlanCastPayment(p, cast)
	}
	if got.Plan != nil && got.Reason == "search_limit" {
		return got
	}
	if got.Plan != nil {
		got.Reason = ""
	}
	return got
}

// planAbilityPayment plans the mana part of printed activated ability
// `ability` (a flat pile index, as the "ability" option carries it) of id.
func (e *Engine) planAbilityPayment(p state.PlayerID, id state.ObjID, ability int) PaymentPlanOutcome {
	return e.planAbilityPaymentLifePips(p, id, ability, -1)
}

// planAbilityPaymentLifePips is planAbilityPayment with lifePips >= 0 pricing
// the cost's Phyrexian pips for a payer who pays that many with life (CR
// 107.4f): the remaining pips are planned as coloured pips of their colour.
// lifePips < 0 leaves the cost as compiled.
func (e *Engine) planAbilityPaymentLifePips(p state.PlayerID, id state.ObjID, ability, lifePips int) PaymentPlanOutcome {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return PaymentPlanOutcome{Reason: "unsupported"}
	}
	pa, ok := o.PileAbilityAt(ability)
	if !ok || pa.SA == nil || pa.SA.Kind != "AB" || cards.IsManaAbilitySA(pa.SA) {
		return PaymentPlanOutcome{Reason: "unsupported", Detail: "ability"}
	}
	ab := pa.SA
	cost, ok := pay.FixLifeXCost(asPayer(e), p, id, e.abilityCompiledCost(ab).Cost)
	if !ok {
		return PaymentPlanOutcome{Reason: "unsupported", Detail: "cost:life_x"}
	}
	if n := e.ownReduceCostOffer(p, id, ab, pa.Merged); n > 0 && cost.Generic >= n {
		cost.Generic -= n
	} else if n > 0 {
		cost.Generic = 0
	}
	// Heirloom Epic's `TapCreaturesForMana` ability: the offer gate prices the
	// activation with the same conservative creature credit (mana.go's
	// offerCastableUsing), so the witness must compose that identical cost, or
	// its Cost would over-state the activation the payment actually makes.
	if pay.TapCreaturesForMana(ab) {
		cost.Generic -= e.creatureManaOfferCredit(p, id, cost)
	}
	if lifePips >= 0 {
		for i, col := range cost.Phyrexian {
			if i >= lifePips {
				cost.Colored[state.ManaIndex(col)]++
			}
		}
		cost.Phyrexian = nil
	}
	scope := abilityScope(ab)
	statics := e.collectCostStatics()
	composed := e.offerCostForUsing(statics, p, id, cost, scope)
	out := e.planComposedCost(p, id, composed, true, true)
	if (out.Plan == nil || out.Reason != "") && statics.validTarget {
		// No plan at the untargeted price: a ValidTarget$ cost static
		// (Cloud, Planet's Champion's "Equip abilities that target Cloud cost
		// {2} less") is absent from the pre-target composition, but some legal
		// target's price may be payable (CR 601.2c, 601.2f), and the offer
		// gate's potential-target retry prices it (offerCastableUsing). The
		// target is not chosen at offer time, so each potential target's price
		// is tried and the first payable plan is the offer's witness. The
		// chosen target's cost is repriced before payment (rearmAutoPay).
		// Only a single-target announcement is priced this way: the retry
		// tests each candidate as the sole selection, which does not price a
		// multi-target ability.
		if _, max := targetBounds(ab); max == 1 {
			var potential PaymentPlanOutcome
			if _, ok := e.potentialCostModsUsing(statics, p, id, scope, e.costPotentialTargets(p, id, scope), 0, func(m costMods) bool {
				potential = e.planComposedCost(p, id, e.composedOfferCost(p, id, cost, m, scope), true, true)
				return potential.Plan != nil && potential.Reason == ""
			}); ok {
				return potential
			}
		}
	}
	if out.Reason != "insufficient" || cost.Waterbend <= 0 {
		return out
	}
	// A Waterbend<N> part's taps pay its generic (CR 701.67a): what the sources
	// cannot fund, the offer's credit may, and the ask requires those taps.
	for _, red := range e.withWaterbendOfferCredit(p, id, cost.Tap, cost.XMin, costMods{Waterbend: cost.Waterbend}).Reduces {
		composed.Generic -= red.Generic
	}
	return e.planComposedCost(p, id, composed, true, true)
}

// planComposedCost plans the mana part of composed, the offer-time cost
// (offerCostFor's composition) of a play of id: an activated ability of the
// permanent id (ability true, selfSource true) or a spell id (ability
// false). Only the mana part is planned; the non-mana parts are the play's
// own asks, and the plan must leave them payable:
//
//   - a fixed-count tapXType part (Heap Gate's "tap an untapped Gate you
//     control") needs untapped candidates the plan did not tap for mana. The
//     planner tries every assignment of candidates to the part (bounded by
//     planReservationLimit) with those candidates withheld from the mana
//     census, so a play is priced whichever candidates it must keep;
//   - a fixed-count sacrifice part needs candidates the plan did not
//     sacrifice or return: a source TAPPED for mana is still a legal
//     sacrifice (CR 602.2b pays the costs in any order), so Makeshift
//     Munitions' "{1}, Sacrifice an artifact" may tap an artifact land for
//     the {1} and then sacrifice it;
//   - a cost that taps or untaps the source itself withholds it.
//
// The verdict is "insufficient" (a proof, modulo the census) only when no
// mana plan exists at all, or when every candidate assignment was tried and
// each had no mana plan; any other failure is "unsupported" with Detail
// "cost:reserved_sources".
func (e *Engine) planComposedCost(p state.PlayerID, id state.ObjID, composed Cost, ability, selfSource bool) PaymentPlanOutcome {
	return e.planComposedCostFor(p, id, decision.PlannedCast{}, composed, ability, selfSource)
}

// planComposedCostFor is planComposedCost with the cast the plan is for: its
// Object is the card pay.PaymentPlanHandDemand leaves out of key 6 (an
// either-or additional cost's witness prices the cast while the card is still
// in hand, so it must name it exactly as the plain cast planner does).
func (e *Engine) planComposedCostFor(p state.PlayerID, id state.ObjID, cast decision.PlannedCast, composed Cost, ability, selfSource bool) PaymentPlanOutcome {
	if len(composed.Hybrid) != 0 {
		return e.planHybridCost(p, id, composed, ability, selfSource)
	}
	mana := Cost{Colored: composed.Colored, Generic: composed.Generic, X: composed.X, XMin: composed.XMin,
		Hybrid: composed.Hybrid, Phyrexian: composed.Phyrexian, Twobrid: composed.Twobrid,
		HybridPhyrexian: composed.HybridPhyrexian, Snow: composed.Snow}
	if detail := pay.PlanCostDetail(mana); detail != "" {
		return PaymentPlanOutcome{Reason: "unsupported", Detail: detail}
	}
	if global, detail := e.paymentPlanGlobalManaEffect(p, id); global {
		return PaymentPlanOutcome{Reason: "unsupported", Detail: detail}
	}
	// A consumption-only query (the shape-cast planner arm) reads the plan
	// only for whether it pays and what it consumes, so a cost the option
	// set provably cannot fund is answered without the search
	// (planCoverInsufficient).
	if e.planConsumptionOnly && e.planCoverInsufficient(p, mana) {
		return PaymentPlanOutcome{Reason: "insufficient"}
	}
	// tapped: sources the cost taps (their tapping abilities cannot pay);
	// kept: sources the cost sacrifices (they may tap for mana first, but
	// no alternative of theirs may consume them).
	var tapped, kept []state.ObjID
	if selfSource && (composed.Tap || composed.Untap) {
		tapped = append(tapped, id)
	}
	parts, selfKept := planCostParts(composed, id, selfSource)
	kept = append(kept, selfKept...)
	manaOnly := Cost{Colored: mana.Colored, Generic: mana.Generic}
	desc := pay.DescriptorFor(id, ability, composed)
	desc.FaceIdx = e.planFaceIdx(id) // an alternate-face route's restriction reads the cast face
	return e.planReserving(p, id, composed, ability, selfSource, parts, func(tapR, keptR []state.ObjID) PaymentPlanOutcome {
		out := pay.PlanPaymentCostWithout(asPayer(e), p, cast, manaOnly, &desc, append(slices.Clone(tapped), tapR...), nil, append(slices.Clone(kept), keptR...))
		if out.Plan != nil && out.Reason == "search_limit" {
			// The node budget only stops the hunt for the rank-best plan; the
			// plan it holds pays the cost, so it is a witness like any other
			// (a cost with hybrid pips and a dozen sources used to lose it).
			out.Reason = ""
		}
		return out
	})
}

// planCostParts is the fixed-count tap and sacrifice parts of cost a mana plan
// may compete for, and the source itself when the cost sacrifices it.
func planCostParts(cost Cost, id state.ObjID, selfSource bool) (parts []planCostPart, kept []state.ObjID) {
	for _, part := range cost.TapPermanent {
		if part.Dyn == "" && part.N > 0 {
			parts = append(parts, planCostPart{part: part, tap: true})
		}
	}
	for _, part := range cost.Sac {
		switch {
		case part.Announced || part.N <= 0:
		case selfSource && pay.PlanSelfCost(part, id):
			kept = append(kept, id)
		default:
			parts = append(parts, planCostPart{part: part})
		}
	}
	return parts, kept
}

// planReserving asks plan for a mana plan with nothing reserved, and when that
// plan spends a permanent the cost itself must tap or sacrifice (Heap Gate's
// "{1}, {T}, tap an untapped Gate": the other Gate tapped for the {1}; Makeshift
// Munitions' "{1}, Sacrifice an artifact": the only other artifact a Treasure
// sacrificed for the {1}), tries each assignment of candidates to those parts:
// a tap candidate withheld from tapping for mana, a sacrifice candidate from
// being consumed.
func (e *Engine) planReserving(p state.PlayerID, id state.ObjID, cost Cost, ability, selfSource bool, parts []planCostPart, plan func(tapR, keptR []state.ObjID) PaymentPlanOutcome) PaymentPlanOutcome {
	got := plan(nil, nil)
	if got.Plan == nil {
		// No plan even with every other source is a proof.
		return got
	}
	if e.planLeavesCostPayable(p, id, cost, ability, selfSource, *got.Plan, nil) {
		return got
	}
	exhaustive, allInsufficient := true, true
	var found *PaymentPlanOutcome
	e.planEachReservation(p, id, cost, ability, parts, func(tapR, keptR []state.ObjID) bool {
		retry := plan(tapR, keptR)
		if retry.Plan == nil {
			if retry.Reason != "insufficient" {
				allInsufficient = false
			}
			return true
		}
		if e.planLeavesCostPayable(p, id, cost, ability, selfSource, *retry.Plan, tapR) {
			found = &retry
			return false
		}
		allInsufficient = false
		return true
	}, &exhaustive)
	hasTap, hasSac := false, false
	for _, pt := range parts {
		hasTap, hasSac = hasTap || pt.tap, hasSac || !pt.tap
	}
	switch {
	case found != nil:
		return *found
	case exhaustive && allInsufficient && !(hasTap && hasSac):
		// Every assignment was tried. (With both a tap and a sacrifice
		// part the assignments keep them distinct, which a payment need
		// not: no proof then.)
		return PaymentPlanOutcome{Reason: "insufficient", Detail: "cost:reserved_sources"}
	}
	return PaymentPlanOutcome{Reason: "unsupported", Detail: "cost:reserved_sources"}
}

// planCostPart is one fixed-count tap (tap true) or sacrifice part of a
// play's cost that a mana plan may compete for.
type planCostPart struct {
	part CostPart
	tap  bool
}

// planHybridCost plans a cost with hybrid pips ({R/G}: one mana of either
// colour, CR 107.4e) by resolving every pip to each of its colours in turn
// and planning the resolved cost: the first resolution with a witness
// wins, and the play is unpayable when every resolution is. More than
// planHybridLimit pips is unsupported.
func (e *Engine) planHybridCost(p state.PlayerID, id state.ObjID, composed Cost, ability, selfSource bool) PaymentPlanOutcome {
	pips := composed.Hybrid
	if len(pips) > planHybridLimit {
		return PaymentPlanOutcome{Reason: "unsupported", Detail: "cost:hybrid"}
	}
	allInsufficient := true
	for mask := 0; mask < 1<<len(pips); mask++ {
		c := composed
		c.Hybrid = nil
		for i, h := range pips {
			sym := h.A
			if mask>>i&1 == 1 {
				sym = h.B
			}
			idx := state.ManaIndex(sym)
			if idx < 0 || idx >= len(c.Colored) {
				return PaymentPlanOutcome{Reason: "unsupported", Detail: "cost:hybrid"}
			}
			c.Colored[idx]++
		}
		got := e.planComposedCost(p, id, c, ability, selfSource)
		if e.planKeepSources && got.Plan != nil && paymentPlanConsumes(*got.Plan) {
			// A witness that must keep every source is asked of each
			// colouring: the first one that pays may need a source the
			// cast cannot spare (a Treasure) while another pays from
			// the lands alone.
			got = PaymentPlanOutcome{Reason: "insufficient"}
		}
		if got.Plan != nil && got.Reason == "" {
			return got
		}
		if got.Reason != "insufficient" {
			allInsufficient = false
		}
	}
	if allInsufficient {
		return PaymentPlanOutcome{Reason: "insufficient", Detail: "cost:hybrid"}
	}
	return PaymentPlanOutcome{Reason: "unsupported", Detail: "cost:hybrid"}
}

// planHybridLimit bounds planHybridCost's 2^n resolutions.
const planHybridLimit = 4

// planReservationLimit bounds the candidate assignments planComposedCost
// tries for a cost's tap parts (and its sacrifice-conflict retries). A
// larger space is not searched and its verdict is never a proof.
const planReservationLimit = 64

// planEachReservation calls try with each assignment of distinct
// candidates to parts -- untapped candidates for a tap part (never id itself
// when the cost taps it), sacrifice candidates for a sacrifice part -- in
// zone order, as combinations within a part, split into the tap and the
// sacrifice assignments. It stops when try returns false; *exhaustive is
// cleared when planReservationLimit cut the enumeration short.
func (e *Engine) planEachReservation(p state.PlayerID, id state.ObjID, cost Cost, ability bool, parts []planCostPart, try func(tapR, sacR []state.ObjID) bool, exhaustive *bool) {
	cands := make([][]state.ObjID, len(parts))
	for i, pt := range parts {
		if !pt.tap {
			cands[i] = pay.SacrificeCostCandidates(asPayer(e), p, id, pt.part, ability)
			continue
		}
		for _, oid := range pay.TapCostCandidates(asPayer(e), p, id, pt.part) {
			if cost.Tap && oid == id {
				continue
			}
			cands[i] = append(cands[i], oid)
		}
	}
	calls := 0
	chosen := make([][]state.ObjID, len(parts))
	taken := func(oid state.ObjID) bool {
		for _, c := range chosen {
			if slices.Contains(c, oid) {
				return true
			}
		}
		return false
	}
	var rec func(part, from int) bool
	rec = func(part, from int) bool {
		if part == len(parts) {
			if calls >= planReservationLimit {
				*exhaustive = false
				return false
			}
			calls++
			var tapR, sacR []state.ObjID
			for i, pt := range parts {
				if pt.tap {
					tapR = append(tapR, chosen[i]...)
				} else {
					sacR = append(sacR, chosen[i]...)
				}
			}
			return try(tapR, sacR)
		}
		if int32(len(chosen[part])) == parts[part].part.N {
			return rec(part+1, 0)
		}
		for i := from; i < len(cands[part]); i++ {
			oid := cands[part][i]
			if taken(oid) {
				continue
			}
			chosen[part] = append(chosen[part], oid)
			ok := rec(part, i+1)
			chosen[part] = chosen[part][:len(chosen[part])-1]
			if !ok {
				return false
			}
		}
		return true
	}
	rec(0, 0)
}

// planConsumed is the set of plan sources the plan uses up: every source
// (tapped) when tapped is true, else only the last-resort steps that
// sacrifice or return their source.
func planConsumed(plan decision.PaymentPlan, tapped bool) map[state.ObjID]bool {
	out := make(map[state.ObjID]bool, len(plan.Activations)) // lookup only
	for _, a := range plan.Activations {
		if tapped || (a.Consequence != nil && (a.Consequence.Sacrifice || a.Consequence.ReturnToHand)) {
			out[a.Source] = true
		}
	}
	return out
}

// planLeavesCostPayable reports whether cost's fixed-count tap and
// sacrifice parts still have enough candidates once plan's sources are
// spent. A tap part needs untapped candidates the plan did not tap; reserved
// (when non-nil) is the exact assignment the tap parts take, withheld from
// the plan. A sacrifice part needs candidates the plan did not sacrifice or
// return (a tapped source can still be sacrificed) and that no tap part
// claimed; a self-sacrifice needs the plan to leave id in play. The cost's
// own {T} claims id only for the tap: a sacrifice part may still take id --
// CR 602.2b pays the costs in any order, so the source is tapped and then
// sacrificed (Bloodflow Connoisseur's "{T}, Sacrifice a creature" beside no
// other creature).
func (e *Engine) planLeavesCostPayable(p state.PlayerID, id state.ObjID, cost Cost, ability, selfSource bool, plan decision.PaymentPlan, reserved []state.ObjID) bool {
	tapped := planConsumed(plan, true)
	gone := planConsumed(plan, false)
	claimed := map[state.ObjID]bool{} // lookup only
	enough := func(cands []state.ObjID, n int32, spent map[state.ObjID]bool, skipSelf bool) bool {
		for _, oid := range cands {
			if n <= 0 {
				break
			}
			if spent[oid] || claimed[oid] || (skipSelf && oid == id) {
				continue
			}
			claimed[oid] = true
			n--
		}
		return n <= 0
	}
	if reserved != nil {
		for _, oid := range reserved {
			if tapped[oid] {
				return false
			}
			claimed[oid] = true
		}
	} else {
		for _, part := range cost.TapPermanent {
			if part.Dyn == "" && part.N > 0 && !enough(pay.TapCostCandidates(asPayer(e), p, id, part), part.N, tapped, cost.Tap) {
				return false
			}
		}
	}
	for _, part := range cost.Sac {
		if part.Announced || part.N <= 0 {
			continue
		}
		if selfSource && pay.PlanSelfCost(part, id) {
			if gone[id] {
				return false
			}
			continue
		}
		if !enough(pay.SacrificeCostCandidates(asPayer(e), p, id, part, ability), part.N, gone, false) {
			return false
		}
	}
	return true
}

// paymentPlanCensus is the planner census's coverage of p's mana
// abilities (paymentPlanCensusOf).
type paymentPlanCensus struct {
	// complete: the census prices every mana ability p could activate, so
	// the planner's "insufficient" is a proof.
	complete bool
	// relaxable: every ability the census misses has a relaxed
	// alternative set (paymentPlanRelaxedAlternatives), held in relaxed.
	relaxable bool
	relaxed   [][]pay.Alt
	// paid holds the relaxed alternatives of the missed abilities whose
	// cost includes mana (a filter: Heap Gate's "{1}, {T}: Add one mana of
	// any color"), each with its fee; relaxed holds the free ones.
	paid []relaxedPaid
	// fund is the prefix search's funding for a gap ability with no sound
	// relaxation but a mana fee (a repeatable fee-only ability such as
	// Orochi Leafcaller's "{G}: Add one mana of any color"): every
	// activation spends its fee from other sources, bounded only by the
	// search's node budget.
	fund int
	// bound and boundPaid hold the same relaxed units as relaxed and paid
	// would, plus the extra copies a repeatable tap source can make in one
	// payment (paymentPlanRelaxedAlternatives' repeats). They are set only
	// when the census is incomplete and NOT relaxable, and they feed one
	// necessary-condition proof that may skip a prefix search that could
	// not succeed. The proof is never reported (never "insufficient") and
	// never substituted for the relaxed proof above: dirac mirrors the
	// search and its slot accounting node for node, so only an outcome the
	// search itself could not have changed may be skipped.
	bound     [][]pay.Alt
	boundPaid []relaxedPaid
	bounded   bool
	// gaps lists, in zone order, the sources with an ability the census
	// misses (potentialPrefixScript activates them).
	gaps []state.ObjID
}

// boundedProof returns the census a bounded necessary-condition proof runs
// with: the bounded relaxation units in the relaxed and paid slots
// paymentPlanRelaxProof reads. The rest of the census is carried unchanged,
// and a run's outcome is read only as "could a plan exist", never reported.
func (c paymentPlanCensus) boundedProof() paymentPlanCensus {
	c.relaxed, c.paid = c.bound, c.boundPaid
	return c
}

// fees is the mana the missed paid abilities' fees total (how many other
// sources a prefix may activate to fund them).
func (c paymentPlanCensus) fees() int {
	n := c.fund
	for _, pd := range c.paid {
		n += int(pd.fee)
	}
	return n
}

// paymentPlanCensusOf reports whether the planner's source census prices
// every mana ability p could activate: each battlefield permanent with an
// activatable mana ability is a census unit with an alternative for every
// one of those abilities. Only then is the planner's "insufficient" a
// proof.
//
// "Activatable" is PotentialMana's own membership: the eligibility walk
// with its mana part priced against hyp, the seat's potential-mana bound
// (so a paid source such as Heap Gate's "{1}, {T}: Add one mana of any
// color" counts, since other sources could fund its fee), while every
// non-mana part of the cost is read for real. A tapped land's {T} ability
// can contribute nothing to this payment (CR 602.2b: its cost cannot be
// paid), so it never makes the census incomplete; nor does an ability that
// makes nothing (manaAbilityYieldsNothing: an empty commander colour
// identity, CR 903.4f, or an Amount$ that resolves to zero).
//
// For each ability the census misses it also builds the relaxed
// alternatives (paymentPlanRelaxedAlternatives); relaxable is false when
// one of them has none. It builds the bounded relaxation in the same walk
// (bound/boundPaid), which is sound even once relaxable is false: the once
// path stops there, the bounded one continues, and both are dropped when the
// census is complete or still relaxable.
func (e *Engine) paymentPlanCensusOf(p state.PlayerID, hyp *state.Mana) paymentPlanCensus {
	out := paymentPlanCensus{complete: true, relaxable: true}
	units := pay.PaymentPlanQueryUnits(asPayer(e), p)
	at := make(map[state.ObjID]int, len(units)) // lookup only
	for i, u := range units {
		at[u.ID] = i
	}
	var probe *Engine
	bounded := true // every gap ability so far has a once-only or bounded relaxation
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		var abs []*cards.SA
		var face *cards.Face
		if o := e.G.Obj(id); o != nil {
			face = o.Face()
		}
		for _, ma := range e.appendAvailableManaAbilitiesGate(nil, nil, p, id, true) {
			// An ability that makes nothing -- "any color in your commander's
			// color identity" with no such colour (CR 903.4f), an Amount$ that
			// resolves to zero -- can, like a tapped land's, contribute
			// nothing to the payment, so the planner missing it leaves the
			// census complete and no prefix activates it.
			if !(face != nil && e.manaAbilityYieldsNothing(p, id, face, ma)) && e.manaAbilityPayablePool(p, id, ma, hyp) {
				abs = append(abs, ma)
			}
		}
		if len(abs) == 0 {
			continue
		}
		var alts []pay.Alt
		if i, ok := at[id]; ok {
			alts = pay.PaymentPlanQueryAlternatives(asPayer(e), units[i])
		}
		for _, ma := range abs {
			covered := false
			for _, a := range alts {
				if pay.SameManaAbility(a.Ma, ma) {
					covered = true
					break
				}
			}
			if covered {
				continue
			}
			out.complete = false
			if !slices.Contains(out.gaps, id) {
				out.gaps = append(out.gaps, id)
			}
			if !out.relaxable && !bounded {
				continue
			}
			relaxed, fee, repeats, once, ok := e.paymentPlanRelaxedAlternatives(p, id, ma, &probe)
			if !ok && fee > 0 {
				// a repeatable fee-only ability converts colour, not mana:
				// each activation spends its fee from other sources, so the
				// prefix search must be allowed to tap one source per fee per
				// activation, up to its own node budget
				out.fund += int(fee) * potentialScriptNodes
			}
			if out.relaxable {
				if !ok || !once || (fee > 0 && len(out.paid) >= relaxedPaidLimit) {
					// A repeatable tap source is deliberately not relaxable:
					// the relaxed proof below trusts only a once-only unit,
					// and promoting one would move relaxed_census verdicts
					// and dirac's mirrored slot accounting.
					out.relaxable, out.relaxed, out.paid = false, nil, nil
				} else if fee > 0 {
					out.paid = append(out.paid, relaxedPaid{alts: relaxed, fee: fee})
				} else {
					out.relaxed = append(out.relaxed, relaxed)
				}
			}
			if bounded {
				if !ok || (fee > 0 && len(out.boundPaid)+int(repeats) > relaxedPaidLimit) {
					bounded = false
					continue
				}
				for i := int32(0); i < repeats; i++ {
					if fee > 0 {
						out.boundPaid = append(out.boundPaid, relaxedPaid{alts: relaxed, fee: fee})
					} else {
						out.bound = append(out.bound, relaxed)
					}
				}
			}
		}
	}
	if out.complete {
		out.relaxable, out.relaxed, out.paid, out.gaps = false, nil, nil, nil
	}
	out.bounded = bounded && !out.complete && !out.relaxable
	if !out.bounded {
		out.bound, out.boundPaid = nil, nil
	}
	return out
}

// tapOnlyManaCost reports whether a mana ability's cost is exactly literal
// tapXType parts: no {T}/{Q}, no mana fee, and no other payment at all. That
// is the repeatable shape paymentPlanRelaxedAlternatives bounds by its
// untapped candidates (Supportive Parents' "Tap two untapped creatures you
// control: Add one mana of any color"); a dynamic tapXType<X/...> part is
// excluded, as its count is announced rather than bounded by the board.
func tapOnlyManaCost(c costvocab.Cost) bool {
	if len(c.TapPermanent) == 0 {
		return false
	}
	want := costvocab.Cost{TapPermanent: c.TapPermanent}
	if !c.Equal(&want) {
		return false
	}
	for _, part := range c.TapPermanent {
		if part.Dyn != "" || part.N <= 0 {
			return false
		}
	}
	return true
}

// paymentPlanRelaxedAlternatives is the RELAXATION of mana ability ma of
// source id for an unpayability proof: one free, normal-tier alternative per
// production the ability can make (a fixed production; one per colour of an
// Any/Combo/Chosen choice; one per colour multiset for an amount above one),
// with its cost ignored -- no fee, no tapped companion, no counter. Every
// real activation sequence maps onto a relaxed plan that produces at least
// as much, so a play no relaxed plan pays is unpayable.
//
// That holds only while the relaxed ability is usable at most once in the
// payment, which is why once is false unless its cost taps, untaps,
// sacrifices or returns the source or it carries a once-only activation
// limit. ok is false when neither shape applies, and false too when the
// amount cannot be bounded (the engine's own evaluator, taken at its larger
// value on the live board and on the all-tapped probe, since a payment only
// taps) or the production names no colour. A relaxed alternative is never a
// witness: its Source is carried for ranking only and the caller discards
// every relaxed Plan.
//
// repeats is how many times one relaxed unit may be used in a single
// payment: 1 for the once-only shapes, and, for the repeatable tap-another
// cost it refuses today (Supportive Parents' "Tap two untapped creatures you
// control: Add one mana of any color"), K = min over its literal tap parts
// of floor(candidates/N): each activation taps N distinct untapped
// permanents, and nothing in a mana-payment prefix untaps them, so K bounds
// the activations a real sequence can make. Only the bounded
// necessary-condition proof (paymentPlanCensus.boundedProof) uses K copies;
// the relaxed proof above never runs on such a source (once is false), so
// its verdicts and dirac's mirrored slot accounting do not move. K == 0
// contributes no unit, but is still a sound bound: that cost cannot be paid
// even once.
//
// fee is the mana the ability's own cost spends (Heap Gate's {1}); the
// proof charges it as generic on top of the play's cost whenever the
// ability may be used (paymentPlanRelaxProof), so a filter is never counted
// as fresh mana.
func (e *Engine) paymentPlanRelaxedAlternatives(p state.PlayerID, id state.ObjID, ma *cards.SA, probe **Engine) (alts []pay.Alt, fee, repeats int32, once, ok bool) {
	o := e.G.Obj(id)
	if o == nil || ma == nil || ma.API != "Mana" {
		return nil, 0, 0, false, false
	}
	cost := e.manaAbilityCC(p, id, ma).Cost
	if cost.X != 0 || cost.XMin != 0 || len(cost.Hybrid)+len(cost.Phyrexian)+len(cost.Twobrid)+len(cost.HybridPhyrexian) != 0 || cost.Snow != 0 {
		return nil, 0, 0, false, false
	}
	// The fee is the mana the activation spends whatever else follows: a
	// repeatable fee-only ability (Orochi Leafcaller's "{G}: Add one mana of
	// any color", Energy Refractor's "{2}: ...") has no sound once-only
	// relaxation below, but the prefix search still has to fund each
	// activation, so the census reads its fee even when ok is false.
	fee = cost.Generic + cost.Colored.Total()
	once = cost.Tap || cost.Untap || strings.TrimSpace(ma.ParamStr(cards.PKActivationLimit)) == "1" ||
		strings.TrimSpace(ma.ParamStr(cards.PKGameActivationLimit)) == "1"
	for _, part := range cost.Sac {
		once = once || pay.PlanSelfCost(part, id)
	}
	for _, part := range cost.Return {
		once = once || pay.PlanSelfCost(part, id)
	}
	if once {
		repeats = 1
	} else if tapOnlyManaCost(cost) {
		// A repeatable tap-another cost: every activation taps N distinct
		// untapped permanents of each part, and a mana-payment prefix never
		// untaps them, so the untapped candidates bound the activations.
		repeats = -1
		for _, part := range cost.TapPermanent {
			n := int32(len(pay.ManaTapCandidates(asPayer(e), p, id, part.Spec, nil))) / part.N
			if repeats < 0 || n < repeats {
				repeats = n
			}
		}
		if repeats == 0 {
			// The cost cannot be paid even once: no unit, still bounded.
			return nil, 0, 0, false, true
		}
	} else {
		// repeatable and not a bounded tap part: no sound relaxation, but
		// the fee is still the prefix search's funding (fees below)
		return nil, fee, 0, false, false
	}
	amt := availableAmount(ma)
	if amt <= 0 {
		live, ok := e.castWindowAmount(p, id, o, ma)
		if !ok {
			return nil, 0, 0, false, false
		}
		if *probe == nil {
			*probe = e.paymentPlanTappedProbe(p)
		}
		tapped, ok := (*probe).castWindowAmount(p, id, o, ma)
		if !ok {
			return nil, 0, 0, false, false
		}
		amt = max(live, tapped)
	}
	creature := e.IsCreature(id)
	// The tap fires the static TapsForMana triggers whichever colour it takes,
	// so each relaxed production carries the largest extra they add (CR 605.4a):
	// a relaxation may only over-produce.
	extra := e.potentialTapExtra(id, ma)
	var out []pay.Alt
	add := func(m state.Mana) {
		for i := range m {
			m[i] += extra[i]
		}
		// The relaxed alternative's production rides on the plan's activation
		// exactly as a census unit's does (pay/planalts.go): the witness gate
		// (potentialWitnessOffers) prices the play from the pool plus every
		// activation's Produces, so an empty field would hide a fee-only
		// source's mana and reject the very plan the relaxed proof is asking
		// about. The alternative is never executed -- paymentPlanRelaxProof
		// discards every Plan -- so this only feeds that gate.
		out = append(out, pay.Alt{Activation: decision.PaymentActivation{Source: id, Produces: pay.ManaAmount(m)},
			Mana: m, Creature: creature, Ma: ma, Tier: pay.TierNormal, Flex: 1, FlexAll: 1})
	}
	if !relaxedAddProduction(effects.ManaOf(ma), amt, add) {
		return nil, 0, 0, false, false
	}
	// A Condition-split ability (Gemstone Caverns' luck counter, River of
	// Tears' land drop) adds its sub's production INSTEAD when the gate holds
	// (CR 605.1a): the sub's productions are further alternatives, so the
	// relaxation still only over-produces (mana_condsplit.go).
	if sub := condSplitLiveSub(e, p, id, ma); sub != nil {
		if subAmt := availableAmount(sub); subAmt <= 0 || !relaxedAddProduction(effects.ManaOf(sub), subAmt, add) {
			return nil, 0, 0, false, false
		}
	}
	return out, fee, repeats, once, true
}

// relaxedAddProduction hands add one relaxed production per way mp can make
// amt units (a fixed production; one per colour multiset of an
// Any/Combo/Chosen choice), false when it names no colour or amt is out of
// the enumerated range.
func relaxedAddProduction(mp *effects.ManaParams, amt int32, add func(state.Mana)) bool {
	counts, any := mp.Counts, mp.CountsAny
	if !any {
		var m state.Mana
		for i, n := range counts {
			m[state.ManaIndex(cards.ManaSymbol(i))] += n * amt
		}
		if m.Total() <= 0 {
			return false
		}
		add(m)
		return true
	}
	var cols []int
	for i, n := range counts {
		if n > 0 {
			cols = append(cols, int(state.ManaIndex(cards.ManaSymbol(i))))
		}
	}
	if len(cols) == 0 || amt > 6 {
		return false
	}
	// Every multiset of amt units over cols (non-decreasing index runs).
	var m state.Mana
	var rec func(from int, left int32)
	rec = func(from int, left int32) {
		if left == 0 {
			add(m)
			return
		}
		for i := from; i < len(cols); i++ {
			m[cols[i]]++
			rec(i, left-1)
			m[cols[i]]--
		}
	}
	rec(0, amt)
	return true
}

type potentialModeBaseCostCode uint16

const (
	potentialModeBaseCostEmpty potentialModeBaseCostCode = iota + 1
	potentialModeBaseCostMayplay
	potentialModeBaseCostPlot
	potentialModeBaseCostFlashback
	potentialModeBaseCostKicked
	potentialModeBaseCostBuyback
	potentialModeBaseCostOffspring
	potentialModeBaseCostEntwined
	potentialModeBaseCostSurged
	potentialModeBaseCostMiracle
	potentialModeBaseCostOmenAlt
	potentialModeBaseCostAdventureAlt
	potentialModeBaseCostModalSpell
	potentialModeBaseCostSplitAlt
	potentialModeBaseCostRoomAlt
	potentialModeBaseCostAftermath
	potentialModeBaseCostAdventureRecast
	potentialModeBaseCostPreparedCopy
	potentialModeBaseCostSneak
	potentialModeBaseCostPrototype
	potentialModeBaseCostMultikick
)

var potentialModeBaseCostCodes = state.NewStrCodes(
	state.StrEntry[potentialModeBaseCostCode]{Key: "", Val: potentialModeBaseCostEmpty},
	state.StrEntry[potentialModeBaseCostCode]{Key: "mayplay", Val: potentialModeBaseCostMayplay},
	state.StrEntry[potentialModeBaseCostCode]{Key: "plot", Val: potentialModeBaseCostPlot},
	state.StrEntry[potentialModeBaseCostCode]{Key: "flashback", Val: potentialModeBaseCostFlashback},
	state.StrEntry[potentialModeBaseCostCode]{Key: "kicked", Val: potentialModeBaseCostKicked},
	state.StrEntry[potentialModeBaseCostCode]{Key: "multikicked", Val: potentialModeBaseCostMultikick},
	state.StrEntry[potentialModeBaseCostCode]{Key: "buyback", Val: potentialModeBaseCostBuyback},
	state.StrEntry[potentialModeBaseCostCode]{Key: "offspring", Val: potentialModeBaseCostOffspring},
	state.StrEntry[potentialModeBaseCostCode]{Key: "entwined", Val: potentialModeBaseCostEntwined},
	state.StrEntry[potentialModeBaseCostCode]{Key: "surged", Val: potentialModeBaseCostSurged},
	state.StrEntry[potentialModeBaseCostCode]{Key: "miracle", Val: potentialModeBaseCostMiracle},
	state.StrEntry[potentialModeBaseCostCode]{Key: "omen_alt", Val: potentialModeBaseCostOmenAlt},
	state.StrEntry[potentialModeBaseCostCode]{Key: "adventure_alt", Val: potentialModeBaseCostAdventureAlt},
	state.StrEntry[potentialModeBaseCostCode]{Key: "modal_spell", Val: potentialModeBaseCostModalSpell},
	state.StrEntry[potentialModeBaseCostCode]{Key: "split_alt", Val: potentialModeBaseCostSplitAlt},
	state.StrEntry[potentialModeBaseCostCode]{Key: "room_alt", Val: potentialModeBaseCostRoomAlt},
	state.StrEntry[potentialModeBaseCostCode]{Key: "aftermath", Val: potentialModeBaseCostAftermath},
	state.StrEntry[potentialModeBaseCostCode]{Key: "adventure_recast", Val: potentialModeBaseCostAdventureRecast},
	state.StrEntry[potentialModeBaseCostCode]{Key: "prepared_copy", Val: potentialModeBaseCostPreparedCopy},
	state.StrEntry[potentialModeBaseCostCode]{Key: "sneak", Val: potentialModeBaseCostSneak},
	state.StrEntry[potentialModeBaseCostCode]{Key: "prototype", Val: potentialModeBaseCostPrototype},
)

// planFaceIdx is the Descriptor.FaceIdx of e.planFace on object id: 1 + its
// index in the object's Card.Faces, or 0 when no alternate face is being
// planned or it is not one of the object's own faces (offerAsFace prices
// such a face against the live face, exactly as no face does).
func (e *Engine) planFaceIdx(id state.ObjID) uint8 {
	if e.planFace == nil {
		return 0
	}
	if o := e.G.Obj(id); o != nil && o.Card != nil {
		for i, f := range o.Card.Faces {
			if f == e.planFace {
				return uint8(i + 1)
			}
		}
	}
	return 0
}
