package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Waterbend (CR 701.67): an activated ability's cost may include N generic
// that can be paid by tapping that many untapped artifacts and/or creatures
// instead. The offer credit (waterbendCredit) reduces the mana the ability
// is offered for; the picks are announced after the tap costs and before X,
// pay one generic each in pick order at payment, and cannot also be mana
// sources in the window. Gorge: raise_cost_extra.go withWaterbendOfferCredit,
// pay.ConvokeAsk (Kind "waterbend_generic"), cast_commit.go EmitConvokeTaps.

const pkWaterbend pickKind = 0x2000

// Debug hooks for the lockstep and mutation tests.
var (
	// DebugWaterbendNoCredit drops the offer credit.
	DebugWaterbendNoCredit bool
	// DebugWaterbendSrcTap offers a {T} ability's source as its own
	// waterbend pick (mutation).
	DebugWaterbendSrcTap bool
	// DebugWaterbendPaid reports a paid ability: its card, the waterbend
	// picks, and how many of them are mana sources.
	DebugWaterbendPaid func(card CardID, picks, manaPicks int)
)

// prepWaterbend sets anyWaterbend: some ability costs CostWaterbend.
func prepWaterbend(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if t.Cards[i].Abilities[j].Cost.Flags&kop.CostWaterbend != 0 {
				g.anyWaterbend = true
				return
			}
		}
	}
}

// waterbendCandidate reports whether sl can be tapped for a waterbend cost:
// untapped, an artifact or creature, and no attached bestow card (gorge also
// names a reconfigured card, which kgen does not lower). An equipped Equipment
// qualifies (CR 701.67a).
func (s *State) waterbendCandidate(k *prep, sl Slot) bool {
	pp := &s.perms[sl]
	if pp.flags&pfTapped != 0 || s.bestowAttached(k, sl) {
		return false
	}
	return s.typesAt(k, sl)&(kop.TArtifact|kop.TCreature) != 0
}

// waterbendManaSource reports whether sl is an untapped mana source the
// offer credit must not count: gorge's withWaterbendOfferCredit excludes every
// untapped permanent with a usable mana ability (untappedManaSource ->
// availableManaAbilities), so a summoning-sick creature whose mana ability
// needs its {T} is NOT a source this turn and does earn the credit
// (canTapForMana carries the same tap-sick gate).
func (s *State) waterbendManaSource(k *prep, sl Slot) bool {
	return s.canTapForMana(k, sl)
}

// waterbendCredit returns cost less the greedy waterbend offer credit for
// ability a: in p's battlefield order, each untapped artifact/creature that is
// not an untapped mana source pays one generic, up to the ability's amount. The
// source counts unless its own {T} is part of the cost (CR 701.67a, 118.3).
// Gorge: withWaterbendOfferCredit.
func (s *State) waterbendCredit(k *prep, p uint8, src Slot, a *kop.Ability, cost kop.ManaCost) kop.ManaCost {
	n := int(a.Cost.N)
	if k.gen == nil || !k.gen.anyWaterbend || DebugWaterbendNoCredit || n <= 0 || cost.Generic == 0 {
		return cost
	}
	if a.Cost.Flags&kop.CostTap == 0 {
		src = NoSlot
	}
	var ord [MaxPerm]Slot
	cnt := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), ord[:], 0)
	credit := 0
	for _, sl := range ord[:cnt] {
		if credit >= n || credit >= int(cost.Generic) {
			break
		}
		if sl == src || !s.waterbendCandidate(k, sl) || s.waterbendManaSource(k, sl) {
			continue
		}
		credit++
	}
	cost.Generic -= uint8(credit)
	return cost
}

// waterbendPlanCost is cost less the offer credit when object o's ability ab costs a
// waterbend: what a source cannot fund, the taps may, so gorge's planner prices the
// credited cost before it calls a play unpayable (planAbilityPaymentLifePips).
func (s *State) waterbendPlanCost(k *prep, p uint8, o Obj, ab uint8, cost kop.ManaCost) kop.ManaCost {
	ob := &s.objs[o]
	if !k.gen.anyWaterbend || ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return cost
	}
	a := &k.t.Cards[s.cardAt(ob.slot)].Abilities[ab]
	if a.Cost.Flags&kop.CostWaterbend == 0 {
		return cost
	}
	return s.waterbendCredit(k, p, ob.slot, a, cost)
}

// waterbendPoolPaidOffer reports whether the activation being built is a
// waterbend ability whose offer credit (waterbendCredit) the floating pool
// alone pays: gorge's offer gate priced that credited cost, so it is an
// offered option, not a potential play the witness lowers. The picks are
// asked next and waterbendManualPay settles or aborts the payment.
func (s *State) waterbendPoolPaidOffer(k *prep, p uint8, cost kop.ManaCost) bool {
	credited, ok := s.waterbendBuildCredit(k, p, cost)
	return ok && s.poolPays(p, &credited)
}

// waterbendBuildCredit is waterbendCredit for the activation being built; false when
// it isn't a waterbend ability, and cost goes back as it came. A witness is priced on
// the credit too (gorge planAbilityPaymentLifePips).
func (s *State) waterbendBuildCredit(k *prep, p uint8, cost kop.ManaCost) (kop.ManaCost, bool) {
	if k.gen == nil || !k.gen.anyWaterbend {
		return cost, false
	}
	e := &s.bld().e
	if e.kind != seAbility || e.ab == 255 || e.ab >= abDelayed {
		return cost, false
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.Cost.Flags&kop.CostWaterbend == 0 {
		return cost, false
	}
	return s.waterbendCredit(k, p, s.srcSlot(), a, cost), true
}

// waterbendApplyPicks subtracts the announced waterbend picks from cost in
// pick order (gorge applyConvoke's waterbend branch).
func (s *State) waterbendApplyPicks(cost kop.ManaCost) kop.ManaCost {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkWaterbend && cost.Generic > 0 {
			cost.Generic--
		}
	}
	return cost
}

func (s *State) waterbendPicked(o Obj) bool {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().picks[i] == o && s.bld().pickKs[i] == pkWaterbend {
			return true
		}
	}
	return false
}

func (s *State) waterbendPickCount() int {
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkWaterbend {
			n++
		}
	}
	return n
}

// waterbendOptions poses the next waterbend pick: one option per eligible
// permanent in battlefield order, Done closing the announcement. The ask
// commits once it reaches gorge's Max = min(#options, N); a first ask with
// no option is skipped. Done waits for the taps the payment can't do without
// (waterbendTapsNeeded).
//
//go:noinline
func (s *State) waterbendOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if k.gen == nil || !k.gen.anyWaterbend || e.kind != seAbility || e.ab == 255 || e.ab >= abDelayed || s.bld().donePicks&pkWaterbend != 0 {
		return false
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.Cost.Flags&kop.CostWaterbend == 0 {
		return false
	}
	n := int(a.Cost.N)
	if n <= 0 || a.Cost.Mana.Generic == 0 {
		return false
	}
	// gorge castasks.go:76-80 (SourceTaps, b986f411f): a {T} ability's
	// source taps for {T}, so it is never one of its own waterbend picks
	// (CR 118.3). Same rule as the tap-X pick (cast.go:1578).
	self := NoSlot
	if a.Cost.Flags&kop.CostTap != 0 && !DebugWaterbendSrcTap {
		self = s.srcSlot()
	}
	var ord [MaxPerm]Slot
	cnt := s.battlefieldOrder(e.ctrl, s.ctrlM[e.ctrl].and(s.occ), ord[:], 0)
	offered := 0
	for _, sl := range ord[:cnt] {
		if !s.waterbendCandidate(k, sl) || sl == self {
			continue
		}
		if s.pickedKind(s.perms[sl].obj, pkTapX) {
			continue
		}
		offered++
	}
	maxPicks := offered
	if n < maxPicks {
		maxPicks = n
	}
	if g := int(a.Cost.Mana.Generic); g < maxPicks {
		maxPicks = g
	}
	if s.waterbendPickCount() >= maxPicks {
		return false
	}
	d.N = 0
	d.Kind, d.Sub, d.Player = DkChoose, ChWaterbend, e.ctrl
	pose := false
	for _, sl := range ord[:cnt] {
		if !s.waterbendCandidate(k, sl) || sl == self {
			continue
		}
		o := s.perms[sl].obj
		if s.waterbendPicked(o) || s.pickedKind(o, pkTapX) {
			continue
		}
		pose = true
		if d.N >= MaxOptions-1 {
			s.capFull(StallTapPick)
			break
		}
		d.add(MakeAction(AkChoose, uint8(o), 0, 0))
	}
	if !pose && s.waterbendPickCount() == 0 {
		return false
	}
	if s.waterbendPickCount() >= s.waterbendTapsNeeded(k, e, a) {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	s.bld().pickK = pkWaterbend
	return true
}

// waterbendTapsNeeded is the fewest taps the announcement can make and still
// leave the mana window able to finish (CR 601.2g-h, 701.67a). Gorge reads it
// once, before any pick, so a pick still counts here as the mana source it would
// otherwise be. An unknown amount or a window the walk can't price needs none.
// Gorge: waterbendTapsNeeded.
func (s *State) waterbendTapsNeeded(k *prep, e *stackEntry, a *kop.Ability) int {
	if a.Cost.Mana.X != 0 || !s.windowCensusComplete(k, e) {
		return 0
	}
	b := s.bld()
	kinds := b.pickKs
	for i := range b.pickKs[:b.pickN] {
		if b.pickKs[i] == pkWaterbend {
			b.pickKs[i] = 0
		}
	}
	defer func() { b.pickKs = kinds }()
	cost := s.buildCost(k)
	for n := 0; n <= min(int(a.Cost.N), int(cost.Generic)); n++ {
		c := cost
		c.Generic -= uint8(n)
		ok, known := s.windowDry(k, e.ctrl, &c, b.bfl&bflAnyColor != 0)
		if !known {
			return 0
		}
		if ok {
			return n
		}
	}
	return 0
}

// windowCensusComplete reports whether gorge's window census prices every source of
// the payer: a permanent that can tap for a restricted mana ability this payment may
// spend understates what the window can produce (CR 106.6). Gorge:
// castWindowCensusComplete.
func (s *State) windowCensusComplete(k *prep, e *stackEntry) bool {
	if !k.gen.anyRestr && !k.gen.anyRestrSrc && !k.gen.anyAnyR && !k.gen.anyTapExtra {
		return true
	}
	d := s.payDescNow(NoObj)
	var ord [MaxPerm]Slot
	cnt := s.battlefieldOrder(e.ctrl, s.ctrlM[e.ctrl].and(s.occ), ord[:], 0)
	for _, sl := range ord[:cnt] {
		pm := &s.perms[sl]
		if pm.flags&pfTapped != 0 || pm.flags&pfSick != 0 && s.isCreature(k, sl) && s.kw(k, sl)&kop.KwHaste == 0 || s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind == kop.AbMana && (a.ManaRestrict != 0 || a.ManaRestrictAb != 0) && s.restrSrcAdmits(k, e.ctrl, sl, a, &d) {
				return false
			}
		}
		// A tap trigger that adds mana to this source's production is one the
		// window census doesn't price (gorge manaProductionAltered, CR 605.1b).
		if k.gen.anyTapExtra {
			if _, hit := s.srcExtraHit(k, sl, s.manaPCAt(k, sl)); hit {
				return false
			}
		}
	}
	return true
}

// waterbendDone closes the waterbend pick; the next buildStep advances past
// bsTapCost.
func (s *State) waterbendDone() {
	s.bld().donePicks |= pkWaterbend
	s.bld().pickK, s.bld().pickLeft = 0, 0
}

// payWaterbend taps the announced permanents in pick order after the mana
// cost is paid (gorge EmitConvokeTaps); a pick that has left the
// battlefield is skipped.
func (s *State) payWaterbend(k *prep) {
	picks, manaPicks := 0, 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] != pkWaterbend {
			continue
		}
		picks++
		o := s.bld().picks[i]
		if s.objs[o].zone != kop.ZoneBattlefield {
			continue
		}
		sl := s.objs[o].slot
		if s.canTapForMana(k, sl) {
			manaPicks++
		}
		was := s.perms[sl].flags & pfTapped
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		if was == 0 && k.gen != nil && s.subs[kop.TrigTaps].any() {
			s.fireTaps(k, sl)
		}
	}
	if DebugWaterbendPaid != nil && s.bld().e.ab != 255 && k.t.Cards[s.bld().e.card].Abilities[s.bld().e.ab].Cost.Flags&kop.CostWaterbend != 0 {
		DebugWaterbendPaid(s.bld().e.card, picks, manaPicks)
	}
}

// waterbendManualPay reports whether the ability entry e pays its cost the
// way gorge's fallback window does. The offer arms a plan for the printed cost,
// or for the cost less the offer credit when the sources cannot fund the printed
// one (potential_plan.go). Once the picks settle the cost, gorge plans again for
// it without them, and falls back to the manual window only when no plan exists
// (rearmAbilityAutoPay): the window taps sources in battlefield order until the
// reduced cost is paid, filtering them by the composed full cost and excluding the
// picks (convokeCommitted). saved holds which picks holdCostTaps tapped. A plan
// that stands is tapped here, so the pool pays what follows.
func (s *State) waterbendManualPay(k *prep, e *stackEntry, saved *[len(build{}.picks)]uint8) bool {
	if k.gen == nil || !k.gen.anyWaterbend || e.kind != seAbility || e.ab == 255 {
		return false
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.Cost.Flags&kop.CostWaterbend == 0 {
		return false
	}
	full := a.Cost.Mana
	if s.waterbendPickCount() == 0 {
		// every credited pick declined: the offer gate priced the reduced
		// cost, so the printed cost may be unpayable and gorge's window then
		// aborts the activation (CR 733.1) after tapping as it walked
		cost := s.buildCost(k)
		var chk planner
		chk.dry = true
		s.tapForPK(k, e.ctrl, &cost, int(e.x), NoObj, false, false, &chk)
		return chk.needT > 0 && !chk.found
	}
	if s.windowPoolPays(e.ctrl, &full, false) {
		return false
	}
	s.releaseCostTaps(k, saved)
	credited := s.waterbendCredit(k, e.ctrl, s.srcSlot(), a, full)
	s.retakeCostTaps(saved)
	cost := s.buildCost(k)
	if !s.abilityPlanSettles(k, e, full, credited, cost, saved) {
		return true
	}
	s.tapFor(k, e.ctrl, &cost, int(e.x), NoObj)
	return false
}

// waterbendTargetAbort reports whether the pending waterbend activation can
// no longer be completed: the cost less the announced picks is unpayable from
// the pool and the CR 601.2g window (the picks are withheld, as at payment).
// Gorge's affordableTargetCandidates probes the same window after the picks
// are committed; every candidate is withheld, so a required target reverses
// the activation ("cast aborted: no legal target") and an up-to target asks
// nothing and reverses at payment.
func (s *State) waterbendTargetAbort(k *prep) bool {
	if k.gen == nil || !k.gen.anyWaterbend {
		return false
	}
	e := &s.bld().e
	if e.kind != seAbility || e.ab == 255 || e.ab >= abDelayed {
		return false
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.Cost.Flags&kop.CostWaterbend == 0 {
		return false
	}
	// an incomplete census keeps the offer (gorge affordableTargetCandidates)
	if !s.windowCensusComplete(k, e) {
		return false
	}
	// buildCost already subtracts the announced picks (gorge's
	// affordableTargetCandidates folds them through applyConvoke before the
	// affordability probe), so the probe prices the cost the payment will
	// charge. The picks are withheld from the window, as at payment.
	cost := s.buildCost(k)
	saved := s.fork()
	defer s.unfork()
	var hide slotMask
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkWaterbend && s.objs[s.bld().picks[i]].zone == kop.ZoneBattlefield {
			hide = hide.with(s.objs[s.bld().picks[i]].slot)
		}
	}
	s.occ = s.occ.andNot(hide)
	ok, _ := s.windowTapRider(k, e.ctrl, &cost, false, s.bld().bfl&bflAnyColor != 0)
	s.CopyFrom(saved)
	return !ok
}
