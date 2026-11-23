package rules

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// targetBoundReadsPromisedGift limits the alternate-branch offer check to
// target bounds whose SVar table can read Count$PromisedGift. It is a
// conservative over-approximation: a SVar that reads PromisedGift for a
// non-bound purpose also trips it. That is the safe direction for an
// OFFER-feasibility union (it can only widen the offered set, never narrow
// it, and the post-election ask still reads the elected branch), so the
// false-positive cost is an offered cast whose only feasible branch the
// player then does not elect -- the ordinary CR 601.2c reversal.
func targetBoundReadsPromisedGift(o *state.Object, sa *cards.SA) bool {
	if o == nil || o.Face() == nil || sa == nil {
		return false
	}
	if effects.TargetsOf(sa).Has(effects.TgtBoundPromisedGift) {
		return true
	}
	for _, body := range o.Face().SVars {
		if strings.Contains(body, "Count$PromisedGift") {
			return true
		}
	}
	return false
}

// targetSAAvailable reports whether a target declaration admits at least one
// legal announcement at offer time: enough legal candidates for its resolved
// mandatory minimum AND the cross-target set constraints the post-push ask
// enforces (per-controller exclusivity/OneEach, same-controller group capacity
// and set-property capacity). It is still not a full cast/payment check:
// target-dependent cost modifiers and the final target announcement belong to
// the post-push ask. CR 601.2c is the reason the offer must not admit a
// declaration the ask will reverse (CR 733.1) the instant it is submitted.
func (e *Engine) targetSAAvailable(p state.PlayerID, id, excludeSelf state.ObjID, sa *cards.SA, x int32, xPending bool) bool {
	return targetSAAvailableGift(e, p, id, excludeSelf, sa, x, xPending, nil)
}

// targetSAAvailableGift is targetSAAvailable judging a Count$PromisedGift
// bound under one Gift election (gift non-nil) instead of the union of both
// (nil): targetsAvailable asks each election in turn, so a root and a chain
// link that are each feasible only under DIFFERENT elections (Long River's
// Pull: "counter target creature spell" unpromised, "instead counter target
// spell" promised) are not offered on a board where neither election works.
func targetSAAvailableGift(e *Engine, p state.PlayerID, id, excludeSelf state.ObjID, sa *cards.SA, x int32, xPending bool, gift *bool) bool {
	if sa == nil || !effects.TargetsOf(sa).Targeted() {
		return true
	}
	// An unannounced X bound cannot be read yet, but no X makes a target of an
	// object the rest of the description rules out (CR 601.2c, 602.2b).
	if anyX := effects.TargetsOf(sa).AnyX; xPending && anyX != nil {
		sa = anyX
	}
	min, dynamic := targetOfferMin(e, p, id, sa, x, xPending, gift)
	// The census is a pure read; the two answers that never look at the
	// population return before walking it.
	if dynamic || min <= 0 {
		return true
	}
	if capCMC, capped := e.maxTotalTargetCMC(p, id, sa, x); capped {
		candidates := e.legalTargetCandidates(p, id, excludeSelf, sa)
		candidates, _, _ = e.totalCMCCappedCandidates(candidates, p, id, sa, x)
		if !e.targetChoiceFeasible(sa, candidates, min) {
			return false
		}
		// A subset-sum cap binds even when enough individually legal
		// candidates exist: the cheapest `min` of them is the minimal-sum
		// subset of that size (mana values are nonnegative), so if it busts
		// the cap no legal subset does. Every current corpus carrier has
		// TargetMin$ 0, so this arm is prophylactic. Derive the effective
		// minimum the same way targetChoiceFeasible does (the OneEach/
		// same-controller/set-property bounds can lower min) so both readers
		// reason about the same count.
		eff := min
		eff, _, _, _ = e.oneEachTargetBounds(sa, candidates, eff, 0)
		eff, _, _, _ = e.sameControllerTargetBounds(sa, candidates, eff, 0)
		eff, _, _, _, _ = e.setPropTargetBounds(sa, candidates, eff, 0)
		if eff > 0 {
			vms := make([]int, 0, len(candidates))
			for _, c := range candidates {
				if c.kind == "player" {
					// Players are valid members of a target set and contribute
					// zero to its total mana value.
					vms = append(vms, 0)
					continue
				}
				if o := e.G.Obj(c.obj); o != nil && o.Face() != nil {
					vms = append(vms, int(o.Face().ManaValue()))
				}
			}
			if len(vms) < eff {
				return false
			}
			slices.Sort(vms)
			total := 0
			for _, mv := range vms[:eff] {
				total += mv
			}
			return total <= capCMC
		}
		return true
	}
	if !targetCrossConstrained(sa) {
		// No cross-target constraint: targetChoiceFeasible reduces to
		// len(candidates) >= min, which the limited census answers exactly
		// (candidatesForLimit stops at min only when no post-filter can drop
		// a candidate) without matching the rest of the population -- an
		// any-target spell stops at the player seats.
		ok := e.candidatesCountForLimit(p, id, excludeSelf, sa, true, min) >= min
		if walkCacheVerify && ok != e.targetChoiceFeasible(sa, e.legalTargetCandidates(p, id, excludeSelf, sa), min) {
			panic("rules: limited target census disagrees with the full census")
		}
		return ok
	}
	candidates := e.legalTargetCandidates(p, id, excludeSelf, sa)
	return e.targetChoiceFeasible(sa, candidates, min)
}

// targetOfferMin is the mandatory minimum the offer census judges a target
// declaration against, or dynamic=true when the declaration cannot be judged
// before its announcement exists. targetSAAvailable and the joint
// TargetUnique$ chain census (uniqueChainTargetsFeasible) both read it, so a
// slot's demand means the same thing to both.
func targetOfferMin(e *Engine, p state.PlayerID, id state.ObjID, sa *cards.SA, x int32, xPending bool, gift *bool) (min int, dynamic bool) {
	// A pending {X} is announced before targets, so a bare X bound cannot be
	// judged at offer time. Keep that shape offerable and let targetAsk use the
	// settled value. SVar-backed bounds remain readable now and are checked.
	// Read by literal key: the param census's rot guard rejects a dynamic
	// Params key that is not a function parameter.
	if xPending && effects.TargetsOf(sa).Has(effects.TgtBoundX) {
		return 0, true
	}
	if gift != nil {
		// One election, judged as made (targetSAAvailableGift).
		min, _ = e.resolvedTargetBoundsWithGift(p, id, sa, x, gift)
		if min > 0 && xPending && effects.TargetsOf(sa).Has(effects.TgtValidXBound) {
			return 0, true
		}
		return min, false
	}
	min, _ = e.resolvedTargetBounds(p, id, sa, x)
	// CR 702.168a: the Gift election has not been made yet when this census
	// runs, so a bound that reads Count$PromisedGift is feasible if EITHER
	// branch of the promise is satisfiable. The union is a min of the two
	// branches' minimums: the census compares candidates against min, and a
	// smaller min is satisfiable exactly when at least one branch is. The
	// post-election ask (and every other reader) still resolves the ELECTED
	// branch through the nil-override resolvedTargetBounds, so promising
	// remains what switches the actual bound.
	if targetBoundReadsPromisedGift(e.G.Obj(id), sa) {
		promised := true
		if pmin, _ := e.resolvedTargetBoundsWithGift(p, id, sa, x, &promised); pmin < min {
			min = pmin
		}
	}
	if min > 0 && xPending && effects.TargetsOf(sa).Has(effects.TgtValidXBound) {
		return 0, true
	}
	return min, false
}

// targetCrossConstrained reports whether sa carries a cross-target
// constraint targetChoiceFeasible reads beyond the candidate count: per-
// controller exclusivity (oneEachTargetBounds), same-controller grouping
// (sameControllerTargetBounds) or a set-property constraint
// (setPropTargetBounds). Each of those helpers is the identity on min when
// its predicate here is false.
func targetCrossConstrained(sa *cards.SA) bool {
	if targetControllerExclusive(sa) || effects.TargetsOf(sa).Has(effects.TgtSameController) {
		return true
	}
	mode, _ := targetSetPropMode(sa)
	return mode != decision.SetPropNone
}

// targetChoiceFeasible is the ONE home for whether a target declaration with
// the given candidate population admits at least one legal selection under
// every cross-target constraint the ask enforces: the resolved mandatory
// minimum (with TargetMin$/TargetMax$ OneEach respelled to the
// distinct-controller count), TargetsForEachPlayer$/TargetsWithDifferentControllers$
// exclusivity, TargetsWithSameController$ group capacity, and the
// set-property capacity. Both the offer census (targetSAAvailable) and the
// post-push cast ask (cast.go's targetAsk) call it against their OWN candidate
// list, so an offered cast can always announce a legal target and the two sites
// cannot drift. Cost-driven candidate pruning stays the ask's alone: only it
// can re-price a target-dependent cost modifier.
func (e *Engine) targetChoiceFeasible(sa *cards.SA, candidates []targetCandidate, min int) bool {
	if min <= 0 {
		return true
	}
	min, _, exclusive, distinct := e.oneEachTargetBounds(sa, candidates, min, 0)
	min, _, sameCapacity, sameController := e.sameControllerTargetBounds(sa, candidates, min, 0)
	min, _, setCapacity, setMode, _ := e.setPropTargetBounds(sa, candidates, min, 0)
	if len(candidates) < min {
		return false
	}
	if exclusive && min > distinct {
		return false
	}
	if sameController && min > sameCapacity {
		return false
	}
	if setMode != decision.SetPropNone && min > setCapacity {
		return false
	}
	return true
}

// charmTargetsAvailable evaluates the possible CR 601.2b mode announcement
// before a cast is offered. Target-bearing modes with an unsatisfiable
// mandatory minimum are removed from the possible announcement set, using
// the same target census castModeAsk uses after the spell reaches the stack.
// A mode whose target count depends on an announcement remains offerable.
// xPending leaves the announcement pending; x is the announced X the census
// judges the modes at (xPending false): the census xBoundTargetPayable walks
// per candidate X.
func (e *Engine) charmTargetsAvailable(p state.PlayerID, id, excludeSelf state.ObjID, sa *cards.SA, x int32, xPending bool) bool {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return true
	}
	choices := effects.CharmOf(sa).Modes
	ctx := effects.NewCtxPtr(id, p, effects.CtxInit{})
	if pc := e.cast; pc != nil && pc.card == id {
		ctx.Kicker.PendingTeamwork = castAnswerCodes.Code(pc.mode) == castAnswerTeamworkMode
	}
	effects.SetSVars(ctx, o.Face().SVars)
	legal := make([]string, 0, len(choices))
	for _, name := range choices {
		name = strings.TrimSpace(name)
		sub := cards.ResolveSVar(o.Face().SVars, name)
		if sub != nil && !modeTargetsAvailable(e, p, id, excludeSelf, sub, x, xPending) {
			continue
		}
		legal = append(legal, name)
	}
	legal = effects.CharmEligibleModes(e, id, sa, legal)
	min, _, repeat := effects.CharmModeBounds(e, ctx, sa, len(legal))
	return repeat || min <= len(legal)
}

// targetsAvailable reports whether the target requirement that can be proved
// before offering a spell or ability is satisfiable. Dynamic announcements
// remain offerable until the post-announcement targetAsk backstop evaluates
// them. excludeSelf is the CR 115.5 object for a spell, or zero for an
// activated ability whose Source permanent may target itself.
// targetsReadXPending reports whether targetsAvailable(sa) reads its
// xPending argument at all: only the Choices$ charm census and a
// ValidTgts$ census behind no Announce$ do. Every other shape answers from
// sa alone, so an offer gate may skip pricing the X announcement (two parsed
// costs per card) for it.
func targetsReadXPending(sa *cards.SA) bool {
	if sa == nil {
		return false
	}
	if sa.API == "Charm" && effects.CharmOf(sa).HasChoices {
		return true
	}
	if effects.TargetsOf(sa).Targeted() {
		return true
	}
	// A targeting SubAbility$ link is censused too (chainTargetsAvailable),
	// and its bounds or spec may read the pending X exactly as a root's do.
	for sub := sa.Sub; sub != nil; sub = sub.Sub {
		if effects.TargetsOf(sub).Targeted() {
			return true
		}
	}
	return false
}

func (e *Engine) targetsAvailable(p state.PlayerID, id, excludeSelf state.ObjID, sa *cards.SA, xPending bool) bool {
	if sa == nil {
		return true
	}
	if sa.Sub != nil && targetBoundReadsPromisedGift(e.G.Obj(id), sa) {
		// CR 702.168a: the Gift election is ONE choice for the whole spell,
		// made before targets. The root and its chain must be announceable
		// under the same election, so each election is judged whole.
		for _, promised := range [2]bool{false, true} {
			if e.rootTargetsAvailable(p, id, excludeSelf, sa, xPending, &promised) &&
				e.chainTargetsAvailable(p, id, excludeSelf, sa, 0, xPending, &promised) {
				return true
			}
		}
		return false
	}
	return e.rootTargetsAvailable(p, id, excludeSelf, sa, xPending, nil) &&
		e.chainTargetsAvailable(p, id, excludeSelf, sa, 0, xPending, nil)
}

// chainTargetsAvailable is the SubAbility$ half of the offer census (CR
// 601.2c): every link of the chain announces its targets on cast, so a
// mandatory link with no legal target makes the whole spell or ability
// unannounceable -- Bite Down with no creature or planeswalker the caster
// doesn't control cannot be cast. It walks the same links the cast flow's
// subTargetAsk announces (collectSubTargetPreAsks) through the same
// feasibility predicate the root uses, and stays OFFERABLE for every link it
// cannot judge before the announcement exists:
//   - a link behind an optional additional cost (Condition$ Kicked /
//     OptionalCost): whether it owes a target depends on the cast option;
//   - a link whose declaration reads an earlier target of the same spell
//     (subTargetingReadsRootTarget): not announced by the cast flow at all
//     (castSubPreAskable).
//
// A TargetUnique$ link ("another target creature") is judged twice: alone,
// like any link, and jointly with the root and the chain's other unique
// links (uniqueChainTargetsFeasible) -- its pool excludes every earlier
// declaration's answer, so three unique "target creature" declarations need
// three distinct creatures. Skipping the link outright (the old census)
// offered Stand Together with one creature on the table and Swift Kick with
// no creature its unique link could name (cardfuzz fuzz-1003 planrev).
//
// The post-push ask (subTargetAsk) is the backstop for the unjudged links,
// exactly as targetAsk is for a dynamic root.
func (e *Engine) chainTargetsAvailable(p state.PlayerID, id, excludeSelf state.ObjID, sa *cards.SA, x int32, xPending bool, gift *bool) bool {
	if sa.Sub == nil {
		return true
	}
	var svars map[string]string
	svarsRead := false
	var uniq []*cards.SA
	for _, sub := range e.collectSubTargetPreAsks(sa) {
		if castSubCondition(sub) != castCondOther {
			continue
		}
		if !svarsRead {
			svarsRead = true
			if o := e.G.Obj(id); o != nil && o.Face() != nil {
				svars = o.Face().SVars
			}
		}
		if subTargetingReadsRootTarget(sub, svars) {
			continue
		}
		subExclude := excludeSelf
		if attachSelfDestination(sub) {
			subExclude = id
		}
		if !targetSAAvailableGift(e, p, id, subExclude, sub, x, xPending, gift) {
			return false
		}
		if effects.TargetUniqueRequested(sub) {
			uniq = append(uniq, sub)
		}
	}
	return len(uniq) == 0 || uniqueChainTargetsFeasible(e, p, id, excludeSelf, sa, uniq, x, xPending, gift)
}

func (e *Engine) rootTargetsAvailable(p state.PlayerID, id, excludeSelf state.ObjID, sa *cards.SA, xPending bool, gift *bool) bool {
	if sa.API == "Charm" && effects.CharmOf(sa).HasChoices {
		return e.charmTargetsAvailable(p, id, excludeSelf, sa, 0, xPending)
	}
	// An Announce$ value can change the target restriction itself; its value
	// is not available until the cast transaction reaches the announcement
	// stage, so retain the post-announcement backstop for that shape. A
	// declaration that reads no announced name (Primitive Justice's "destroy
	// target artifact", whose announced counts size only its chain) is judged
	// now like any other.
	if announceShapesTargets(e, id, sa) {
		return true
	}
	if !effects.TargetsOf(sa).Targeted() {
		return true
	}
	return targetSAAvailableGift(e, p, id, excludeSelf, sa, 0, xPending, gift)
}

// costAnnouncesX reports whether paying this cost announces a value for {X}
// before targets are chosen: a printed {X} mana symbol or a PayEnergy<X>
// energy part (Forge announces both through the same ability X). A cost that
// announces X makes every ValidTgts$ bound that reads that X (cmcEQX and its
// siblings) a DYNAMIC bound -- the same carve-out TargetMin$/TargetMax$/
// Announce$/Choices$ already have -- so the offer gate does not withhold the
// action on a bound whose value does not exist yet; the post-announcement
// askTarget backstop evaluates it once the X is fixed (CR 601.2b before
// 601.2c).
// costAnnouncesX reports whether the cost announces an X the cast or
// activation chooses (CR 601.2b/107.3i): a printed {X} mana symbol, a
// PayEnergy<X> part, an announced PayLife<X> payment, an announced
// SubCounter<X/Kind> removal, or a tapXType<X/Spec> part whose tap election
// announces it (the dynTapCost head). The tap-election clause does not add
// the announced-Sac clause: this gate's callers treat a true answer as "the
// X-bound targets are dynamic -- offer and evaluate at the ask", and the
// tap election is announced BEFORE the target ask (the tap stage precedes
// targetAsk in continueCast), so the bound is already fixed when targets
// are chosen either way; the clause only stops the offer gate from
// withholding the action on a bound whose value the tap election will
// supply (Aryel's powerLEX).
func costAnnouncesX(c Cost) bool { return costAnnouncesXRef(&c) }

// costAnnouncesXRef is costAnnouncesX reading c in place.
func costAnnouncesXRef(c *Cost) bool {
	if c.X > 0 {
		return true
	}
	for _, part := range c.Energy {
		if part.Spec == "X" {
			return true
		}
	}
	if len(c.LifeX) > 0 {
		return true
	}
	for _, part := range c.SubCounter {
		if part.Announced {
			return true
		}
	}
	for _, part := range c.TapPermanent {
		if part.Dyn == "X" {
			return true
		}
	}
	// An X-bound group exile part (Fabrication Foundry's withTotalCMCEQX)
	// announces X from the totals its election can reach (groupAnnounceXAsk).
	for _, part := range c.Exile {
		if part.AtLeast && part.Group.Var != "" {
			return true
		}
	}
	// An announced Blight<X> part binds the cast's X exactly like Sac<X/Spec>
	// (Blighted Nightmare's `Blight<X> Return<1/CARDNAME>` ability): the
	// payment settles the announced count, so the X was announced even when
	// its value is 0.
	for _, part := range c.Blight {
		if part.Announced {
			return true
		}
	}
	return false
}

// castTargetsAvailable is the cast-offer guard: the spell card may not target
// itself (CR 115.5), so excludeSelf is the card id. A cost that announces an
// X (printed {X} or a SpellAbility Cost$ PayEnergy<X>) relaxes an X-bound
// spec to the post-announcement backstop (costAnnouncesX above).
//
// A split card in a zone that combines its halves (CR 709.4: hand, graveyard,
// exile) is judged as the one-half spell it becomes (CR 709.3b), so protection
// from a colour only the other half carries (Blightbeetle's protection from
// green against Armed, red, of the green Armed // Dangerous) withholds nothing.
// A fused cast keeps both halves' colours and asks splitCastTargetsAvailable.
func (e *Engine) castTargetsAvailable(p state.PlayerID, id state.ObjID, sa *cards.SA) bool {
	run := func() bool {
		xPending := false
		if !targetsReadXPending(sa) {
			// xPending is never read for this shape (targetsReadXPending).
			return e.targetsAvailable(p, id, id, sa, false)
		}
		if o := e.G.Obj(id); o != nil && o.Face() != nil {
			xPending = costAnnouncesXRef(&e.faceCompiledCost(o.Face()).Cost)
			if ab := o.Face().SpellAbility(); ab != nil {
				xPending = xPending || costAnnouncesXRef((&e.abilityCompiledCost(ab).Cost))
			}
		}
		return e.targetsAvailable(p, id, id, sa, xPending)
	}
	if o := e.G.Obj(id); o != nil && o.Zone != state.ZStack {
		if _, split := o.SplitHalves(); split {
			return e.offerAsSpellOnStack(id, run)
		}
	}
	return run()
}

// abilityTargetsAvailable is the activated-ability offer guard. It is what
// stops an ability with no legal target from being re-offered in a loop after
// the transaction aborts it (CR 602.2b / 601.2c: such an ability cannot be
// activated at all). Unlike a cast, an activated ability CAN target its own
// Source permanent (Mother of Runes targeting itself), so no self-exclusion
// applies -- EXCEPT for an attach ability (API$ Attach, the Equip/Reconfigure
// expansion): CR 701.3a attaches to ANOTHER permanent, and targetAsk
// excludes the source from an attach SA's pool, so the offer must exclude it
// too. Without that, a creature that GAINED an Equip ability (Trazyn the
// Infinite with an Equipment in its owner's graveyard) as the only creature
// its controller had was offered an Equip whose ask then found no legal
// target and aborted, forever. An ability cost that
// announces an X (a {X} mana symbol or PayEnergy<X>) relaxes an X-bound spec
// to the post-announcement backstop.
func (e *Engine) abilityTargetsAvailable(p state.PlayerID, id state.ObjID, ab *cards.SA) bool {
	var excludeSelf state.ObjID
	if attachSelfDestination(ab) {
		excludeSelf = id
	}
	if !e.abilityTargetBoundAvailable(p, id, excludeSelf, ab) {
		return false
	}
	if !targetsReadXPending(ab) {
		// xPending is never read for this shape (targetsReadXPending).
		return e.targetsAvailable(p, id, excludeSelf, ab, false)
	}
	cost := e.abilityCompiledCost(ab).Cost
	return e.targetsAvailable(p, id, excludeSelf, ab, costAnnouncesX(cost)) &&
		e.xBoundTargetPayable(p, id, excludeSelf, ab, cost, abilityScope(ab), true)
}

// charmModeXBound reports whether some mode of the Choices$ charm sa declares a
// target bounded by the announced {X}. It reads only the spell's face.
func charmModeXBound(o *state.Object, sa *cards.SA) bool {
	if o == nil || o.Face() == nil {
		return false
	}
	for _, name := range effects.CharmOf(sa).Modes {
		// CharmParams.Modes are cards.SplitModeNames's output: already trimmed.
		sub := cards.ResolveSVar(o.Face().SVars, name)
		if sub != nil && effects.TargetsOf(sub).Has(effects.TgtValidXBound) {
			return true
		}
	}
	return false
}

// xBoundTargetPayable reports whether some announcement of base's {X} both pays,
// from the pool plus every untapped source, and leaves sa a legal target (CR
// 601.2b, 601.2c, 601.2h). A target bounded by an {X} cost is otherwise judged
// without a value for X, so Rydia, Summoner of Mist would be offered with one
// mana available and only a mana value 4 Saga in the graveyard. A reduction that
// reads the paid X breaks the price's monotony and is not judged here.
func (e *Engine) xBoundTargetPayable(p state.PlayerID, id, excludeSelf state.ObjID, sa *cards.SA, base Cost, scope costScope, ability bool) bool {
	charm := isCharmWithChoices(sa)
	if base.X <= 0 || !(charm && charmModeXBound(e.G.Obj(id), sa) || !charm && effects.TargetsOf(sa).Has(effects.TgtValidXBound)) || costAnnouncesPaidX(base) {
		return true
	}
	bound := e.PotentialMana(p)
	statics := e.collectCostStatics()
	prev := e.cast
	defer func() { e.cast = prev }()
	// The potential bound prices an open "any colour" production as unbounded
	// in every slot (potential.go), so one Treasure Token can "pay" an X the
	// seat cannot: one mana of any colour is one unit, not one per colour.
	// priceExactly confirms an announcement with the payment planner, whose
	// insufficient verdict is a proof only over a complete source census
	// (insufficient below: the planner prices only the mana abilities its
	// source census covers, so over an incomplete census the verdict proves
	// nothing unless a relaxation of the abilities it misses still cannot pay,
	// paymentPlanRelaxProof: a {T}, remove-a-counter source (Gemstone Mine's
	// "one mana of any color") the planner cannot execute can still make the
	// owed colour, CR 601.2g). A ValidTarget$ cost static can reduce
	// the price for one legal target choice (offerCastableUsing's
	// potential-target retry), so an unpayable untargeted composition retries
	// each potential target's composition; an unsupported shape is not a proof
	// either way and keeps the bound's answer.
	onceGap := false
	haveCensus := false
	priceExactly := func(cost Cost) bool {
		// "insufficient" is a proof only while the planner's source census
		// covers every mana ability the seat could activate, or a relaxation
		// of the ones it misses still cannot pay: a filter's "{1}, {T}: Add one
		// mana of any color" (Unknown Shores) is a gap, so Killing Glare's X=1
		// ({1}{B} from Plains, Radiant Grove and Shores) stays offered (CR
		// 605.3b, 601.2g).
		insufficient := func(composed Cost) bool {
			// bound is the potential pool the census reads.
			verdict := func() PaymentPlanOutcome { return e.planComposedCost(p, id, composed, ability, ability) }
			if verdict().Reason != pay.ReasonInsufficient {
				return false
			}
			e.beginDerivedMemo()
			defer e.endDerivedMemo()
			defer pay.PaymentPlanQueryEnd(asPayer(e), pay.PaymentPlanQueryBegin(asPayer(e)))
			census := e.paymentPlanCensusOf(p, &bound)
			onceGap, haveCensus = census.onceGap, true
			if census.complete {
				return true
			}
			// A lone paid gap's output cannot pay its own fee: price it as a cut
			// of the announced cost (paymentPlanRelaxProof's cutSelfFee).
			census.cutSelfFee = true
			if census.relaxable && e.paymentPlanRelaxProof(census, verdict).Reason == pay.ReasonInsufficient {
				return true
			}
			// A repeatable fee-only gap (Crossroads Candleguide's "{2}: Add one
			// mana of any color") has no relaxation and makes the census
			// non-relaxable, yet it never raises the mana total it converts
			// (CR 106.1, 605.3b): the count proof altcast.go's provenUnpayable
			// runs. An announcement the pool plus every census unit's largest
			// production cannot fund by count has no real payment, so the X
			// must not be offered (CR 601.2b, 601.2h). An ability payment keeps
			// the over-bound answer.
			if ability {
				return false
			}
			credit := e.hasCastConvoke(id) || e.hasCastImprovise(id) || e.hasCastDelve(id, false)
			return !census.gapGains && costOutrunsSupply(asPayer(e), p, int32(e.G.Players[p].Pool.Total()), census.gapSupply, composed, credit)
		}
		if !insufficient(e.offerCostForUsing(statics, p, id, cost, scope)) {
			return true
		}
		if !statics.validTarget {
			return false
		}
		ok := false
		e.potentialCostModsUsing(statics, p, id, scope, e.costPotentialTargets(p, id, scope), 0, func(m costMods) bool {
			if insufficient(e.composedOfferCost(p, id, cost, m, scope)) {
				return false
			}
			ok = true
			return true
		})
		return ok
	}
	for x := base.XMin; ; x++ {
		announced := base.WithX(x)
		if !e.offerCastableUsing(statics, p, id, &announced, scope, ability, &bound) {
			return false
		}
		e.cast = &pendingCast{player: p, card: id, ability: -1, x: x}
		var hasTarget bool
		if charm {
			// Confront the Past: the X bound sits on a mode's target, not on the Charm.
			hasTarget = e.charmTargetsAvailable(p, id, excludeSelf, sa, x, false)
		} else {
			hasTarget = e.targetSAAvailable(p, id, excludeSelf, sa, x, false)
		}
		// Price the announcement exactly before offering it (or walking past
		// it to a pricier X).
		if !priceExactly(announced) {
			return false
		}
		if hasTarget {
			// Confirm the announcement against a bounded potential mana.
			// PotentialMana prices a storage land's "remove any number of
			// storage counters: add that much {W}" (Cost$ T
			// SubCounter<X/KIND>, Amount$ X) as unbounded, and the payment
			// planner's census cannot execute that ability, so priceExactly
			// cannot prove such an announcement unpayable (an unsupported
			// shape "keeps the bound's answer"). The same bound with
			// SubCounterBound reads each such ability at the counters it can
			// remove (potential_subcounter.go), which is still an
			// over-approximation of every real payment (the other sources
			// keep their potential). A cost the bounded bound cannot pay has
			// no real payment, so the X must not be offered (CR 107.3i
			// announces X before CR 601.2b's targets, and CR 601.2h's payment
			// must reach it). The cost rises with X, so a pricier
			// announcement (and every later one) is unpayable too.
			desc := offerPayment(id, ability, scope)
			composed := e.offerCostForUsing(statics, p, id, announced, scope)
			desc.Cost, desc.SubCounterBound = &composed, true
			bounded := e.potentialManaFor(p, &desc)
			cost := announced
			if !e.offerCastableUsing(statics, p, id, &cost, scope, ability, &bounded) {
				return false
			}
			// A one-shot gap the census cannot relax is the shape the cast
			// window can price source-exclusively: Wooded Bastion's
			// "{G/W}, {T}: Add {G}{G}" spends the Plains's {W} for its fee,
			// while the potential bound adds that production beside the same
			// land's "{T}: Add {C}" and the Plains's own {W}. A repeatable
			// fee-only gap keeps the historical potential answer (the spell
			// burst recording pins it), so only a onceGap reaches this check.
			// Refuse only when the window's census priced every ability the
			// seat could activate (CR 601.2h), the same soundness gate the
			// target menu's probe uses; an incomplete census keeps the
			// potential bound rather than hiding a play it cannot price. A
			// mode's own payment descriptor is not built here, so the check
			// stays on the plain spell and the plain activation (Sequence
			// Engine's "{X}, {T}: Exile target creature card with mana value X"
			// beside Wooded Bastion's gap, CR 106.6, 601.2h, 602.2b).
			if haveCensus && onceGap && scope.Mode == "" && (ability || scope.Kind == "Spell") {
				from := state.ZHand
				if o := e.G.Obj(id); o != nil {
					from = o.Zone
				}
				pcCost := composed
				pcCost.X = base.X // XAnnounced on the window's descriptor; the cost is already folded
				pc := &pendingCast{player: p, card: id, from: from, ability: -1, cost: pcCost}
				if ability {
					pc.ability = 0 // isAbility: the window prices an activation's payment class
				}
				units := e.castWindowUnits(pc)
				if e.castWindowCensusComplete(pc, units) && !e.castWindowPays(pc, composed, units) {
					return false
				}
			}
			return true
		}
	}
}
