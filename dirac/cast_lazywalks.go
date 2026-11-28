package dirac

// scriptOffersEager is scriptOffers as it ran before its nested walks were
// made conditional on a reader (script_gen.go scriptOffersLazy): every walk
// built where the candidate loop might read it. Only the shadow hook runs it
// (DebugShadowNestedWalks, DebugEagerNestedWalks); keep it byte for byte the
// old body, so the hook compares the lazy path with the code it replaced.
func (s *State) scriptOffersEager(k *prep, d *Decision, capSpent *Decision, p uint8) (searched int) {
	restr, anyR := s.scriptRestr(k, p), s.scriptAnyR(k, p)
	tapsDef := s.scriptTapsDefer(k, p)
	mix := s.mixBoard(k, p)
	filt := s.scriptOfferFilt(k, p, mix)
	tx := s.scriptTapXModelled(k, p).or(s.scriptCtrModelled(k, p))
	dh := k.gen.anyDiscHand && s.dhBoard(k, p) // Lion's Eye Diamonds the only gaps (mana_discardhand.go)
	ca := k.gen.anyComboAny && s.caBoard(k, p) // a Combo Any source among the gaps (mana_comboany.go)
	caM := s.caGapMask(k, p, ca)
	// a Combo Any source beside a filter (the filter search's otherComboAny
	// step): gorge's census is as unrelaxable as with the source alone
	caAny := s.caOffersBoard(k, p, ca)
	anyDiff := s.anyDifferentGaps(k, p) // a Firemind Vessel among the gaps (mana_distinctcolours.go)
	life := s.lifeGapMask(k, p)         // a life-only mana ability among the gaps (mana_lifegap.go)
	// raBoard: a live reflected Amount$ source is the board's only modelled
	// census gap (no filter, no other gap kind beside it). gorge's census is
	// unrelaxable there (mana_reflamt.go reflAmtBoundPlay), so the candidates
	// are the any-mana walk and the bound arm admits them (reflAmtBoundPlay).
	raBoard := !ca && !dh && !filt.any() && !mix && !restr.any() && !anyR.any() &&
		!tapsDef.any() && !s.scriptCtr(k, p).any() && !tx.any() && !anyDiff.any() && !life.any() &&
		s.reflAmtLive(k, p)
	if !dh && !ca && !s.caBesideGrant(k, p) && !s.scriptGaps(k, p).or(restr).or(anyR).or(tapsDef).or(filt).or(s.scriptCtr(k, p)).or(tx).or(anyDiff).or(life).any() {
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
	relax := s.scriptRelax(k, p, filt, caAny && !s.caRelaxable(k, p, ca) || twoLive || sacAny || raBoard)
	// A live repeatable fee-only filter (Nomadic Elf) has no relaxed or bounded
	// proof in gorge either: it searches every insufficient play, and the gaps
	// walk (one conversion per filter) cannot say which a many-times-activated
	// one pays (mana_filterrepeat.go).
	repeatLive := s.filtRepeatIn(k, filt)
	var any, gaps Decision
	s.plainGapsOptions(k, &gaps, filt)
	s.mergeBareGaps(k, &gaps, filt)
	src := &gaps
	if s.scriptAnyWalk(k, p, filt, relax, caAny || twoLive) {
		s.plainOptions(k, &any, availAny)
		src = &any
		if sacAny {
			s.unionGapsWalk(&any, &gaps)
		}
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
	var noWide, noWideNF Decision
	if mixWide {
		s.plainOptions(k, &noWide, availGaps|availNoWide) // (gorge's potential walk counts a wide source only for what its restriction admits)
		// The held-out walk is the fee-charged gaps walk, but the relaxed
		// proof accepts any subset of the paid gaps, the empty filter subset
		// included (availNoFilt): a play that subset pays with the wide
		// source's mana held out is not proven insufficient (tail-42 pair
		// 49: Subterranean Hangar's storage counters paid Liliana's Contract
		// beside the School of the Unseen's {2},{T} fee and Gwenna's
		// creature-only mana, the cast unoffered).
		s.plainOptions(k, &noWideNF, availGaps|availNoWide|availNoFilt)
	}
	skip := offerSkip{
		d: d, capSpent: capSpent, gaps: &gaps,
		sacAny: sacAny, sacRepeat: sacRepeat,
		wide: wide, anyR: anyR, mixWide: mixWide, noWide: &noWide, noWideNF: &noWideNF,
		anyWalk: src == &any, reflFilt: s.reflFiltBoard(k, p, ca, filt),
	}
	for i := 0; i < int(src.N); i++ {
		a := src.Options[i]
		if s.scriptOfferDrops(k, p, a, &skip) {
			continue
		}
		if s.pricedByPrintedSources(k, &real, a) {
			continue // priced by the printed sources: gorge's planner finds it sufficient and runs no prefix search for it
		}
		if plainCastMode(a) && s.plainCastUnsearched(k, p, a) {
			continue // gorge runs no prefix search for it: the printed sources pay it but the witness eats a target (witness_unreachable, mana_sacsource.go), or the shape is additional_cost and the composition is its only witness, an insufficient one dropped (potentialPlayVerdictShared)
		}
		if cd := &k.t.Cards[s.objs[Obj(a.Source())].card]; isCastKind(a.Kind()) && (s.spellTgtDep(k, p, s.objs[Obj(a.Source())].card, Obj(a.Source())) || cd.HasModeCost && !modalPricedAtPrinted(a) && !s.modalFloorFree(k, p, s.objs[Obj(a.Source())].card)) {
			continue // gorge searches no prefix for a target-priced cost, nor for a modal one whose cheapest mode still costs mana (a may-play or flashback cast is priced at its printed cost: potentialPlayVerdictShared's mayplay arm, and potentialModeCastPlan for a "flashback" mode)
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
	boundLive := filtDeferBoundLive(sacAny, filt, tapsDef)
	rt := scriptRoute{
		dh: dh, ca: ca, caM: caM, filt: filt, mix: mix,
		tapsDef: tapsDef, restr: restr, anyR: anyR, anyDiff: anyDiff,
		tx: tx, life: life, gaps: &gaps,
		twoLive: twoLive, repeatLive: repeatLive,
	}
	for i := 0; i < nc && searched < potentialScriptPlays; i++ { // the first potentialScriptPlays: searched
		a := cand[i]
		if !s.gapsOrBoundPlay(k, p, boundLive, &gaps, a) && !twoLive && !repeatLive && !((caAny || raBoard) && s.reflAmtBoundPlay(k, p, a)) && !s.ownTapWalkPlay(k, p, a, &ownWalk, &ownBuilt) {
			if !relax {
				searched++
			}
			continue
		}
		searched++
		if s.autopayDropsPlainCast(a) && !s.lifePlanned(k, p, life, &lifeWalk, &lifeBuilt, a) && !s.phyHalfPlanned(k, p, a) {
			continue // a plain cast: dropped by the autopay surface unless gorge's planner prices it with a life-only source (mana_lifeplan.go)
		}
		if s.scriptRouteDrop(k, &rt, a, p) {
			continue
		}
		d.add(a)
	}
	return searched
}

// DebugShadowNestedWalks (TestNestedWalksShadow): every scriptOffers call also
// runs scriptOffersEager on a fork of the same State and panics unless the
// two leave the same options in d in the same order, the same spent cap, the
// same searched count and the same game State (shadowSameState).
var DebugShadowNestedWalks bool

// DebugEagerNestedWalks (TestNestedWalksShadow): scriptOffers runs the eager
// twin in place of the lazy path, so a whole game can be played both ways and
// its decision stream compared.
var DebugEagerNestedWalks bool

// shadowMismatch is the last shadow difference, for the test's report: the
// lazy State and the eager fork's, copied when the hook is about to panic.
var shadowMismatch struct {
	lazy, eager State
}

// shadowCalls counts the scriptOffers calls the shadow compared, and
// shadowPlays those whose candidate walk held a play the lazy path read.
var shadowCalls, shadowPlays int

func (s *State) scriptOffersHook(k *prep, d *Decision, capSpent *Decision, p uint8) int {
	if !DebugShadowNestedWalks {
		return s.scriptOffersEager(k, d, capSpent, p)
	}
	f := s.fork()
	dE := *d
	var cE Decision
	cp := (*Decision)(nil)
	if capSpent != nil {
		cE, cp = *capSpent, &cE
	}
	shadowCalls++
	nE := f.scriptOffersEager(k, &dE, cp, p)
	n := s.scriptOffersLazy(k, d, capSpent, p) // its forks sit above f in the pool
	why := ""
	switch {
	case n != nE:
		why = "searched"
	case !sameOptionList(d, &dE):
		why = "options"
	case capSpent != nil && !sameOptionList(capSpent, &cE):
		why = "capSpent"
	case !shadowSameState(s, f):
		why = "state"
	}
	if nE > 0 {
		shadowPlays++
	}
	if why != "" {
		shadowMismatch.lazy.CopyFrom(s)
		shadowMismatch.eager.CopyFrom(f)
	}
	s.unfork()
	if why != "" {
		panic("dirac: scriptOffers lazy walks differ from the eager path: " + why)
	}
	return n
}

func sameOptionList(a, b *Decision) bool {
	if a.N != b.N {
		return false
	}
	for i := 0; i < int(a.N); i++ {
		if a.Options[i] != b.Options[i] {
			return false
		}
	}
	return true
}

// shadowSameState compares the lazy State with the eager fork's, the
// filtBoundPays memo aside: a walk the lazy path skipped may have left a
// different entry there, and the memo answers only for its full key
// (mana_filter.go fbpMemo).
func shadowSameState(s, f *State) bool {
	m := s.g.fbp
	s.g.fbp = f.g.fbp
	same := s.SameGame(f)
	s.g.fbp = m
	return same
}
