package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Scripted plays (Bloom Tender). gorge's payment-planner census cannot
// price a "one mana of each colour among permanents you control" ability,
// so with one untapped the census is incomplete: a potential play the
// planner proves insufficient gets a prefix search (rules
// PotentialPaymentPlans -> potentialPrefixScript) that activates those
// sources first; the play is offered when the planner then pays it, and the
// oracle plays the prefix, then the play (prScript). gorge searches only the
// first potentialScriptPlays insufficient plays of its potential walk, and
// under the autopay surface the oracle drops a scripted plain cast from the
// hand (Config.GorgeAutopaySurface). The kernel mirrors it: the plays
// listed with any mana available but not with the sources available are the
// insufficient ones, in priority-walk order; the first potentialScriptPlays are searched; a
// searched play the sources plus the gaps' mana pay is offered, and at the
// play the gaps are tapped into the pool in battlefield order, an any-colour
// counter ability branched on its colour ask (mana_countercost.go ctrScript),
// until the plain walk lists it.

const potentialScriptPlays = 16 // gorge rules/potential_script.go

// availFor modes (genState.availX): the gaps' mana added, any mana, or the
// plain walk without the script hook.
const (
	availGaps  uint16 = 1
	availAny   uint16 = 2
	availPlain uint16 = 4
	availWalk  uint16 = 8    // the real mana, no offer surface (grantCap)
	availHyp   uint16 = 128  // with availWalk: gorge's PotentialMana bound, a choice source counting every colour it can make (mana_owncolours.go)
	availOwn   uint16 = 1024 // with availGaps: an activation's own {T} source stays in the census, as gorge's PotentialMana bound counts it (mana_ownwalk.go)
)

// scriptGaps: p's activatable each-colour sources, a slot mask.
func (s *State) scriptGaps(k *prep, p uint8) slotMask {
	if !k.gen.anyEachCol {
		return slotMask{}
	}
	return s.gapMask(k, p, gfEachCol)
}

// liveGrantGap reports whether p controls an activatable granted mana
// ability: the grant search then lists the board's filters among its
// candidates (grantSrch.candidates), so one search activates both gaps in
// battlefield order.
func (s *State) liveGrantGap(k *prep, p uint8) bool {
	if k.gen == nil || len(k.gen.grantPC) == 0 || !s.grantsLive(k) {
		return false
	}
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		if s.grantGapUsable(k, sl) {
			return true
		}
	}
	return false
}

// scriptRestr: p's activatable restricted-mana sources (Giada) that are
// census gaps, a slot mask. None: gorge's census lists a fixed RestrictValid$
// source since (85fddcb68), so a potential play's prefix search no
// longer activates it; the manual mana window still does (windowRestr).
func (s *State) scriptRestr(k *prep, p uint8) slotMask {
	if !k.gen.anyRestrScript {
		return slotMask{}
	}
	return s.gapMask(k, p, gfRestrScript|gfDeferred)
}

// windowRestr: p's activatable restricted-mana sources (Giada), the ward /
// unless window's: it activates every one in battlefield order, planner
// unit or not (gorge's windowManaUnits keeps no restriction).
func (s *State) windowRestr(k *prep, p uint8) slotMask {
	if !k.gen.anyRestr {
		return slotMask{}
	}
	return s.gapMask(k, p, gfRestr)
}

// scriptAny reports whether a scripted prefix is possible: an each-colour,
// restricted or any-colour gap, a filter-only gap, a counter-cost source, or
// a Taps-deferred one (mana_tapsdefer.go).
func (s *State) scriptAny(k *prep) bool {
	return k.gen != nil && (k.gen.anyScriptGap || k.sacOnlyMana || k.ctrMana || k.gen.anyTapsDefer || k.gen.anyDiscHand || k.gen.anyComboAny || k.gen.uses.has(useLifeGap) || k.gen.uses.has(useAnyDifferent))
}

// gapMask: p's untapped permanents whose card carries the sparse mana
// ability bit (gfEachCol, gfRestr, gfAnyR, gfFilt, gfGapOther) and can be
// activated now.
func (s *State) gapMask(k *prep, p uint8, bit uint32) slotMask {
	var m slotMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if !k.gen.has(s.cardAt(sl), bit) {
			continue
		}
		pm := &s.perms[sl]
		if s.pickedForMana(pm.obj) {
			continue // a Convoke/Waterbend pick is tapped for that cast, not for mana
		}
		if pm.flags&pfTapped != 0 || s.activationSick(k, sl) || s.genLost(k, sl) ||
			k.gen.anyCantAct && s.actForbidden(k, sl, true) || // a CantBeActivated lock (cast_cantactivate.go)
			k.gen.anyDeferred && k.gen.has(s.cardAt(sl), gfDeferred) && (!s.deferLifeSurvives(k, p, sl) || s.deferNoMana(k, sl)) {
			continue
		}
		m = m.with(sl)
	}
	return m
}

// eachColorOut is the mana gap sl adds: one of each colour among p's
// permanents matching its filter (effects eachColorAmongValid).
func (s *State) eachColorOut(k *prep, p uint8, sl Slot) kop.ColorMask {
	a := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.eachCol(s.cardAt(sl))]
	var m kop.ColorMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		o := b.first()
		if s.matchPerm(k, a.ManaAmong, o, p, sl) {
			m |= s.colorsAt(k, o)
		}
	}
	return m & kop.ColAny
}

// availScript adds the availFor mode's extra mana.
func (s *State) availScript(k *prep, p uint8, a *avail) {
	if s.g.availX&availAny != 0 {
		a.addUnits(0x3f, 99)
		return
	}
	if k.gen.anyDiscHand && s.dhBoard(k, p) {
		s.availDH(k, p, a) // a Lion's Eye Diamond's relaxed units (mana_discardhand.go)
	}
	if k.gen.anyComboAny && s.caBoard(k, p) {
		s.availCA(k, p, a) // a Combo Any source's any-colour units (mana_comboany.go)
	}
	for b := s.scriptGaps(k, p); b.any(); b = b.rest() {
		s.eachGapUnits(k, p, b.first(), a)
	}
	// a life-only source (Treasonous Ogre): the prefix search's gap unit, one
	// of its fixed colour or of any. Only the gaps walk counts it (availGaps;
	// availScript is called for no availWalk mode), never the plain walk. It
	// counts each source's unit independently of the shared life total, an
	// over-count a potential walk may make: scriptDFS fails an activation a
	// path has made unpayable and backtracks, so no offer survives it.
	s.availLifeGap(k, p, a)
	// a restricted source (Giada): the prefix search prices with its mana
	// as with any gap's (the restriction is not read there), and an Angel
	// spell may then spend it (scriptDeadEnd decides)
	for b := s.scriptRestr(k, p); b.any(); b = b.rest() {
		c := s.cardAt(b.first())
		if k.gen.has(c, gfDeferred) {
			a.addUnits(s.deferMask(k, b.first()), 1) // the deferred source's plain mana (mana_deferred.go)
			continue
		}
		ab := &k.t.Cards[c].Abilities[k.gen.restr(c)]
		a.addUnits(1<<bits.TrailingZeros8(uint8(ab.ManaColors)), max(ab.ManaN, 1))
	}
	// a Taps-deferred source (Firdoch Core): its relaxed census unit, the
	// one-mana any/fixed-colour tap it adds (availFor's, which the plain walk
	// leaves out: mana_tapsdefer.go)
	for b := s.scriptTapsDefer(k, p); b.any(); b = b.rest() {
		if a.capped && !s.tapsDeferGapOwn(k, b.first()) {
			continue // availForCapped's loop counted this tap already: once per source, at its largest production
		}
		s.addTapsDeferUnits(k, b.first(), a)
	}
	if k.ctrMana {
		s.availCtr(k, p, a) // a counter-cost source (mana_countercost.go)
	}
	if k.tapMana {
		s.availTapX(k, p, a) // a tap-another-permanent source (mana_tapanother.go)
	}
	if k.gen.anyAnyR {
		s.availAnyR(k, p, a)
	}
	// a Firemind Vessel (AnyDifferent) is a census gap the prefix search
	// pays: its N mana, each any of the five colours here (the distinct-colour
	// rule is the search's, scriptDeadEndAny), an over-bound like the census's
	// (mana_distinctcolours.go)
	for b := s.anyDifferentGaps(k, p); b.any(); b = b.rest() {
		a.addUnits(0x1f, anyDifferentUnits(s.anyDifferentAbilityAt(k, b.first())))
	}
	if k.gen.anyFilt && s.g.availX&availNoFilt == 0 && (s.filtOnly(k, p) || s.mixBoard(k, p)) {
		s.availFilt(k, p, a) // (beside another gap kind the filter stays unmodelled: mana_filter.go)
	}
}

// plainOptions is p's priority walk without the script hook, under
// availFor mode m. The availPlain mark is the probe's: the walks that model
// gorge's planner (the Phyrexian offer gate's storage-battery read,
// mana_phyrexian.go phyActAffords) read without the batteries' counters,
// which the planner reaches only through the script route (pair 149).
func (s *State) plainOptions(k *prep, d *Decision, m uint16) {
	d.N = 0
	s.g.availX = availPlain | m
	s.priorityOptions(k, d)
	s.g.availX = 0
}

func hasOption(d *Decision, a Action) bool {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == a {
			return true
		}
	}
	return false
}

// scriptRelax reports whether gorge's census is relaxable on p's board.
// unrelax holds the live sources that make it unrelaxable on their own: a
// two-permanent source (Baylen), a Combo Any gap, a sacrifice-any one
// (Krark-Clan Ironworks: tapXSacAnyBound). So does a fourth filter, or a
// dynamic one out of bounds (mana_filter.go filtRelaxable). A live dead
// counter-cost gap (Component Pouch) is unrelaxable as well: gorge's census
// records it while the cost is payable and paymentPlanRelaxedAlternatives
// enumerates no colour for its production (potential_plan.go), so the prefix
// search runs on the any-mana walk and the search's slot accounting counts
// every insufficient play whatever it finds; the dead gap contributes no units
// to either walk (availCtr).
func (s *State) scriptRelax(k *prep, p uint8, filt slotMask, unrelax bool) bool {
	return !unrelax && !s.scriptGaps(k, p).any() && s.filtRelaxable(k, filt) && !s.deadCtrMask(k, p).any()
}

// addCostShapeUnscripted reports whether the plain cast of card c carries a
// mandatory additional-cost shape gorge prices only on its census: the cast
// planner declines it ("shape:additional_cost", paymentPlanCastShapeDetail's
// PlanNonManaAdmissible arms) and the composition is its only witness, an
// insufficient one dropped with no prefix search for it
// (potentialPlayVerdictShared). The scriptable exceptions keep their plays:
// an either-or part (AlternateAdditionalCost), fixed-count tapXType extras
// (spellExtrasAreFixedTaps) and an announced PayLife<X> alone
// (spellExtrasAreAnnouncedLife); the announced Sac<X> keeps its pool arm
// (cost_sacx.go).
func addCostShapeUnscripted(k *prep, c CardID) bool {
	ac := spellAddCost(k, c)
	if sacXCost(ac) || !planDeclinesAddCost(k, c) {
		return false
	}
	cd := &k.t.Cards[c]
	return !cd.HasAltAdd && !spellFixedTapsOnly(k, c) &&
		!(lifeXCost(ac) && ac.Flags == kop.CostPayLife && manaEmpty(&ac.Mana)) &&
		!discardXOnlyShape(k, c)
}

// discardXOnlyShape reports a card with no printed {X} whose spell ability adds
// nothing to its mana but an announced Discard<X/Spec> (Vengeful, Restless and
// Nostalgic Dreams). gorge's potential planner keeps the composition's verdict
// for it (potential_plan.go spellExtrasAreAnnouncedDiscardX), so an insufficient
// composition goes on to the census and the prefix search like any cast's:
// X = 0 pays nothing extra and a source a prefix floats pays the mana (CR
// 601.2b, 601.2g). With a printed {X} the shape stays census-only (Abandon Hope).
func discardXOnlyShape(k *prep, c CardID) bool {
	ac := spellAddCost(k, c)
	return k.t.Cards[c].Cost.X == 0 && discardXCost(ac) &&
		ac.Flags&^(kop.CostDiscard|kop.CostDiscardFiltered) == 0 && !ac.ExileBf
}

// plainCastUnsearched is the script walk's two plain-cast drops: the printed
// sources pay the cast but its largest-X witness eats a target (gorge's
// verdict is "witness_unreachable", mana_sacsource.go), and the spell's own
// Cost$ carries an additional-cost shape gorge prices only on its census, an
// insufficient one dropped and never scripted (potentialPlayVerdictShared);
// the announced Sac<X> keeps its pool arm (cost_sacx.go).
func (s *State) plainCastUnsearched(k *prep, p uint8, a Action) bool {
	if a.Kind() != AkCast || a.Mode()&3 != 0 || a.Mode()&(castMayPlay|castFace|castWarp) != 0 {
		return false
	}
	if s.xWitnessUnreachable(k, p, Obj(a.Source())) {
		return true
	}
	c := s.objs[Obj(a.Source())].card
	return addCostShapeUnscripted(k, c)
}

// offerSkip is the script walk loop's drop state: the loop-invariant masks
// scriptOfferDrops prices a play against.
type offerSkip struct {
	d, capSpent, gaps, noWide                 *Decision
	sacAny, sacRepeat, wide, mixWide, anyWalk bool
	anyR                                      slotMask
}

// scriptOfferDrops reports whether the walk loop's play a is not offered in
// the decision: already on it or on the spent cap, dropped by the sacrifice-
// any / repeat arms, unfundable by a wide source (the relaxed proof's drop,
// and the mixed-board noWide arm), or — on the any-mana walk — a cast the
// payment bound built with its restrictions applied cannot afford
// (restrBoundDrops).
func (s *State) scriptOfferDrops(k *prep, p uint8, a Action, o *offerSkip) bool {
	return !isScriptPlayKind(a.Kind()) || hasOption(o.d, a) || hasOption(o.capSpent, a) ||
		s.sacDrops(k, p, o.sacAny, o.sacRepeat, o.gaps, a) ||
		o.wide && !s.wideAdmits(k, a, p, o.anyR) ||
		o.mixWide && !hasOption(o.noWide, a) && !s.wideAdmits(k, a, p, s.scriptWide(k, p)) ||
		o.anyWalk && s.restrBoundDrops(k, p, a)
}

// scriptOffers adds to d the plays a scripted prefix pays.
func (s *State) scriptOffers(k *prep, d *Decision, capSpent *Decision, p uint8) {
	restr, anyR := s.scriptRestr(k, p), s.scriptAnyR(k, p)
	tapsDef := s.scriptTapsDefer(k, p)
	var filt slotMask
	mix := s.mixBoard(k, p)
	if k.gen.anyFilt {
		// (filtOnly; a restricted gapOther whose mana no play the walk lists
		// can spend keeps no filter off the walk: mana_filter.go restrGapOthers)
		if f := s.scriptFilt(k, p); f.any() && (mix || !s.gapOthers(k, p).andNot(s.restrGapOthers(k, p)).any()) {
			filt = f
		}
	}
	tx := s.scriptTapXModelled(k, p).or(s.scriptCtrModelled(k, p))
	dh := k.gen.anyDiscHand && s.dhBoard(k, p) // Lion's Eye Diamonds the only gaps (mana_discardhand.go)
	ca := k.gen.anyComboAny && s.caBoard(k, p) // a Combo Any source among the gaps (mana_comboany.go)
	var caM slotMask
	if ca {
		caM = s.caGaps(k, p)
	}
	anyDiff := s.anyDifferentGaps(k, p) // a Firemind Vessel among the gaps (mana_distinctcolours.go)
	life := s.lifeGapMask(k, p)         // a life-only mana ability among the gaps (mana_lifegap.go)
	if !dh && !ca && !s.scriptGaps(k, p).or(restr).or(anyR).or(tapsDef).or(filt).or(s.scriptCtr(k, p)).or(tx).or(anyDiff).or(life).any() {
		return
	}
	// With only restricted gaps (Giada, a Courtyard) gorge's census is
	// relaxable (each has a fixed relaxed alternative; a Tender's has
	// none): a play the relaxed proof cannot pay, the gaps walk's, is
	// insufficient and no search, so it takes none of the searches
	// (potential_plan.go paymentPlanRelaxProof), and the candidates are
	// the gaps walk's alone (one walk, not two).
	// A live Baylen makes the census unrelaxable (gorge paymentPlanRelaxedAlternatives,
	// no once-only cost): every walk play is searched; so is every play beside a live
	// Krark-Clan Ironworks when the potential bound is open. On a closed bound the
	// Ironworks' plays are the gaps walk's and those gorge's PotentialMana bound
	// pays on its own (sacAny; tapXSacAnyBound; sacPotPays). A closed-bound
	// sacrifice-any one whose cost sacrifices dedicated tokens (Jungle Patrol's
	// "Sacrifice a token named Wood: Add {R}") is the Baylen shape, not this one:
	// once is false there too, gorge runs no bounded proof, and every
	// census-incomplete play takes a search slot (sacRepeatLive). One that
	// sacrifices a permanent type (Krark-Clan Ironworks' artifact) is not: its
	// fodder can be a mana source the bound already prices.
	sacAny, sacOpen := s.tapXSacAnyBound(k, p)
	sacRepeat, twoLive := s.sacRepeatBoard(k, p, sacOpen)
	relax := s.scriptRelax(k, p, filt, ca || twoLive || sacAny)
	// A live repeatable fee-only filter (Nomadic Elf) has no relaxed or bounded
	// proof in gorge either: it searches every insufficient play, and the gaps
	// walk (one conversion per filter) cannot say which a many-times-activated
	// one pays (mana_filterrepeat.go).
	repeatLive := s.filtRepeatIn(k, filt)
	var any, gaps Decision
	s.plainGapsOptions(k, &gaps, filt)
	s.mergeBareGaps(k, &gaps, filt)
	src := &gaps
	if s.scriptAnyWalk(k, p, filt, relax, ca || twoLive) {
		s.plainOptions(k, &any, availAny)
		src = &any
	}
	// the insufficient plays in gorge's walk order (potentialWalkOf: the
	// hand walk, the permission and graveyard casts, then the battlefield's
	// activations in entry order): the kernel's walk lists activations in
	// slot order
	var cand [MaxOptions]Action
	var key [MaxOptions]uint64
	nc := 0
	wide := !mix && !tapsDef.any() && !ca && s.wideGapsOnly(k, p, restr, anyR) // (gorge's relaxed proof reads a wide source against each play; a Taps-deferred gap is none of them)
	// The drop of a play no wide source may fund is the relaxed proof's
	// (potentialRestrictionAdmits): an unrelaxable census (a live
	// two-permanent source, Supportive Parents) runs no proof, so gorge
	// searches every play the walk lists, a wide source's mana counted.
	mixWide := mix && relax && s.scriptWide(k, p).any()
	var real Decision // the plays the printed sources pay, plain casts the autopay surface drops included
	if s.cfg&cfgGorgeAutopay != 0 {
		s.plainOptions(k, &real, availWalk)
	}
	var noWide Decision
	if mixWide {
		s.plainOptions(k, &noWide, availGaps|availNoWide) // (gorge's potential walk counts a wide source only for what its restriction admits)
	}
	skip := offerSkip{
		d: d, capSpent: capSpent, gaps: &gaps,
		sacAny: sacAny, sacRepeat: sacRepeat,
		wide: wide, anyR: anyR, mixWide: mixWide, noWide: &noWide,
		anyWalk: src == &any,
	}
	for i := 0; i < int(src.N); i++ {
		a := src.Options[i]
		if s.scriptOfferDrops(k, p, a, &skip) {
			continue
		}
		if a.Kind() == AkCast && a.Mode()&3 == 0 && a.Mode()&(castMayPlay|castFace|castWarp) == 0 && a.Target() == 0 && hasOption(&real, a) {
			continue // priced by the printed sources: gorge's planner finds it sufficient and runs no prefix search for it
		}
		if a.Kind() == AkCast && a.Mode()&3 == 0 && a.Mode()&(castMayPlay|castFace|castWarp) == 0 && s.plainCastUnsearched(k, p, a) {
			continue // gorge runs no prefix search for it: the printed sources pay it but the witness eats a target (witness_unreachable, mana_sacsource.go), or the shape is additional_cost and the composition is its only witness, an insufficient one dropped (potentialPlayVerdictShared)
		}
		if cd := &k.t.Cards[s.objs[Obj(a.Source())].card]; isCastKind(a.Kind()) && (s.spellTgtDep(k, p, s.objs[Obj(a.Source())].card, Obj(a.Source())) || cd.HasModeCost && a.Mode()&castMayPlay == 0 && !s.modalFloorFree(k, p, s.objs[Obj(a.Source())].card)) {
			continue // gorge searches no prefix for a target-priced cost, nor for a modal one whose cheapest mode still costs mana (a may-play cast is priced at its printed cost: potentialPlayVerdictShared's mayplay arm)
		}
		if isCastKind(a.Kind()) && a.Mode()&castMayPlay == 0 && k.gen.anyOptCost && optCostCard(k, s.objs[Obj(a.Source())].card) {
			continue // nor for an OptionalCost card: the planner declines it (shape:optional_cost, cost_optional.go); a may-play cast it composes (potentialModeCastPlan)
		}
		if a.Kind() == AkCast && a.Mode()&3 == 0 && a.Mode()&castMayPlay == 0 && k.t.Cards[s.objs[Obj(a.Source())].card].HasMultikicker {
			continue // nor for a Multikicker card's plain cast: gorge's planner declines the face (payment_plan.go "shape:optional_cost"), so a plain cast only a census gap pays is not offered (Gnarlid Pack beside Jaspera Sentinel); the multikicked cast keeps its own offer gate (legal_walk_hand.go phMultikicker)
		}
		if s.scriptShapeSkip(k, p, a) {
			continue // nor for an exile, forage, return, blight or evidence part (shape:additional_cost; objPartCast), nor a contribution (shape:contribution); a forage-or-pay either-or (Feed the Cycle) stays with gorge's offer gate, which searches the filter script for its paid part (payment_plan.go altAdd)
		}
		kk := s.walkKey(a, p, i)
		j := nc
		for ; j > 0 && key[j-1] > kk; j-- {
			cand[j], key[j] = cand[j-1], key[j-1]
		}
		cand[j], key[j] = a, kk
		nc++
	}
	var lifeWalk, ownWalk Decision
	lifeBuilt, ownBuilt := false, false
	searched := 0
	for i := 0; i < nc && searched < potentialScriptPlays; i++ { // the first potentialScriptPlays: searched
		a := cand[i]
		if !s.gapsOrBoundPlay(k, p, sacAny, &gaps, a) && !twoLive && !repeatLive && !s.ownTapWalkPlay(k, p, a, &ownWalk, &ownBuilt) {
			if !relax {
				searched++
			}
			continue
		}
		searched++
		if s.autopayDropsPlainCast(a) && !s.lifePlanned(k, p, life, &lifeWalk, &lifeBuilt, a) && !s.phyHalfPlanned(k, p, a) {
			continue // a plain cast: dropped by the autopay surface unless gorge's planner prices it with a life-only source (mana_lifeplan.go)
		}
		if dh {
			if !s.dhScript(k, a, p, false) {
				continue // no script: the discarded hand held the play, or three mana of one colour do not pay it (mana_discardhand.go)
			}
		} else if ca {
			if !s.caSearch(k, a, p, restr, anyR, tapsDef, caM) {
				continue // no script within gorge's search: no colour answers pay the play (mana_comboany.go)
			}
		} else if (filt.any() || mix) && s.liveGrantGap(k, p) {
			// a cast beside a filter and a live grant needs both: the grant
			// search's filter step funds the filter's fee from a granted
			// sacrifice ability (Rain of Filth's lands; grantSearch's
			// stepFilt), which the filter-only script skips (grantHasCost).
			if a.Kind() == AkActivate {
				// An activation's line the grant search prices may fund the
				// filter's fee from a granted tap — and gorge's menu arm
				// prices an activation from the floating pool or the payment
				// planner, whose census of sources holds printed abilities
				// only (pay/board.go PaymentAbility). A line that floats a
				// grant's mana before the fee prices a set gorge's planner
				// never sees, so the grant search prices no activation here:
				// the filter-only script prices the lines the planner does
				// (its floats are the printed sources', CR 601.2b; tail-26
				// pair 35: the Merfolk's {1}{U} through Henge of Ramos's
				// "{2},{T}: one mana of any color", whose fee the grant
				// search funded with two granted Elf taps).
				if !s.filtScript(k, a, p, false) {
					continue // no script within gorge's search budget (filtScript)
				}
			} else if ga, gstall := s.grantActScript(k, a, p); !ga {
				if isCastKind(a.Kind()) && s.filtScript(k, a, p, false) {
					// the filter-only script prices it (grantHasCost)
				} else {
					// the drop is final — grantCap ran before scriptOffers
					// (legal.go) — and one of the grant search's lines ran
					// into a rule gap the kernel stalls by name: declare
					// it, not a dropped offer (the search declares
					// nothing, mana_grantedoffers.go grantSearch)
					s.capStall(gstall)
					continue // no script within gorge's one search's budget (grantActScript)
				}
			}
		} else if filt.any() || mix {
			if !s.filtScript(k, a, p, false) {
				continue // no script within gorge's search budget
			}
		} else if tapsDef.any() && s.liveGrantGap(k, p) {
			if ga, gstall := s.grantActScript(k, a, p); !ga {
				// the drop is final and one of the grant search's lines
				// ran into a rule gap the kernel stalls by name: declare
				// it, not a dropped offer (the search declares nothing)
				s.capStall(gstall)
				continue // the grant search is gorge's one search over the deferred gap and the grants (mana_granttapsdefer.go); a cast too: Quest for Renewal's Taps watcher defers Gold Myr, so five mana from two Mountains, the Myr and two Saprolings under Brightcap Badger pays Quakebringer only by that search
			}
		} else if tx.any() && !s.tapXDeadEndRoute(k, p, tx, tapsDef) && s.tapXBoard(k, p) {
			if offered, stall := s.tapXScriptOffered(k, a, p); !offered {
				// every route gave the play up and one of the search's lines
				// ran into a rule gap the kernel stalls by name: declare it,
				// not a dropped offer. The drop is final — grantCap ran
				// before scriptOffers (legal.go) — and the search itself
				// declares nothing (mana_tapanother.go tapXScript).
				s.capStall(stall)
				continue // no script within gorge's budget (mana_tapanother.go), nor in the one search beside the grants (mana_granttapx.go, mana_grantsacx.go)
			}
		} else if k.gen.anyRefl && s.reflBoard(k, p, restr, anyR) {
			if found, dead := s.reflScript(k, a, p, false); !found || dead {
				continue // no script within gorge's budget, or one the pool cannot complete (mana_reflected.go)
			}
		} else if anyR.any() || tapsDef.any() || anyDiff.any() || life.any() {
			if s.scriptDeadEndAny(k, a, p, restr, anyR, tapsDef, s.scriptCtr(k, p), anyDiff) {
				continue // no script within gorge's budget (mana_window.go scriptDeadEndAny)
			}
		} else if !restr.any() && !s.scriptCtr(k, p).any() && s.eachFeeBoard(k, p) {
			if !s.eachScript(k, a, p, false) {
				continue // no script within gorge's search budget (mana_eachfee.go)
			}
		} else if restr.any() || s.scriptCtr(k, p).any() {
			if dead, stall := s.scriptDeadEnd(k, a, p, restr); dead {
				// the drop is final and one of the counter-cost search's
				// lines ran into a rule gap the kernel stalls by name:
				// declare it, not a dropped offer (the search itself
				// declares nothing: scriptDeadEnd runs ctrScript on a fork,
				// and unfork discards a declaration made there; tapXScript's
				// shape)
				s.capStall(stall)
				continue // no script within gorge's budget (mana_countercost.go ctrScript; a counter-cost gap alone, no restricted source beside it)
			}
		}
		d.add(a)
	}
}

// mergeBareGaps adds to gaps the plays of the gaps walk with the filters left
// unactivated: a play the other sources pay once the gaps are tapped is in
// gorge's relaxed proof, whose subsets include the empty one (availNoFilt).
func (s *State) mergeBareGaps(k *prep, gaps *Decision, filt slotMask) {
	if !filt.any() {
		return
	}
	var bare Decision
	s.plainOptions(k, &bare, availGaps|availNoFilt)
	for i := 0; i < int(bare.N); i++ {
		if !hasOption(gaps, bare.Options[i]) && int(gaps.N) < MaxOptions {
			gaps.add(bare.Options[i])
		}
	}
}

// tapXScriptOffered searches play a for the tap-another script, and when that
// finds none, for the grant search beside the grants it covers: a
// second-ability tap-another source's (tapXSecondLive) or a counter-cost gap
// (grantBesideTapXGaps), and a sacrifice-another source's (sacXBesideGrant) —
// gorge's one prefix search activates every gap of the board, the grants
// beside its census gaps (rules scriptSearch.visit; mana_granttapx.go,
// mana_countercost.go, mana_grantsacx.go). It reports the tap-another search's
// latched stall (tapXScript) only when every route failed: a stall a later
// route priced past must not reach the real state (the caller declares it
// where it drops the play).
func (s *State) tapXScriptOffered(k *prep, a Action, p uint8) (bool, uint8) {
	found, stall := s.tapXScript(k, a, p, false)
	if found {
		return true, 0
	}
	if g, gstall := s.grantBesideTapXGaps(k, a, p); g {
		return true, 0
	} else if stall == 0 {
		stall = gstall // every route failed and the grant search's line latched
	}
	if s.sacXBesideGrant(k, p) {
		f, gstall := s.grantActScript(k, a, p)
		if f {
			return true, 0
		}
		if stall == 0 {
			stall = gstall
		}
	}
	return false, stall
}

// autopayDropsPlainCast reports a plain cast the autopay surface leaves out of
// a potential play's list: gorge's planner prices it or the floating pool pays
// it, else the oracle drops it (gorge.go, "a plain cast the planner does not
// price"), whatever prefix a storage battery's or a grant's activation makes
// of it.
func (s *State) autopayDropsPlainCast(a Action) bool {
	return s.cfg&cfgGorgeAutopay != 0 && a.Kind() == AkCast && a.Mode()&3 == 0 && a.Mode()&(castMayPlay|castFace|castWarp) == 0 && a.Target() == 0
}

// walkKey orders insufficient play a (the kernel walk's option i) as
// gorge's potential walk lists it.
func (s *State) walkKey(a Action, p uint8, i int) uint64 {
	ob := &s.objs[Obj(a.Source())]
	switch {
	case (isCastKind(a.Kind()) || a.Kind() == AkPlot || a.Kind() == AkSneak || a.Kind() == AkBestow) && ob.zone == kop.ZoneHand:
		for h := 0; h < int(s.pl[SeatIx(p)].handN); h++ {
			if s.pl[SeatIx(p)].hand[h] == Obj(a.Source()) {
				k := uint64(h)<<16 | uint64(i)
				if a.Mode() == 0 && a.Target() != 0 || a.Kind() == AkPlot || a.Kind() == AkSneak || a.Kind() == AkBestow || a.Kind() == AkCast && a.Mode()&castWarp != 0 {
					k |= 1 << 15 // an alternative cost's cast follows the card's plain one, though the kernel offers it first
				}
				return k
			}
		}
		return 1<<48 | uint64(i)
	case isCastKind(a.Kind()) && ob.zone != kop.ZoneGraveyard:
		return 1<<48 | uint64(i)
	case isCastKind(a.Kind()):
		// the graveyard casts in graveyard order, a flashback among the
		// permission casts (gorge's walk; pair 45914)
		return 2<<48 | uint64(s.zoneIndex(Obj(a.Source())))<<16 | uint64(i)
	case ob.zone == kop.ZoneBattlefield:
		return 3<<48 | uint64(s.perms[ob.slot].ts)<<16 | uint64(i)
	}
	return 4<<48 | uint64(i)
}

// scriptDeadEnd reports whether gorge's prefix search for play a ends in a
// script the oracle cannot complete, and the first rule-gap stall a failed
// line's fork latched (StallNone none). The search (scriptSearch.visit)
// activates the census gaps depth first in priority-option order, which is
// battlefield order: Bloom Tenders and restricted sources (Giada) alike.
// The planner prices the play from the floating pool without reading the
// restriction (Giada's Angel-only {W} counts toward Squad Rallier's
// {2}{W}), so the script is the battlefield-order chain cut where the walk
// lists a. Played, the planner reads the pool through the restriction, so the
// oracle offers a only when the walk lists it with the chain's mana floating,
// else "script dead end" (oracle gorge.go). Evaluated on a copy, so a stall
// ctrScript latches there is returned for the caller to declare where its drop
// is final, not declared on the copy.
func (s *State) scriptDeadEnd(k *prep, a Action, p uint8, restr slotMask) (bool, uint8) {
	t := s.fork()
	defer s.unfork()
	// An activation's granted mana is the grant search's (grantCap's walk),
	// but availFor does not read a grant's spend rule: count the unit here
	// only when every live grant is unrestricted. A deferred source
	// (Pyromancer's Goggles) is no census unit, so a play the deferred gap
	// and an unrestricted grant together pay is absent from that walk and
	// must be priced here (restrictedGrantLive).
	noGrant := a.Kind() == AkActivate && s.restrictedGrantLive(k, p)
	t.g.noGrant = noGrant
	var order [MaxPerm]Slot
	ctr := t.scriptCtr(k, p) // a counter-cost gap (Ramos) among them, as scriptPrefix plays it
	n := t.battlefieldOrder(p, t.scriptGaps(k, p).or(restr).or(ctr), order[:], 0)
	var path [MaxPerm]uint8
	_, found, dead := t.ctrScript(k, a, p, order[:n], ctr, restr, path[:], noGrant)
	stall := t.g.stalled // the failed search declared on this fork; read before the deferred unfork
	return !found || dead, stall
}

// restrictedGrantLive: p controls a source whose granted mana ability carries
// a spend restriction (Mm'menon's spell-only blue). availFor counts granted
// units without reading that rule, so a prefix search must keep them out
// (noGrant) unless every live grant is unrestricted.
func (s *State) restrictedGrantLive(k *prep, p uint8) bool {
	if k.gen == nil || len(k.gen.grantRestr) == 0 {
		return false
	}
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		var gi [maxGrants]int16
		for i, n := 0, s.grantIdxs(k, m.first(), &gi); i < n; i++ {
			if k.gen.grantRestr[gi[i]] != 0 {
				return true
			}
		}
	}
	return false
}

// scriptCompletes reports whether play a is on p's plain walk with a script's
// restricted mana floating: gorge's planner reads the pool through the
// restriction (pay.PlanPool), so what the restriction admits counts.
func (s *State) scriptCompletes(k *prep, p uint8, a Action) bool {
	var d Decision
	s.plainOptions(k, &d, 0)
	return hasOption(&d, a)
}

// tapGap taps gap sl into p's pool: a Bloom Tender's colours, or a
// restricted source's mana (restr), added to held too.
func (s *State) tapGap(k *prep, p uint8, sl Slot, restr slotMask, held *[kop.NumManaKinds]uint8) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if k.gen.has(s.cardAt(sl), gfDeferred) {
		if a := k.deferAb(s.cardAt(sl)); a != nil && a.Cost.Flags&kop.CostPayLife != 0 {
			s.payLifeCost(k, p, &a.Cost, 0) // Boseiju's "Pay 2 life" (mana_deferred.go deferLifeSurvives)
		}
		s.floatDeferred(k, p, sl) // (the window's gap too: restr is empty there)
		return
	}
	if restr.has(sl) {
		c := s.cardAt(sl)
		ab := &k.t.Cards[c].Abilities[k.gen.restr(c)]
		col := bits.TrailingZeros8(uint8(ab.ManaColors))
		n := s.restrAbN(k, sl, ab) // an evaluated amount (Vhal's toughness), not kgen's placeholder 1
		if !s.floatRestrAb(k, p, sl, ab, col, n) {
			s.capFull(StallRestrSrc) // never a silent spill
		}
		held[col] += n
		return
	}
	s.eachProduce(k, p, sl, s.eachDefaultAns(k, p, sl)) // (a fee-paid each-colour gap: mana_eachfee.go)
}

// plainListedPays reports whether the plain walk's listing of a is one the
// planner can price: the walk's relaxed census counts a filter as a free unit
// (availFilt), so a play only the filter script pays is listed there too. A
// listing with no plan would leave the play's mana step no witness (the
// autopay planner's panic), so scriptPrefix applies the filter script instead.
// Only a plain activation's printed cost is probed; an X, hybrid, power-up,
// waterbend or tap-X cost keeps the walk's answer (its plan needs the asks the
// build poses first).
func (s *State) plainListedPays(k *prep, a Action, p uint8, d *Decision) bool {
	if !hasOption(d, a) {
		return false
	}
	if a.Kind() != AkActivate {
		return true
	}
	o := Obj(a.Source())
	if o == 0 || int(o) >= len(s.objs) {
		return true
	}
	ob := &s.objs[o]
	c := ob.card
	ai := int(a.Mode())
	if int(c) >= len(k.t.Cards) || ai >= len(k.t.Cards[c].Abilities) {
		return true
	}
	ab := &k.t.Cards[c].Abilities[ai]
	if !plainPricedCost(ab) {
		return true
	}
	sl := NoSlot
	if ob.zone == kop.ZoneBattlefield {
		sl = ob.slot
	}
	cost := s.abPlannedCost(k, ab, sl, p)
	var pk planner
	pk.dry, pk.noGrant = true, true
	// the play's own payment descriptor: the walk probes the play before its
	// build begins, so payDescNow is not yet the play's, and a restricted
	// alternative the play admits (a creature-ability source beside a
	// creature's ability) would drop from the census and send the play to the
	// script prefix, whose taps gorge's plain plan never makes (gorge's
	// potential planner prices the play with its own descriptor)
	pk.rd, pk.rdSet = s.grantPayDescOf(k, a), !DebugPlannerFlexCensus
	s.tapPlanNeed(k, p, &cost, 0, NoObj, false, false, &pk)
	return !(pk.needT > 0 && !pk.found)
}

// plainPricedCost reports whether ab's printed cost is one the plain planner
// prices without the asks a build poses first: no X, hybrid, snow, Phyrexian,
// own-reduce, power-up, waterbend, tap-X or sacrifice part (plainListedPays,
// actScriptPays).
func plainPricedCost(ab *kop.Ability) bool {
	return !(ab.Cost.Mana.X != 0 || ab.Cost.XMin != 0 || ab.Cost.Mana.Hyb[0] != 0 ||
		ab.Cost.Mana.Snow != 0 || ab.Cost.Mana.Phyrexian() || ab.OwnReduce != (kop.ValueRef{}) ||
		ab.Cost.Flags&(kop.CostPowerUp|kop.CostWaterbend|kop.CostTapX|kop.CostSac|kop.CostSacSelf) != 0)
}

// scriptPrefix plays gorge's script for play a: the search's script, played
// (ctrScript) -- p's gaps tapped in battlefield (entry) order, a restricted
// source among them (Giada), an any-colour counter ability branched on its
// colour ask, until the plain walk lists a (nothing when it already does,
// and nothing new when no branch lists it: the chain's taps as the linear
// walk played them). The restricted mana floats beside the pool (State.rb),
// priced by the search's walk as gorge's prefix search does (restrOptions).
func (s *State) scriptPrefix(k *prep, a Action, p uint8) {
	// An activation on a board with a live granted mana ability prices without
	// the grant (otherGaps): its filter prefix would apply a funding-first
	// path gorge's search only reaches after the grant's branches. The grant
	// search (grantPrefix) owns the prefix on such a board; a cast keeps the
	// filter route (otherGaps counts a cast's grants, noGrant off). The
	// filter-only arm below yields to land's liveGrantGap for every play
	// (CR 601.2g-h); this guard keeps the mix board's filter route off such
	// an activation too.
	if k.gen.anyDiscHand && s.dhBoard(k, p) {
		var d Decision
		s.plainOptions(k, &d, availNoActScript)
		if !hasOption(&d, a) {
			s.dhScript(k, a, p, true) // Lion's Eye Diamonds the only gaps (mana_discardhand.go)
		}
		return
	}
	if k.gen.anyComboAny && s.caBoard(k, p) {
		s.caPrefix(k, a, p) // a Combo Any source among the gaps (mana_comboany.go)
		return
	}
	act := a.Kind() == AkActivate && s.liveGrantGap(k, p)
	if s.mixBoard(k, p) {
		if !act {
			var d Decision
			s.plainOptions(k, &d, availNoActScript)
			if !s.plainListedPays(k, a, p, &d) {
				// The filter prefix owns the play when it prices one. A no-{T}
				// tap-another source beside it leaves the filter search
				// unpayable (filtFundable), so the tapX search's prefix is the
				// fallback (mana_tapanother.go). Trying the filter first keeps
				// its trigger order when it is the play's payer (a filter
				// answer may tap a creature and stack its trigger before the
				// play is priced, CR 603.3).
				if !s.filtScript(k, a, p, true) && s.tapXTwoLiveSource(k, p) && s.tapXPrefix(k, a, p) {
					return
				}
			}
		}
		return
	}
	// The tapX route owns the prefix only when a real tap-another source is
	// on the board, or no Taps-deferred gap is: beside a depletion land the
	// DFS search's replay (scriptPrefixAny) taps both gaps in battlefield
	// order, and the tapX search's gaps (rs.gaps) hold no Taps-deferred one.
	if (k.tapMana || k.ctrMana) && (s.scriptTapXModelled(k, p).any() || !s.scriptTapsDefer(k, p).any()) && s.tapXPrefix(k, a, p) {
		return // a tap-another-permanent board (mana_tapanother.go)
	}
	gaps, restr, anyR := s.scriptGaps(k, p), s.scriptRestr(k, p), s.scriptAnyR(k, p)
	tapsDef := s.scriptTapsDefer(k, p)
	ctr := s.scriptCtr(k, p)
	anyDiff := s.anyDifferentGaps(k, p)
	life := s.lifeGapMask(k, p)
	// A Taps-deferred source (Rikala's Island) beside the filters and no other
	// gap kind is the filter search's own otherGap step (mana_filtertapsdefer.go):
	// gorge's one prefix search taps it and the filter in battlefield order
	// (Energy Refractor's {2} paid by the Island's extra {U}), which the
	// Taps-deferred search below never funds (CR 605.3a, 601.2g-h).
	tdFilt := tapsDef.any() && !gaps.or(restr).or(anyR).or(ctr).or(anyDiff).or(life).any() &&
		k.gen.anyFilt && s.filtOnly(k, p) && !s.liveGrantGap(k, p)
	if tdFilt || !gaps.or(restr).or(anyR).or(tapsDef).or(ctr).or(anyDiff).or(life).any() {
		// A live grant makes the grant search's candidates the board's
		// filters too, activated with the grants in battlefield order
		// (grantSrch.candidates): one search taps both gaps, so a filter
		// whose fee a granted {C} pays is not pre-empted here by a
		// filter-only script that would fund the fee from a land instead.
		if k.gen.anyFilt && s.filtOnly(k, p) && !s.liveGrantGap(k, p) {
			var d Decision
			s.plainOptions(k, &d, availNoActScript)
			// A live granted mana gap (a Humble Merchant) beside the filter:
			// gorge's one prefix search covers the board's filters with its
			// grants (mana_grantedoffers.go) and taps the granted gap first
			// (gaps before funding, scriptSearch.visit), its mana paying the
			// filter's fee — the filter script's funding would tap a printed
			// land the search leaves untapped, so grantPrefix applies it
			// instead (CR 601.2g-h)
			// A Phyrexian plain cast whose pip-free half the planner prices
			// runs no prefix: gorge announces the pips first (CR 601.2b) and
			// its CR 601.2g window then activates the filter with the plan's
			// mana (Moltensteel Dragon beside a Selesnya Signet), so a script
			// tapped ahead of the pip ask parts at the first tap flag.
			if !s.plainListedPays(k, a, p, &d) && !s.grantGapLive(k, p) && !(s.autopayDropsPlainCast(a) && s.phyHalfPlanned(k, p, a)) {
				s.filtScript(k, a, p, true)
			}
		}
		return
	}
	if k.gen.anyRefl && s.reflBoard(k, p, restr, anyR) {
		var d Decision
		s.plainOptions(k, &d, availNoActScript)
		if !hasOption(&d, a) {
			s.reflScript(k, a, p, true) // (mana_reflected.go)
		}
		return
	}
	if anyR.any() || tapsDef.any() || anyDiff.any() || life.any() {
		s.scriptPrefixAny(k, a, p, restr, anyR, tapsDef, ctr, anyDiff)
		return
	}
	if !restr.any() && !ctr.any() && s.eachFeeBoard(k, p) {
		s.eachScript(k, a, p, true) // a fee-paid each-colour gap (Tarnation Vista): gorge's search, mana_eachfee.go
		return
	}
	var d Decision
	s.plainOptions(k, &d, availNoActScript)
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, gaps.or(restr).or(ctr).or(anyDiff), order[:], 0) // gorge's option order (two Tenders)
	if hasOption(&d, a) {
		return
	}
	var path [MaxPerm]uint8
	m, found, _ := s.ctrScript(k, a, p, order[:n], ctr, restr, path[:], false)
	if found { // the script gorge's search found: its taps, played
		var held [kop.NumManaKinds]uint8
		for j := 0; j < m; j++ {
			s.ctrChainTap(k, p, a, order[j], ctr, restr, int(path[j]), &held)
		}
		return
	}
	// no script within gorge's search: the chain's taps as the linear walk
	// played them, each gap's first answer (the play stays on the offer unfunded)
	var held [kop.NumManaKinds]uint8
	for j := 0; j < n; j++ {
		s.ctrChainTap(k, p, a, order[j], ctr, restr, 0, &held)
	}
}

// FeatScripted reports whether p's priority option a is a scripted play:
// one only a gap's prefix pays, which gorge's net list (the planned
// payment actions and the decision's own options) does not hold.
func (s *State) FeatScripted(p uint8, a Action) bool {
	k := s.k()
	if !s.scriptAny(k) || !isScriptPlayKind(a.Kind()) || !s.scriptGaps(k, p).or(s.scriptRestr(k, p)).or(s.scriptAnyR(k, p)).or(s.scriptTapsDefer(k, p)).or(s.scriptCtr(k, p)).or(s.scriptTapXModelled(k, p)).or(s.lifeGapMask(k, p)).any() && !(k.gen.anyFilt && s.filtOnly(k, p)) && !(k.gen.anyDiscHand && s.dhBoard(k, p)) && !(k.gen.anyComboAny && s.caBoard(k, p)) {
		return false
	}
	var d Decision
	s.plainOptions(k, &d, 0)
	return !hasOption(&d, a)
}

// filterFunds reports whether the payment d of the play costing cost can be
// met by the censuses grantCap's walk leaves out -- a controlled or
// first-and-only filter's mana, a tap-another source, a restricted source --
// once every granted ability the payment refuses is dropped. It is the read
// that decides whether a play absent from the walk may stay on the offer.
func (s *State) filterFunds(k *prep, p uint8, cost *kop.ManaCost, ex Slot, d payDesc) bool {
	var av avail
	s.walkHandAvail(k, p, d, &av)
	if k.tapMana {
		s.availTapX(k, p, &av)
	}
	return s.faceReachable(k, p, &av, cost, 0)
}

// grantCap applies gorge's prefix-search cap to the plays only a granted
// mana ability pays (Imprisoned in the Moon's land): gorge searches a
// prefix for the first potentialScriptPlays plays of its potential walk
// that its printed sources cannot pay (rules PotentialPaymentPlans'
// scripts counter), so a later one is never offered even when the granted
// source would pay it (Squad Rallier's {2}{W} after potentialScriptPlays such casts, pair
// 5618). gorge's potential walk lists the plays p's potential mana pays,
// granted sources included, before any offer surface (availWalk).
// grantGapLive: p still has a granted mana ability (Honest Work's Humble
// Merchant) whose source can tap — the grant search's gap (grantCap), and
// the arm that owns the play's prefix then (mana_grantedoffers.go).
func (s *State) grantGapLive(k *prep, p uint8) bool {
	if !s.grantsLive(k) {
		return false
	}
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		if s.grantGapUsable(k, sl) {
			return true
		}
	}
	return false
}

// capGrantOvercounts is a filter-search gap node's cut of the root offer's
// grant cap (legal.go grantCap call): a granted mana ability counts beside its
// source's printed or restricted one in the walk (one tap makes one of them,
// CR 605.1a), so the restricted options the node's census overcounts are
// pruned with the same grant search the root's offer runs.
func (s *State) capGrantOvercounts(k *prep, d *Decision, p uint8) {
	if k.gen == nil || !k.gen.anyContGrant {
		return
	}
	var capSpent Decision // (the node keeps no capSpent: its options reprice at the deeper nodes)
	s.grantCap(k, d, p, &capSpent)
}

// untappedTapGapLive reports whether one of p's untapped, unsick sources fires
// a Taps watcher gorge's planner defers when it is tapped for mana -- another
// object's (tapsDeferred) or its own, which only a granted mana ability can
// reach (Mechan Navigator's "whenever this creature becomes tapped, draw a
// card, then discard a card" beside Mm'menon's granted mana, selfTapsDefer).
// The planner's own verdict never pays through such a source
// (paymentPlanSourceInterference: insufficient); only its prefix search does,
// and a draw there moves the library top a may-play cast is cast from (CR 400.7).
func (s *State) untappedTapGapLive(k *prep, p uint8) bool {
	if k.gen == nil || !(k.gen.anyTapsDefer || k.gen.anySelfTaps) {
		return false
	}
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].flags&pfTapped != 0 || s.activationSick(k, sl) {
			continue
		}
		if k.gen.anyTapsDefer && s.tapsDeferred(k, p, sl) || k.gen.anySelfTaps && s.selfTapsDefer(k, sl) {
			return true
		}
	}
	return false
}

// mayPlayJoinCensusPays reports whether a non-free may-play cast's composed
// cost is payable from the join census grantCap's walk reads -- the census
// that mirrors gorge's planner census, granted mana abilities included.
// gorge's may-play arm prices the composed cost from that census
// (potentialModeCastPlan -> planComposedCost over pay.PlanPaymentCostWithout),
// not from the plain cast's ungranted one, so the ungranted prune and its
// prefix search do not price it.
func (s *State) mayPlayJoinCensusPays(k *prep, p uint8, a Action, cost *kop.ManaCost) bool {
	if a.Mode()&castMayPlay == 0 || a.Mode()&castFree != 0 {
		return false
	}
	if s.untappedTapGapLive(k, p) {
		return false // gorge's prefix search prices it, not the census (untappedTapGapLive)
	}
	var mav avail
	s.availFor(k, p, &mav)
	if !mav.affords(cost, pipMaskOf(cost), 0) {
		return false
	}
	return !s.ownSacTapNeeded(k, p, cost)
}

// ownSacTapNeeded reports whether the join census pays cost only by counting
// the granted {T} of an untapped printed sacrifice-only source (an Eldrazi
// Spawn under Elven Chorus) beside its printed sacrifice: two units of one
// permanent (CR 602.2b, 605.1a). gorge's composed planner counts the source's
// sacrifice alone and never its granted tap, so the cast is priced by its
// prefix search, which a Spawn's sacrifice-first walk starves (the 160-node
// budget): the offer falls to grantSearch's mirror of that budget, not the
// census's.
func (s *State) ownSacTapNeeded(k *prep, p uint8, cost *kop.ManaCost) bool {
	if k.gen == nil || !k.sacOnlyMana || !k.gen.anyContGrant {
		return false
	}
	var gap slotMask
	var base avail
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		if pc := s.manaPCGen(k, sl); pc.manaSacOnly && s.ownSacGrantTap(k, sl) != nil {
			gap.set(sl)
		}
	}
	if !gap.any() {
		return false
	}
	s.availForExcluding(k, p, &base, gap)
	for m := gap; m.any(); m = m.rest() {
		sl := m.first()
		pc := s.manaPCGen(k, sl)
		base.addUnits(s.srcMaskAt(pc, sl), pc.manaN) // its printed sacrifice stays one unit
	}
	return !base.affords(cost, pipMaskOf(cost), 0)
}

func (s *State) grantCap(k *prep, d *Decision, p uint8, capSpent *Decision) {
	if !s.grantGapLive(k, p) {
		return
	}
	var walk Decision
	m := availWalk
	if s.selfGrantLive(k, p) {
		m |= availHyp // no relaxed proof: gorge's census is not relaxable (mana_owncolours.go)
	}
	if s.storeOpenLive(k, p) {
		m |= availAny // a storage battery folds gorge's potential walk open (mana_storeopen.go)
	}
	s.plainOptions(k, &walk, m)
	// in gorge's potential walk order (walkKey: the hand, the permission
	// casts, the graveyard in graveyard order with its flashbacks, the
	// activations), not the kernel walk's (pair 45914)
	var key [MaxOptions]uint64
	for i := 0; i < int(walk.N); i++ {
		kk := s.walkKey(walk.Options[i], p, i)
		a := walk.Options[i]
		j := i
		for ; j > 0 && key[j-1] > kk; j-- {
			walk.Options[j], key[j] = walk.Options[j-1], key[j-1]
		}
		walk.Options[j], key[j] = a, kk
	}
	// gorge's offer set on an unrelaxable census (a live sacrifice-any or
	// two-permanent tap-another source, a Combo Any gap) is the bound's and
	// gaps walk's plays (mana_tapanother.go tapXSacAnyBound, script_gen.go
	// potentialBoundLists), which the searches below mirror; a play only the
	// separate gap units pay is searched and offered beside a relaxable
	// census and a live tap-another source alone (gorge runs its prefix
	// search only on an incomplete census, potential_plan.go
	// potentialPlayVerdict; a granted ability beside none is priced by the
	// census, Joraga Treespeaker's own level ability).
	relaxCensus := s.grantRelaxCensus(k, p)
	n := 0
	var g grantSrch
	// A play the offer lists on the join of a source's own and granted
	// abilities (one unit of the larger amount, availGrant) that the walk's
	// relaxed census does not list pays only through a mana the source cannot
	// make in one tap (Xenobotanist's one mana of any colour beside a granted
	// {G}{G} cannot fund {U}{U}): gorge's potential walk never lists it.
	for i := int(d.N) - 1; i >= 0; i-- {
		a := d.Options[i]
		if ak := a.Kind(); ak != AkActivate && !isCastKind(ak) || hasOption(&walk, a) {
			continue
		}
		if ak := a.Kind(); isCastKind(ak) && a.Mode()&3 == 1 && s.kickedArmOffer(k, a) {
			// the kicked cast the potential arm's census paid: the walk's
			// plainOptions prices no kicked cast there (the arm runs only at
			// availX == 0, its census the real one, the walk's relaxed), so
			// the walk's lack of it is no gorge refusal -- gorge's planner
			// prices the mode cast itself (potentialModeCastPlan,
			// kw_multikicker.go)
			continue
		}
		if s.tapExtraLive(k) {
			continue // a tap's extra is no part of the walk's units here, yet gorge's relaxed census counts it (mana_tapextra.go)
		}
		cost, ex, need, ok := s.grantPlayCost(k, p, a, true)
		if !ok || s.affordsUngrantedFor(k, p, &cost, ex, s.grantPayDesc(k, a)) {
			continue
		}
		if (k.multiMana && s.filterSource(k, p) || k.gen.anyFiltFirst && s.scriptFilt(k, p).any()) &&
			s.filterFunds(k, p, &cost, ex, s.grantPayDesc(k, a)) {
			// a controlled filter (Rumble Arena's {1},{T}: any colour) or a
			// first-and-only filter (Prophetic Prism's {1},{T}) is no unit of
			// the census here, yet it can fund the play (CR 605.1a, 106.1):
			// keep it only when this play's own payment reaches it, so a
			// filter on the board cannot keep a hand-barred grant's overcount
			// (Starwinder's warp beside Mm'menon; CR 106.6, 702.185a)
			continue
		}
		d.remove(a)
		if !s.grantSearch(k, p, &cost, ex, s.grantPayDesc(k, a), need, &g) {
			// gorge's one prefix search gave the play up (its granted gaps
			// spent the node budget before the tap-another line): the
			// tap-another search alone, which skips the grants, must not
			// offer it again (scriptOffers)
			if g.stall != 0 {
				// the drop is final (capSpent) and one of the search's lines
				// ran into a rule gap the kernel stalls by name: declare it,
				// not a dropped offer. The search itself declares nothing
				// (mana_grantedoffers.go grantSearch, tapXScript's shape).
				s.capFull(g.stall)
			}
			capSpent.add(a)
		}
	}
	for i := 0; i < int(walk.N); i++ {
		a := walk.Options[i]
		if isEscapeCastAction(a) {
			continue // gorge's planner prices no escape mode ("unsupported shape", kw_escape.go): no prefix search, so no storage-battery step offers it
		}
		if s.objPartCast(k, a) {
			// gorge's potential planner refuses the shape before any prefix
			// search (potentialPlayVerdictShared, "shape:additional_cost"), so
			// only the printed sources price it (Fear of Abduction's exile
			// beside Enduring Vitality's creatures; CR 601.2g-h)
			if !s.objPartPrinted(k, p, a) {
				d.remove(a)
				capSpent.add(a)
			}
			continue
		}
		cost, ex, need, ok := s.grantPlayCost(k, p, a, true)
		if !ok {
			continue
		}
		if s.mayPlayJoinCensusPays(k, p, a, &cost) {
			// the may-play arm's cost is the join census's (the helper
			// above), not the plain cast's ungranted read below: keep the
			// offer without the prefix search (Elven Chorus's "{T}: Add one
			// mana of any color" creatures beside a top-of-library Grazing
			// Whiptail; CR 601.2g, 118.9)
			continue
		}
		if s.affordsUngrantedPlay(k, p, a, &cost, ex, s.grantPayDesc(k, a)) {
			continue // priced by the printed sources, restricted ones the play may spend included
		}
		if s.contribCast(k, p, a) && !s.improviseAloneCast(k, p, a) {
			// a convoke, improvise or delve cast (Dazzling Theater's granted
			// convoke): gorge's potential planner refuses the shape
			// ("shape:contribution") before any prefix search, so the printed
			// sources did not price it and it is not offered (CR 702.51a). An
			// improvise cast that credits no artifact is no contribution
			// (improviseAloneCast): gorge plans it plainly and its prefix
			// search taps the granted source (Organic Extinction beside
			// Brightcap Badger's Saprolings). A twobrid cast is no longer
			// dropped here: gorge's planner resolves its pips (planTwobridCost)
			// and the prefix search prices it like any other mana cost.
			// The refusal is the offer verdict only for a base the credit
			// leaves unpayable: gorge's offer gate prices the credited base
			// itself (castOfferBase, mana.go: convokeCost then improviseCost),
			// so a hand cast whose credited base the printed census pays --
			// the creatures paying the coloured pips with every land tapped
			// (CR 702.51a) -- is still offered, as the credited re-price below
			// reads (a delve cast credits nothing: castOfferBase is the raw base)
			//
			// An alternative-cost keyword cast (impending, evoke, dash,
			// overload) is the exception: gorge's potential planner prices it
			// at its raw keyword cost with no credit and, when the printed
			// sources fall short, answers "unsupported cost:credit" with no
			// prefix search (potentialModeCastPlan, potentialModeCredit). The
			// credited re-price would keep an Overlord of the Mistmoors that
			// only the granted-mana creatures, tapped as sources and as
			// convoke, could pay (CR 702.51a, 702.176a).
			if o := Obj(a.Source()); s.objs[o].zone == kop.ZoneHand && !altKeywordCast(a) {
				if credited := s.convokeCredit(k, p, s.objs[o].card, o, cost); s.affordsUngrantedPlay(k, p, a, &credited, ex, s.grantPayDesc(k, a)) {
					continue
				}
			}
			d.remove(a)
			capSpent.add(a)
			continue
		}
		if !s.grantRelaxedAffords(k, p, a, &cost, ex, s.grantPayDesc(k, a), m) {
			d.remove(a) // gorge's relaxed proof: insufficient, no prefix search (mana_grantrelaxed.go)
			capSpent.add(a)
			continue
		}
		if k.tapMana && hasOption(d, a) && s.tapXOffGrant(k, p) {
			if found, _ := s.tapXScript(k, a, p, false); found {
				continue // a tap-another source (Gene Pollinator) pays it in scriptOffers' search, which the grant search does not run (mana_tapanother.go)
			}
		}
		if m&availHyp == 0 && s.relaxRestrRefuses(k, p, &cost, s.grantPayDesc(k, a)) {
			d.remove(a) // gorge's relaxed proof refuses the restricted grant: insufficient, never searched (mana_grantrelaxrestr.go)
			capSpent.add(a)
			continue
		}
		if n++; n > potentialScriptPlays {
			d.remove(a) // never searched: unpriced
			continue
		}
		if !s.grantSearch(k, p, &cost, ex, s.grantPayDesc(k, a), need, &g) {
			found, stall := false, uint8(0)
			if s.tapXOffGrant(k, p) {
				found, stall = s.tapXScript(k, a, p, false)
			}
			if !found && !s.ctrGapsOutsideGrantSearch(k, p) {
				// Gorge's script search covers every census gap — granted units,
				// tap-another-permanent gaps and counter-cost mana (Wall of Roots'
				// counter beside Wrenn and Realmbreaker's grant) alike (CR 605.1a,
				// 601.2g-h) — so the kernel drops the play only when no route
				// prices it; a live counter-cost gap outside the grant search's storage
				// batteries leaves the play to scriptOffers' counter-gap search
				// (scriptDeadEnd).
				if g.stall != 0 {
					// the drop is final — capSpent keeps scriptOffers from
					// repricing the play — and one of the grant search's lines
					// ran into a rule gap the kernel stalls by name: declare
					// it, not a dropped offer (the search declares nothing;
					// the grant search ran before the tap-another one, so its
					// stall is the first named)
					s.capFull(g.stall)
				}
				// ... and one of the tap-another search's lines ran into a
				// rule gap the kernel stalls by name: declare it, not a
				// dropped offer (mana_tapanother.go tapXScript declares
				// nothing).
				s.capStall(stall)
				d.remove(a) // no prefix within gorge's node budget (mana_grantedoffers.go)
				// The play has had gorge's one prefix search (the grant search
				// covers a grant board's filters with its grants,
				// mana_grantedoffers.go): scriptOffers' filter search must not
				// price it again on a second budget.
				capSpent.add(a)
			}
		} else if s.grantSearchAdds(k, p, a, d, &g, relaxCensus) {
			d.add(a)
		}
	}
}

// grantSearchAdds reports a play a the grant search's prefix pays that the
// offer list d lacks and gorge's walk lists (an improvise cast crediting no
// artifact, whose plain planner never prices it, included: Reverse Engineer
// paid by a Swamp wearing Sheltered Aerie; improviseAloneCast):
// gorge's potential planner offers every play its one prefix search
// pays (potentialPlayVerdict): the offer builder's census prices a
// source's own and granted abilities as one join unit (availGrant),
// so a play only the separate gap units pay -- a land's granted
// any-colour ability beside its printed one with the granting
// source tapped out (Peema Outrider's {2}{G}{G} beside a tapped
// Chromatic Lantern's lands and a Jaspera Sentinel; CR 611.3c,
// 601.2g-h) -- is searched here and added. gorge's planner prices
// the play again at every search node (potentialPlayVerdict,
// mana_grantedoffers.go visit): a trigger the found prefix answered
// lowered the cost under the root price the offer builder priced
// the play with (Gran-Gran's loot discarding a Lesson into the
// graveyard, CR 601.2f, 106.4), so gorge's walk lists what the
// builder left off; so does a prefix that activates a storage
// battery, whose counters the builder's units leave out
// (mana_grantstore.go), except a plain cast under the autopay
// surface, which gorge reaches only by that script and drops: the repriced
// leaf's cast too (a cast whose root price no prefix-free payment meets is
// a scripted one, and the oracle's priorityVocabulary leaves it out)
func (s *State) grantSearchAdds(k *prep, p uint8, a Action, d *Decision, g *grantSrch, relaxCensus bool) bool {
	return !hasOption(d, a) && (g.cheaper && !s.autopayScriptedCast(a) || g.storeStep() && !s.autopayScriptedCast(a) ||
		g.np != 0 && s.improviseAloneCast(k, p, a) && !s.autopayScriptedCast(a) ||
		s.cfg&cfgGorgeAutopay == 0 && relaxCensus && s.tapXCensusGap(k, p).any())
}

// grantRelaxCensus is whether the walk's census is relaxable beside a live
// grant gap: gorge's offer set on an unrelaxable census (a live
// sacrifice-any or two-permanent tap-another source, a Combo Any gap) is
// the bound's and gaps walk's plays, which grantCap's searches mirror.
func (s *State) grantRelaxCensus(k *prep, p uint8) bool {
	_, sacOpen := s.tapXSacAnyBound(k, p)
	var gateFilt slotMask
	if k.gen.anyFilt {
		gateFilt = s.scriptFilt(k, p)
	}
	return s.scriptRelax(k, p, gateFilt,
		k.gen.anyComboAny && s.caBoard(k, p) || k.tapMana && s.tapXTwoLive(k, p) || sacOpen)
}

// objPartCast reports a plain cast a whose spell carries a cost part the
// planner never composes (an exile from the battlefield or the graveyard, a
// forage, a return, a blight, collected evidence): gorge's potential planner
// prices it from the printed sources only.
func (s *State) objPartCast(k *prep, a Action) bool {
	if a.Kind() != AkCast || a.Mode() != 0 || k.t == nil {
		return false
	}
	c := s.objs[a.Source()].card
	ac := spellAddCost(k, c)
	return ac.Flags&costObjFlags != 0 || ac.ExileBf || k.gen != nil && k.gen.anyEvTargets && spellEvidenceTargets(&k.t.Cards[c])
}

// contribCast reports a cast a gorge's planners decline as "shape:contribution"
// (kw_delve.go contribPays).
func (s *State) contribCast(k *prep, p uint8, a Action) bool {
	if !isCastKind(a.Kind()) {
		return false
	}
	o := Obj(a.Source())
	return s.contribPays(k, p, s.objs[o].card, o, s.objs[o].zone == kop.ZoneHand)
}

// scriptShapeSkip is scriptOffers' shape skip for the cast a: an exile,
// forage, return, blight or evidence part (shape:additional_cost;
// objPartCast) beside no either-or pay, or a contribution cast (shape:
// contribution; kw_delve.go contribPays) whose credit is empty (improvise
// alone, improviseAloneCast) still goes to gorge's prefix search.
func (s *State) scriptShapeSkip(k *prep, p uint8, a Action) bool {
	return s.objPartCast(k, a) && !k.t.Cards[s.objs[Obj(a.Source())].card].HasAltAdd ||
		s.contribCast(k, p, a) && !s.improviseAloneCast(k, p, a)
}

// improviseAloneCast reports whether a plain cast of p's card c carries
// improvise and no other shape:contribution keyword or priced extra, and
// credits no artifact: gorge's planner plans such a cast plainly (its
// improviseTaps are empty — improviseCost credits generic only, so a cast
// without generic or without an untapped artifact credits nothing), the
// credited census misses the pips and the prefix search funds a census gap's
// fee beside them: the cast is offered (CR 601.2g; Geralf's Messenger beside
// Inspiring Statuary, decision 389 of tail-08 pair 541). A cast whose credit
// is not empty goes through gorge's planImprovise instead, whose
// "unsupported cost:credit" runs no prefix search (payment_plan_improvise.go)
// — a mode, a may-play/face/warp form, a target, an additional cost, an
// announced X or a live artifact credit keeps the contribution shape.
func (s *State) improviseAloneCast(k *prep, p uint8, a Action) bool {
	if a.Kind() != AkCast || a.Mode()&3 != 0 || a.Mode()&(castMayPlay|castFace|castWarp) != 0 || a.Target() != 0 {
		return false
	}
	o := Obj(a.Source())
	ob := &s.objs[o]
	if ob.zone == kop.ZoneBattlefield {
		return false // a permanent on the battlefield is never a spell being cast (kw_delve.go)
	}
	c := ob.card
	fromHand := ob.zone == kop.ZoneHand
	if k.t.Cards[c].Cost.X != 0 || k.t.Cards[c].Cost.XRed != 0 || k.t.Cards[c].Cost.XPip != 0 {
		return false // the X ask keeps the manual window (payment_plan.go costAnnouncesX)
	}
	if sp := k.t.Cards[c].SpellCost; sp.Flags != 0 || sp.N != 0 || sp.Ctr != 0 {
		return false // an additional sacrifice or tap part keeps it (payment_plan.go spellCost)
	}
	if !s.isImprovise(k, p, c, o, fromHand) || s.hasConvoke(k, p, c, o, fromHand) ||
		s.hasCastDelve(k, p, c, o, fromHand) {
		return false
	}
	return k.t.Cards[c].Cost.Generic == 0 || !s.improviseCreditLive(k, p)
}

// improviseCreditLive reports an untapped artifact of p's the improvise
// credit could tap (gorge improviseCost's candidates, kw_convoke.go
// tapPayCand with imp).
func (s *State) improviseCreditLive(k *prep, p uint8) bool {
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(SeatIx(p), s.ctrlM[SeatIx(p)].and(s.occ), ord[:], 0)
	for _, sl := range ord[:n] {
		if s.tapPayCand(k, sl, true) {
			return true
		}
	}
	return false
}

// objPartPrinted reports whether the printed sources pay objPartCast a's
// mana, as the offer gate prices it (legal.go).
func (s *State) objPartPrinted(k *prep, p uint8, a Action) bool {
	o := Obj(a.Source())
	c := s.objs[o].card
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	return s.affordsUngrantedFor(k, p, &cost, NoSlot, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)})
}

// grantActScript reports whether gorge's one prefix search finds a script for
// the activation a of p on a board with a live granted mana ability and a
// filter: the search tries the grants' colours before the funding tap that
// pays the filter's fee (scriptSearch.visit: the gaps first), spending its
// node budget on mana the activation cannot spend (a creature-spell-only
// grant), so the filter script's funding-first line must not price it
// (filtScript sets noGrant for an activation). The grant search owns the
// prefix on such a board (scriptPrefix) and prices the offer too (CR 601.2g-h).
// The second return is the failed search's latched stall (grantSearch's
// g.stall, StallNone none): the caller declares it only where it finally drops
// the play.
func (s *State) grantActScript(k *prep, a Action, p uint8) (bool, uint8) {
	cost, ex, need, ok := s.grantPlayCost(k, p, a, true)
	if !ok {
		// the filter script declares its own failed line's stall (filtScript)
		return s.filtScript(k, a, p, false), 0
	}
	var g grantSrch
	if s.grantSearch(k, p, &cost, ex, s.grantPayDesc(k, a), need, &g) {
		return true, 0
	}
	return false, g.stall
}

// grantPrefix: gorge's planner census holds printed mana abilities only
// (rules/pay PaymentAbility), so a source whose mana ability an effect
// grants (Imprisoned in the Moon's "{T}: Add {C}") is a census gap
// (potential_plan.go paymentPlanCensusOf). A play the planner cannot price
// without it is a scripted potential play (potentialPrefixScript): the gaps
// are activated first, in battlefield order, the play re-priced after each
// (scriptSearch.visit: gaps first, then priced), and the oracle plays that
// prefix before the play itself (prScript). The kernel taps them the same
// way at the play, so the mana floats from the announcement on.
func (s *State) grantPrefix(k *prep, a Action, p uint8) {
	cost, ex, need, ok := s.grantPlayCost(k, p, a, true)
	if !ok {
		return
	}
	if s.tapXGrantBoard(k, p, a) && !s.affordsUngrantedPlay(k, p, a, &cost, ex, s.grantPayDesc(k, a)) {
		if found, _ := s.tapXScript(k, a, p, true); found {
			return // gorge's one prefix search takes the sacrifice-any source and the grants in battlefield order (mana_tapxgrant.go)
		}
	}
	var g grantSrch
	if s.grantSearch(k, p, &cost, ex, s.grantPayDesc(k, a), need, &g) && g.np != 0 {
		s.grantApply(k, &g) // the found prefix (mana_grantedoffers.go)
		return
	}
	// The granted mana cannot pay the play (a spell-only grant and an
	// ability, CR 605.1a) and the printed sources do not price it either:
	// gorge's potential walk prices printed abilities only, so the play is
	// off the walk and its script search taps the census gaps — the granted
	// sources in battlefield order, their mana floating beside the pool, and
	// the tap-another gap's step among them (grantTapXPrefix; pair 676).
	if !s.affordsUngrantedPlay(k, p, a, &cost, ex, s.grantPayDesc(k, a)) {
		s.grantTapXPrefix(k, a, p)
	}
}

// altKeywordCast reports whether a is a hand cast for one of the
// alternative-cost keyword rows gorge prices at the raw keyword cost
// (altcast_modes.go: evoke, dash, overload, impending with potential set).
func altKeywordCast(a Action) bool {
	if a.Kind() != AkCast || a.Mode() != 0 {
		return false
	}
	switch a.Target() {
	case castImpTgt, castEvokeTgt, castDashTgt, castOverloadTgt:
		return true
	}
	return false
}
