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
// play the gaps are tapped into the pool in battlefield order until the
// plain walk lists it.

const potentialScriptPlays = 16 // gorge rules/potential_script.go

// availFor modes (genState.availX): the gaps' mana added, any mana, or the
// plain walk without the script hook.
const (
	availGaps  uint8 = 1
	availAny   uint8 = 2
	availPlain uint8 = 4
	availWalk  uint8 = 8   // the real mana, no offer surface (grantCap)
	availHyp   uint8 = 128 // with availWalk: gorge's PotentialMana bound, a choice source counting every colour it can make (mana_owncolours.go)
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
	return k.gen != nil && (k.gen.anyScriptGap || k.sacOnlyMana || k.ctrMana || k.gen.anyTapsDefer || k.gen.anyDiscHand)
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
		if pm.flags&pfTapped != 0 || s.activationSick(k, sl) || s.genLost(k, sl) ||
			k.gen.anyCantAct && s.actForbidden(k, sl, true) { // a CantBeActivated lock (cast_cantactivate.go)
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
	for b := s.scriptGaps(k, p); b.any(); b = b.rest() {
		s.eachGapUnits(k, p, b.first(), a)
	}
	// a restricted source (Giada): the prefix search prices with its mana
	// as with any gap's (the restriction is not read there), and an Angel
	// spell may then spend it (scriptDeadEnd decides)
	for b := s.scriptRestr(k, p); b.any(); b = b.rest() {
		c := s.cardAt(b.first())
		if k.gen.has(c, gfDeferred) {
			a.addUnits(1<<kop.R, 1) // Goggles' plain {R} (mana_deferred.go)
			continue
		}
		ab := &k.t.Cards[c].Abilities[k.gen.restr(c)]
		a.addUnits(1<<bits.TrailingZeros8(uint8(ab.ManaColors)), max(ab.ManaN, 1))
	}
	// a Taps-deferred source (Firdoch Core): its relaxed census unit, the
	// one-mana any/fixed-colour tap it adds (availFor's, which the plain walk
	// leaves out: mana_tapsdefer.go)
	for b := s.scriptTapsDefer(k, p); b.any(); b = b.rest() {
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
	if k.gen.anyFilt && (s.filtOnly(k, p) || s.mixBoard(k, p)) {
		s.availFilt(k, p, a) // (beside another gap kind the filter stays unmodelled: mana_filter.go)
	}
}

// plainOptions is p's priority walk without the script hook, under
// availFor mode m.
func (s *State) plainOptions(k *prep, d *Decision, m uint8) {
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

// scriptOffers adds to d the plays a scripted prefix pays.
func (s *State) scriptOffers(k *prep, d *Decision, capSpent *Decision, p uint8) {
	restr, anyR := s.scriptRestr(k, p), s.scriptAnyR(k, p)
	tapsDef := s.scriptTapsDefer(k, p)
	var filt slotMask
	mix := s.mixBoard(k, p)
	if k.gen.anyFilt {
		if f := s.scriptFilt(k, p); f.any() && (mix || !s.gapOthers(k, p).any()) { // (filtOnly)
			filt = f
		}
	}
	tx := s.scriptTapXModelled(k, p).or(s.scriptCtrModelled(k, p))
	dh := k.gen.anyDiscHand && s.dhBoard(k, p) // Lion's Eye Diamonds the only gaps (mana_discardhand.go)
	if !dh && !s.scriptGaps(k, p).or(restr).or(anyR).or(tapsDef).or(filt).or(s.scriptCtr(k, p)).or(tx).any() {
		return
	}
	// With only restricted gaps (Giada, a Courtyard) gorge's census is
	// relaxable (each has a fixed relaxed alternative; a Tender's has
	// none): a play the relaxed proof cannot pay, the gaps walk's, is
	// insufficient and no search, so it takes none of the searches
	// (potential_plan.go paymentPlanRelaxProof), and the candidates are
	// the gaps walk's alone (one walk, not two).
	twoLive := k.tapMana && s.tapXTwoLive(k, p) // (a live Baylen makes the census unrelaxable: gorge paymentPlanRelaxedAlternatives, no once-only cost; every walk play is searched)
	// A fourth filter, or a dynamic one out of bounds, makes it unrelaxable too (mana_filter.go filtRelaxable).
	relax := !s.scriptGaps(k, p).any() && !twoLive && s.filtRelaxable(k, filt)
	var any, gaps Decision
	s.plainOptions(k, &gaps, availGaps)
	src := &gaps
	if !relax {
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
	wide := !mix && !tapsDef.any() && s.wideGapsOnly(k, p, restr, anyR) // (gorge's relaxed proof reads a wide source against each play; a Taps-deferred gap is none of them)
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
	for i := 0; i < int(src.N); i++ {
		a := src.Options[i]
		if !isScriptPlayKind(a.Kind()) || hasOption(d, a) || hasOption(capSpent, a) ||
			wide && !s.wideAdmits(k, a, p, anyR) ||
			mixWide && !hasOption(&noWide, a) && !s.wideAdmits(k, a, p, s.scriptWide(k, p)) {
			continue
		}
		if a.Kind() == AkCast && a.Mode()&3 == 0 && a.Mode()&(castMayPlay|castFace|castWarp) == 0 && a.Target() == 0 && hasOption(&real, a) {
			continue // priced by the printed sources: gorge's planner finds it sufficient and runs no prefix search for it
		}
		if a.Kind() == AkCast && a.Mode()&3 == 0 && a.Mode()&(castMayPlay|castFace|castWarp) == 0 && s.xWitnessUnreachable(k, p, Obj(a.Source())) {
			continue // the printed sources pay it but the witness eats a target: gorge's verdict is witness_unreachable, no prefix search (mana_sacsource.go)
		}
		if cd := &k.t.Cards[s.objs[Obj(a.Source())].card]; isCastKind(a.Kind()) && (s.spellTgtDep(k, p, s.objs[Obj(a.Source())].card, Obj(a.Source())) || cd.HasModeCost && a.Mode()&castMayPlay == 0 && !modalFloorZero(k, s.objs[Obj(a.Source())].card)) {
			continue // gorge searches no prefix for a target-priced cost, nor for a modal one whose cheapest mode still costs mana (a may-play cast is priced at its printed cost: potentialPlayVerdictShared's mayplay arm)
		}
		if isCastKind(a.Kind()) && a.Mode()&castMayPlay == 0 && k.gen.anyOptCost && optCostCard(k, s.objs[Obj(a.Source())].card) {
			continue // nor for an OptionalCost card: the planner declines it (shape:optional_cost, cost_optional.go); a may-play cast it composes (potentialModeCastPlan)
		}
		if s.objPartCast(k, a) && !k.t.Cards[s.objs[Obj(a.Source())].card].HasAltAdd || s.contribCast(k, p, a) {
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
	searched := 0
	for i := 0; i < nc && searched < potentialScriptPlays; i++ { // the first potentialScriptPlays: searched
		a := cand[i]
		if !hasOption(&gaps, a) && !twoLive {
			if !relax {
				searched++
			}
			continue
		}
		searched++
		if a.Kind() == AkCast && a.Mode()&3 == 0 && a.Mode()&(castMayPlay|castFace|castWarp) == 0 && a.Target() == 0 && s.cfg&cfgGorgeAutopay != 0 {
			continue // a plain cast: dropped by the autopay surface
		}
		if dh {
			if !s.dhScript(k, a, p, false) {
				continue // no script: the discarded hand held the play, or three mana of one colour do not pay it (mana_discardhand.go)
			}
		} else if (filt.any() || mix) && a.Kind() == AkActivate && s.liveGrantGap(k, p) {
			if !s.grantActScript(k, a, p) {
				continue // no script within gorge's one search's budget (grantActScript)
			}
		} else if filt.any() || mix {
			if !s.filtScript(k, a, p, false) {
				continue // no script within gorge's search budget
			}
		} else if tx.any() && s.tapXBoard(k, p) {
			if !s.tapXScript(k, a, p, false) && !(s.tapXSecondLive(k, p) && s.liveGrantGap(k, p) && s.grantActScript(k, a, p)) {
				continue // no script within gorge's budget (mana_tapanother.go), nor in the one search beside the grants (mana_granttapx.go)
			}
		} else if k.gen.anyRefl && s.reflBoard(k, p, restr, anyR) {
			if found, dead := s.reflScript(k, a, p, false); !found || dead {
				continue // no script within gorge's budget, or one the pool cannot complete (mana_reflected.go)
			}
		} else if anyR.any() || tapsDef.any() {
			if s.scriptDeadEndAny(k, a, p, restr, anyR, tapsDef) {
				continue // no script within gorge's budget (mana_window.go scriptDeadEndAny)
			}
		} else if !restr.any() && !s.scriptCtr(k, p).any() && s.eachFeeBoard(k, p) {
			if !s.eachScript(k, a, p, false) {
				continue // no script within gorge's search budget (mana_eachfee.go)
			}
		} else if restr.any() && s.scriptDeadEnd(k, a, p, restr) {
			continue
		}
		d.add(a)
	}
}

// walkKey orders insufficient play a (the kernel walk's option i) as
// gorge's potential walk lists it.
func (s *State) walkKey(a Action, p uint8, i int) uint64 {
	ob := &s.objs[Obj(a.Source())]
	switch {
	case (isCastKind(a.Kind()) || a.Kind() == AkPlot || a.Kind() == AkSneak) && ob.zone == kop.ZoneHand:
		for h := 0; h < int(s.pl[SeatIx(p)].handN); h++ {
			if s.pl[SeatIx(p)].hand[h] == Obj(a.Source()) {
				k := uint64(h)<<16 | uint64(i)
				if a.Mode() == 0 && a.Target() != 0 || a.Kind() == AkPlot || a.Kind() == AkSneak || a.Kind() == AkCast && a.Mode()&castWarp != 0 {
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
// script the oracle cannot complete. The search (scriptSearch.visit)
// activates the census gaps depth first in priority-option order, which is
// battlefield order: Bloom Tenders and restricted sources (Giada) alike.
// The planner prices the play from the floating pool without reading the
// restriction (Giada's Angel-only {W} counts toward Squad Rallier's
// {2}{W}), so the script is the battlefield-order chain cut where the walk
// lists a. Played, the planner reads the pool through the restriction, so the
// oracle offers a only when the walk lists it with the chain's mana floating,
// else "script dead end" (oracle gorge.go). Evaluated on a copy.
func (s *State) scriptDeadEnd(k *prep, a Action, p uint8, restr slotMask) bool {
	t := s.fork()
	defer s.unfork()
	// An activation's granted mana is the grant search's (grantCap's walk),
	// but availFor does not read a grant's spend rule: count the unit here
	// only when every live grant is unrestricted. A deferred source
	// (Pyromancer's Goggles) is no census unit, so a play the deferred gap
	// and an unrestricted grant together pay is absent from that walk and
	// must be priced here (restrictedGrantLive).
	t.g.noGrant = a.Kind() == AkActivate && s.restrictedGrantLive(k, p)
	var order [MaxPerm]Slot
	ctr := t.scriptCtr(k, p) // a counter-cost gap (Ramos) among them, as scriptPrefix plays it
	n := t.battlefieldOrder(p, t.scriptGaps(k, p).or(restr).or(ctr), order[:], 0)
	var d Decision
	var held [kop.NumManaKinds]uint8 // the restricted mana in the copy's pool
	for i := 0; i < n; i++ {
		if ctr.has(order[i]) {
			t.ctrGap(k, p, order[i], t.ctrPlayCol(k, a, order[i]))
			t.sba(k)
		} else {
			t.tapGap(k, p, order[i], restr, &held)
		}
		t.restrOptions(k, &d, p)
		if hasOption(&d, a) {
			if held == ([kop.NumManaKinds]uint8{}) {
				return false
			}
			return !t.scriptCompletes(k, p, a)
		}
	}
	return true
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
		s.floatDeferred(k, p, sl) // (the window's gap too: restr is empty there)
		return
	}
	if restr.has(sl) {
		c := s.cardAt(sl)
		ab := &k.t.Cards[c].Abilities[k.gen.restr(c)]
		col := bits.TrailingZeros8(uint8(ab.ManaColors))
		if !s.floatRestrAb(k, p, sl, ab, col, max(ab.ManaN, 1)) {
			s.capFull(StallRestrSrc) // never a silent spill
		}
		held[col] += max(ab.ManaN, 1)
		return
	}
	s.eachProduce(k, p, sl, s.eachDefaultAns(k, p, sl)) // (a fee-paid each-colour gap: mana_eachfee.go)
}

// scriptPrefix plays gorge's script for play a: p's gaps tapped in
// battlefield (entry) order, a restricted source among them (Giada), until
// the plain walk lists a (nothing when it already does). The restricted
// mana floats beside the pool (State.rb), priced by the search's walk
// as gorge's prefix search does (restrOptions).
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
		s.plainOptions(k, &d, 0)
		if !hasOption(&d, a) {
			s.dhScript(k, a, p, true) // Lion's Eye Diamonds the only gaps (mana_discardhand.go)
		}
		return
	}
	act := a.Kind() == AkActivate && s.liveGrantGap(k, p)
	if s.mixBoard(k, p) {
		if !act {
			var d Decision
			s.plainOptions(k, &d, 0)
			if !hasOption(&d, a) {
				s.filtScript(k, a, p, true)
			}
		}
		return
	}
	if (k.tapMana || k.ctrMana) && s.tapXPrefix(k, a, p) {
		return // a tap-another-permanent board (mana_tapanother.go)
	}
	gaps, restr, anyR := s.scriptGaps(k, p), s.scriptRestr(k, p), s.scriptAnyR(k, p)
	tapsDef := s.scriptTapsDefer(k, p)
	ctr := s.scriptCtr(k, p)
	if !gaps.or(restr).or(anyR).or(tapsDef).or(ctr).any() {
		// A live grant makes the grant search's candidates the board's
		// filters too, activated with the grants in battlefield order
		// (grantSrch.candidates): one search taps both gaps, so a filter
		// whose fee a granted {C} pays is not pre-empted here by a
		// filter-only script that would fund the fee from a land instead.
		if k.gen.anyFilt && s.filtOnly(k, p) && !s.liveGrantGap(k, p) {
			var d Decision
			s.plainOptions(k, &d, 0)
			// A live granted mana gap (a Humble Merchant) beside the filter:
			// gorge's one prefix search covers the board's filters with its
			// grants (mana_grantedoffers.go) and taps the granted gap first
			// (gaps before funding, scriptSearch.visit), its mana paying the
			// filter's fee — the filter script's funding would tap a printed
			// land the search leaves untapped, so grantPrefix applies it
			// instead (CR 601.2g-h)
			if !hasOption(&d, a) && !s.grantGapLive(k, p) {
				s.filtScript(k, a, p, true)
			}
		}
		return
	}
	if k.gen.anyRefl && s.reflBoard(k, p, restr, anyR) {
		var d Decision
		s.plainOptions(k, &d, 0)
		if !hasOption(&d, a) {
			s.reflScript(k, a, p, true) // (mana_reflected.go)
		}
		return
	}
	if anyR.any() || tapsDef.any() {
		s.scriptPrefixAny(k, a, p, restr, anyR, tapsDef)
		return
	}
	if !restr.any() && !ctr.any() && s.eachFeeBoard(k, p) {
		s.eachScript(k, a, p, true) // a fee-paid each-colour gap (Tarnation Vista): gorge's search, mana_eachfee.go
		return
	}
	var d Decision
	s.plainOptions(k, &d, 0)
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, gaps.or(restr).or(ctr), order[:], 0) // gorge's option order (two Tenders)
	var held [kop.NumManaKinds]uint8
	for i := 0; i < n && !hasOption(&d, a); i++ {
		if ctr.has(order[i]) {
			s.ctrGap(k, p, order[i], s.ctrPlayCol(k, a, order[i])) // a Wall of Roots: its counter, its mana (mana_countercost.go)
			s.sba(k)                                               // the script returns to priority after the step
		} else {
			s.tapGap(k, p, order[i], restr, &held)
		}
		s.restrOptions(k, &d, p)
	}
}

// FeatScripted reports whether p's priority option a is a scripted play:
// one only a gap's prefix pays, which gorge's net list (the planned
// payment actions and the decision's own options) does not hold.
func (s *State) FeatScripted(p uint8, a Action) bool {
	k := s.k()
	if !s.scriptAny(k) || !isScriptPlayKind(a.Kind()) || !s.scriptGaps(k, p).or(s.scriptRestr(k, p)).or(s.scriptAnyR(k, p)).or(s.scriptTapsDefer(k, p)).or(s.scriptCtr(k, p)).or(s.scriptTapXModelled(k, p)).any() && !(k.gen.anyFilt && s.filtOnly(k, p)) && !(k.gen.anyDiscHand && s.dhBoard(k, p)) {
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

func (s *State) grantCap(k *prep, d *Decision, p uint8, capSpent *Decision) {
	if !s.grantGapLive(k, p) {
		return
	}
	var walk Decision
	m := availWalk
	if s.selfGrantLive(k, p) {
		m |= availHyp // no relaxed proof: gorge's census is not relaxable (mana_owncolours.go)
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
		if k.gen.anyTapExtra {
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
			capSpent.add(a)
		}
	}
	for i := 0; i < int(walk.N); i++ {
		a := walk.Options[i]
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
		if s.affordsUngrantedPlay(k, p, a, &cost, ex, s.grantPayDesc(k, a)) {
			continue // priced by the printed sources, restricted ones the play may spend included
		}
		if s.contribCast(k, p, a) {
			// a convoke, improvise or delve cast (Dazzling Theater's granted
			// convoke): gorge's potential planner refuses the shape
			// ("shape:contribution") before any prefix search, so the printed
			// sources did not price it and it is not offered (CR 702.51a)
			d.remove(a)
			capSpent.add(a)
			continue
		}
		if !s.grantRelaxedAffords(k, p, a, &cost, ex, s.grantPayDesc(k, a), m) {
			d.remove(a) // gorge's relaxed proof: insufficient, no prefix search (mana_grantrelaxed.go)
			capSpent.add(a)
			continue
		}
		if k.tapMana && hasOption(d, a) && s.tapXOffGrant(k, p) && s.tapXScript(k, a, p, false) {
			continue // a tap-another source (Gene Pollinator) pays it in scriptOffers' search, which the grant search does not run (mana_tapanother.go)
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
			if !s.tapXOffGrant(k, p) || !s.tapXScript(k, a, p, false) {
				// Gorge's script search covers every census gap — granted units and
				// tap-another-permanent gaps alike (CR 605.1a, 601.2g-h) — so the
				// kernel drops the play only when neither route prices it.
				d.remove(a) // no prefix within gorge's node budget (mana_grantedoffers.go)
				// The play has had gorge's one prefix search (the grant search
				// covers a grant board's filters with its grants,
				// mana_grantedoffers.go): scriptOffers' filter search must not
				// price it again on a second budget.
				capSpent.add(a)
			}
		}
	}
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
func (s *State) grantActScript(k *prep, a Action, p uint8) bool {
	cost, ex, need, ok := s.grantPlayCost(k, p, a, true)
	if !ok {
		return s.filtScript(k, a, p, false)
	}
	var g grantSrch
	return s.grantSearch(k, p, &cost, ex, s.grantPayDesc(k, a), need, &g)
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
