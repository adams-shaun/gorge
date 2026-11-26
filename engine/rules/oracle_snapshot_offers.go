package rules

import (
	"strconv"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// oracleSnapshotOffers reads the already-filtered priority actions PLUS the
// potential-action walk's plays, deduped by (source, kind, label). The
// priority offer's gate (castable, and the mana-ability payability read)
// prices against the seat's floating pool alone -- the tap-first decision
// model the bots, the wire and the dirac lockstep replay on, which must not
// change -- while CR 601.2g-h and 602.2b make a play legal when its cost can
// be paid by activating mana abilities during the announcement. The
// snapshot's `offered` is that CR-legal set (the side Forge and XMage report;
// xproof compares [offered]), so the potential-action walk's plays -- the
// engine's own legal-offer walk priced against the untapped-source bound
// (PotentialMana): same timing, restriction, target and non-mana gates,
// every non-mana part against the real state -- fill in what the
// floating-pool gate withheld, each one first CONFIRMED by the engine's own
// source-exclusive payment planner (potentialPlans / planComposedCost: the
// witness a real payment can execute, or the census proof that none exists),
// and each mana activation priced the same way (the live-pool gate beside a
// per-source-exclusive read of the CR 601.2g payment window's own ordered
// search). The bound itself is never the answer: it is an upper bound that
// counts both of a dual's faces and cannot see that a cost's own tap spends
// its source's last mana. Generic mana actions expand through the same named
// legal-ability reader the oracle assertion uses; the labels keep that
// reader's spelling. This must not implement another legality walk.
func oracleSnapshotOffers(r *oracleRun, d *decision.Decision) []OracleSnapOffered {
	var out []OracleSnapOffered
	seen := make(map[OracleSnapOffered]bool, len(d.Options))
	add := func(row OracleSnapOffered) {
		if seen[row] {
			return
		}
		seen[row] = true
		out = append(out, row)
	}
	for _, o := range d.Options {
		if o.Obj == 0 {
			continue
		}
		source := r.objRef(r.e.G.Obj(o.Obj))
		if oracleOpCodes.Code(o.Kind) == oracleOpActivate {
			for _, label := range r.manaAbilityLabels(d.Player, o.Obj) {
				add(OracleSnapOffered{Source: source, Kind: "activate", Label: label})
			}
			continue
		}
		kind := o.Kind
		switch oracleOpCodes.Code(kind) {
		case oracleOpCast, oracleOpPlay:
		default:
			// The priority walk's existing real-play classification includes
			// printed, granted and keyword/special actions. Don't grow a
			// parallel list here when another activation kind is added.
			if !potentialPlayKind(kind) {
				continue
			}
			kind = "activate"
			if o.Kind == optPlayLand {
				kind = "play"
			}
		}
		add(OracleSnapOffered{Source: source, Kind: kind, Label: o.Label})
	}
	r.addPotentialOffers(d, add)
	return out
}

// addPotentialOffers appends the potential-action walk's plays and the mana
// activations the untapped sources really pay for, into add (already
// deduped). The walk is the engine's own (potentialWalkOf); its rows are
// priced against an OVER-bound (PotentialMana), the direction the projection
// must never lose a play in, so each row that the floating-pool gate
// withheld is confirmed here against a source-exclusive read before it is
// reported:
//
//   - a cast, a printed or keyword-granted ability and a turn-face-up action
//     take the payment planner's own verdict (potentialPlans, the machinery
//     EnsurePotentialProofs projects): a witness the real payment can
//     execute confirms the play, the census proof ("insufficient") withholds
//     it, a shape the planner does not price keeps the walk's own admission;
//   - a play the planner's shape switch leaves unpriced but whose composed
//     cost the same planner prices (an SVar-anchored granted activation such
//     as a Station 12+ grant, an unlock, a specialize) takes
//     planComposedCost's verdict through the same recipe planKeywordAbility
//     runs, guards included;
//   - a play_land or a station is never mana-costed and confirms itself;
//   - a mana activation prices its fee against the live pool beside the CR
//     601.2g payment window's own ordered search over the seat's untapped
//     sources, the source the fee's own tap spends excluded (CR 605.3b).
//
// Both halves are pure reads; the map in the caller is only indexed, never
// ranged.
func (r *oracleRun) addPotentialOffers(d *decision.Decision, add func(OracleSnapOffered)) {
	if r.e.G.Over {
		return
	}
	p := d.Player
	_, opts := r.e.potentialWalkOf(p, true)
	// The planner's verdicts, matched positionally to the play-kind rows of
	// opts: potentialPlans walks this same slice with this same filter. Read
	// lazily, so a snapshot whose every new row is a mana activation never
	// plans.
	var plans []PotentialPlan
	plansFor := func() []PotentialPlan {
		if plans == nil {
			plans = r.e.potentialPlans(p, 0)
		}
		return plans
	}
	pi := 0
	for _, o := range opts {
		if !potentialPlayKind(o.Kind) {
			continue
		}
		confirmed := true
		switch {
		case o.Kind == optPlayLand || o.Kind == optStation:
			// Never mana-costed (potentialPlayKind's own contract): nothing
			// to confirm.
		case o.Kind == optCast || o.Kind == optTurnFaceUp ||
			(o.Kind == optAbility && o.SVar == "" && o.GainedSource == 0 && o.AltCostIndex == 0):
			// A shape the planner prices. A witness confirms the play, the
			// census proof withholds it, a shape the planner declines keeps
			// the walk's own admission -- the planner is the engine's own
			// "proven unpayable" authority (pay.ReasonInsufficient), the one
			// EnsurePotentialProofs already projects.
			if pls := plansFor(); pi < len(pls) && pls[pi].Reason == pay.ReasonInsufficient {
				confirmed = false
			}
		default:
			confirmed = r.composedPlayConfirmed(p, o)
		}
		pi++
		if !confirmed {
			continue
		}
		kind := o.Kind
		switch oracleOpCodes.Code(kind) {
		case oracleOpCast, oracleOpPlay:
		default:
			// Printed, granted and keyword/special plays activate, the same
			// classification the priority options get.
			kind = "activate"
			if o.Kind == optPlayLand {
				kind = "play"
			}
		}
		add(OracleSnapOffered{Source: r.objRef(r.e.G.Obj(o.Obj)), Kind: kind, Label: o.Label})
	}
	units := r.e.windowManaUnits(p)
	for _, id := range r.potentialManaSources(p) {
		for _, label := range r.potentialManaLabels(p, id, units) {
			add(OracleSnapOffered{Source: r.objRef(r.e.G.Obj(id)), Kind: "activate", Label: label})
		}
	}
}

// potentialManaSources walks the zones the legal-offer walk's mana section
// walks, in zone order: the battlefield for every seat (an ability another
// player's Activator$ permit reaches p's offer) plus p's hand and graveyard
// (Spirit Guides and Jack-o'-Lantern, CR 605.2a). The choke point behind the
// labels re-checks zone, controller and every other non-payability gate, so
// a generous zone list only costs reads.
func (r *oracleRun) potentialManaSources(p state.PlayerID) []state.ObjID {
	var ids []state.ObjID
	for seat := range r.e.G.Players {
		ids = append(ids, r.e.G.Zone(state.ZBattlefield, state.PlayerID(seat))...)
	}
	ids = append(ids, r.e.G.Zone(state.ZHand, p)...)
	ids = append(ids, r.e.G.Zone(state.ZGraveyard, p)...)
	return ids
}

// composedPlayConfirmed confirms a potential play the planner's shape switch
// leaves unpriced (potentialPlayVerdictShared's final "unsupported, shape")
// whose composed cost the same planner does price. The recipes are the
// planner's own for the sibling shapes, not a second legality walk: the
// cost's derivation is the walk's (planKeywordAbility for an SVar-anchored
// granted activation, the unlock and specialize offer loops' own cost
// readers), and the pricing is planComposedCost, the source-exclusive
// composed-cost planner whose witnesses the real payment can execute.
//
// Every failure mode answers TRUE -- keep the walk's own admission -- because
// this confirm's contract is one-directional: it may only WITHHOLD a play
// whose cost is proven unpayable (a false drop loses one oracle row, a false
// keep is the pooled-bound defect this confirm exists to close). An
// unresolvable option, an unpriced shape (PlanCostDetail), a verdict the
// composition cannot see through (a target-priced reduction static, a
// witness that consumed a candidate of an unreserved part) all keep the
// walk's admission, exactly as planKeywordAbility's guards leave them
// unsupported.
func (r *oracleRun) composedPlayConfirmed(p state.PlayerID, o decision.Option) bool {
	e := r.e
	switch o.Kind {
	case optAbility, optGranted:
		ab := r.grantedAbilitySA(o)
		if ab == nil || ab.Kind != "AB" || cards.IsManaAbilitySA(ab) || e.isLoyaltyAbility(ab) {
			return true
		}
		cost, ok := pay.FixLifeXCost(asPayer(e), p, o.Obj, e.abilityCompiledCost(ab).Cost)
		if !ok {
			return true
		}
		// The ability's own ReduceCost$ (planKeywordAbility's composition
		// step): the offer walk priced the composed cost with it.
		if n := e.ownReduceCostOffer(p, o.Obj, ab, 0); n > 0 {
			if cost.Generic >= n {
				cost.Generic -= n
			} else {
				cost.Generic = 0
			}
		}
		statics := e.collectCostStatics()
		composed := e.offerCostForUsing(statics, p, o.Obj, cost, abilityScope(ab))
		got := e.planComposedCost(p, o.Obj, composed, true, true)
		if got.Reason != "" && statics.validTarget {
			// A target-priced cost static may price cheaper than this
			// pre-target composition: the insufficiency is not the planner's
			// answer (planKeywordAbility's own guard).
			return true
		}
		if got.Reason == "" && got.Plan != nil && abilityPlanUnreservedPart(e.abilityCompiledCost(ab).Cost) &&
			!e.potentialWitnessOffers(p, o, *got.Plan) {
			// The witness consumed a candidate of one of the cost's
			// unreserved parts: not a way to pay it.
			return true
		}
		return got.Reason == ""
	case optUnlock:
		ob := e.G.Obj(o.Obj)
		fi, err := strconv.Atoi(o.Mode)
		if ob == nil || err != nil {
			return true
		}
		cost, ok := e.unlockRoomFaceCost(ob, fi)
		if !ok {
			return true
		}
		composed := e.offerCostForUsing(e.collectCostStatics(), p, o.Obj, cost, specialActionScope("unlock"))
		if got := e.planComposedCost(p, o.Obj, composed, true, false); got.Reason == "" || got.Reason == pay.ReasonInsufficient {
			return got.Reason == ""
		}
		return true
	case optSpecialize:
		ob := e.G.Obj(o.Obj)
		if ob == nil {
			return true
		}
		cost, ok := specializeCost(ob)
		if !ok {
			return true
		}
		if got := e.planComposedCost(p, o.Obj, cost, true, false); got.Reason == "" || got.Reason == pay.ReasonInsufficient {
			return got.Reason == ""
		}
		return true
	}
	return true
}

// grantedAbilitySA resolves a potential option's SVar-anchored ability body
// off the face whose SVar table the offer walk anchored it in: the grantor's
// face when the option names one (a granted-on-recipient activation's anchor
// lives on the grantor, the whole reason the anchor exists), else the
// source's own face (a max-speed grant of its own abilities). A stale anchor
// resolves nil and the caller keeps the walk's admission.
func (r *oracleRun) grantedAbilitySA(o decision.Option) *cards.SA {
	e := r.e
	ob := e.G.Obj(o.Obj)
	if ob == nil || ob.Face() == nil {
		return nil
	}
	f := ob.Face()
	if o.GrantSource != 0 {
		if g := e.G.Obj(o.GrantSource); g != nil && g.Face() != nil {
			f = g.Face()
		} else {
			return nil
		}
	}
	return cards.ResolveSVar(f.SVars, o.SVar)
}

// manaFeeFeasible prices a mana ability's activation fee the CR 602.2b way:
// the fee may be paid by activating mana abilities during the announcement,
// each source tapped at most once, and never the source the fee's own tap
// spends (CR 605.3b: the fee is paid before the mana is added, so the
// ability's own production cannot fund it). The ordered search is the CR
// 601.2g payment window's own (castWindowReachable over the shared fixed
// census, windowManaUnits): a row it cannot reach stays withheld (the census
// prices only the deterministic free shapes, so a fee only a paid source
// could fund is withheld, the safe direction), a row it reaches is funded by
// an executable source order. The full walk's every non-mana read (the
// sacrifice, discard, tap-sick and counter reads, which are
// pool-independent) is taken first against an unbounded pool, so only the
// mana half is re-priced here.
func (r *oracleRun) manaFeeFeasible(p state.PlayerID, id state.ObjID, cc *compiledCost, units []windowManaUnit) bool {
	e := r.e
	// The unbounded pool makes the full walk's mana half trivially payable,
	// so its verdict here is exactly its non-mana reads (the same shape
	// verifyPotentialSkip prices with).
	huge := state.Mana{1 << 28, 1 << 28, 1 << 28, 1 << 28, 1 << 28, 1 << 28}
	if !e.manaCostPayable(p, e.G.Obj(id), id, cc, &huge) {
		return false
	}
	desc := paymentFor(id, true, cc.Cost)
	av := pay.AvailableFor(asPayer(e), p, desc)
	feeUnits := units
	if cc.Cost.Tap || cc.Cost.Untap || len(cc.Cost.Sac) > 0 {
		// The fee's own tap or untap spends the source, and a sacrifice part
		// (self or not) may consume it mid-payment (CR 605.3b: the fee is
		// paid before the mana is added, so the source's own production
		// funds nothing): its unit stays out of the search.
		feeUnits = make([]windowManaUnit, 0, len(units))
		for _, u := range units {
			if u.ID != id {
				feeUnits = append(feeUnits, u)
			}
		}
	}
	pl := &e.G.Players[p]
	conv := asEval(e).Conv(p, desc.ID, desc.SAKind())
	return e.castWindowReachable(p, cc.Cost, av.Pool, pl.Snow, av.Typed, pl.Life, conv, pipRider{}, feeUnits)
}

// potentialManaLabels is manaAbilityLabels with the payability gate priced
// the CR 602.2b way, so an ability the floating pool cannot pay but the
// untapped sources can during the announcement is reported beside the ones
// the priority offer already lists. Printed and intrinsic members come from
// the choke point with its live-pool gate deferred
// (appendAvailableManaAbilitiesGate, the same membership PotentialMana
// records); granted mana members (an AddAbilities grant's AB$ Mana) come
// from grantedAbilities under manaGrantedGatesHold, the one non-payability
// gate home the choke point's granted block shares, so the two lists cannot
// drift. Every member is priced the same way: the live-pool gate
// (manaCostPayable, the full walk against the real pool) beside
// manaFeeFeasible, the per-source-exclusive read of the fee -- the pooled
// untapped-source bound itself is never the answer, because it counts both
// of a dual's faces and the production of a source the fee's own tap spends.
// Both halves label through the same ManaAbilityLabel manaAbilityLabels
// uses, so an ability the floating pool already pays dedupes against the
// priority offer's row instead of duplicating it. A granted ManaReflected
// member is left to the choke point's own read (its candidate walk is a
// separate surface).
func (r *oracleRun) potentialManaLabels(seat state.PlayerID, id state.ObjID, units []windowManaUnit) []string {
	var labels []string
	o := r.e.G.Obj(id)
	price := func(ma *cards.SA, cc *compiledCost) bool {
		if r.e.manaCostPayable(seat, o, id, cc, nil) {
			return true
		}
		return r.manaFeeFeasible(seat, id, cc, units)
	}
	for _, ma := range r.e.appendAvailableManaAbilitiesGate(nil, nil, seat, id, true) {
		cc := r.e.abilityCompiledCost(ma)
		if mf := r.e.manaFactsOf(ma); mf != nil {
			cc = mf.cost
		}
		cc = r.e.pricedManaCC(seat, id, ma, cc)
		if price(ma, cc) {
			labels = append(labels, pay.ManaAbilityLabel(ma, pay.ChosenProducedColour(r.e.G, id)))
		}
	}
	for _, ga := range r.e.grantedAbilities(seat, id) {
		ma := ga.SA
		if ma == nil || ma.API == "ManaReflected" || !cards.IsManaAbilitySA(ma) || r.e.isLoyaltyAbility(ma) {
			continue
		}
		if !manaGrantedGatesHold(r.e, o, seat, id, ma, r.e.abilityRestricted(seat, id, ma)) {
			continue
		}
		cc := r.e.abilityCompiledCost(ma)
		if mf := r.e.manaFactsOf(ma); mf != nil {
			cc = mf.cost
		}
		cc = r.e.pricedManaCC(seat, id, ma, cc)
		if price(ma, cc) {
			labels = append(labels, pay.ManaAbilityLabel(ma, pay.ChosenProducedColour(r.e.G, id)))
		}
	}
	return labels
}
