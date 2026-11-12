package dirac

import (
	"math/bits"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// featview.go exports the facts the network-input encoder (package
// dirac/feat) reads beyond Projection: the battlefield in entry order and
// an object's current keywords. Read-only; no allocation.

// FeatBattlefield appends the objects controller p has on the battlefield
// to dst in entry order (perm timestamp ascending: gorge's battlefield zone
// list order) and returns it.
func (s *State) FeatBattlefield(p uint8, dst []Obj) []Obj {
	n0 := len(dst)
	m := s.ctrlM[SeatIx(p)].and(s.occ)
	for m.any() {
		sl := m.first()
		m = m.rest()
		dst = append(dst, s.perms[sl].obj)
		for j := len(dst) - 1; j > n0 && s.perms[s.objs[dst[j]].slot].ts < s.perms[s.objs[dst[j-1]].slot].ts; j-- {
			dst[j], dst[j-1] = dst[j-1], dst[j]
		}
	}
	return dst
}

// FeatKeywords is object o's keyword mask: a battlefield permanent's derived
// keywords (printed unless it lost all abilities, plus until-end-of-turn,
// override and static grants), and printed reports whether the card's
// printed keywords are among them. Elsewhere it is the card's printed mask.
func (s *State) FeatKeywords(o Obj) (m kop.KwMask, printed bool) {
	ob := &s.objs[o]
	k := s.k()
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return k.cards[ob.card].kw, true
	}
	p := &s.perms[ob.slot]
	return s.kw(k, ob.slot), p.ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0
}

// FeatGranted is the keywords battlefield object o gains beyond its printed
// ones -- until-end-of-turn grants, its override record, a flying counter,
// Continuous statics -- whether or not it already has them printed (gorge
// lists a granted keyword again after the printed list), and whether its
// printed keywords still apply.
func (s *State) FeatGranted(o Obj) (granted kop.KwMask, printed bool) {
	g, _, printed := s.FeatGrantedRep(o)
	return g, printed
}

// FeatKwReps counts, per keyword bit, the listings of a granted keyword
// past its first (saturating at 255): gorge lists a keyword once per
// effect that grants it (two Fleeting Flights list flying twice; Soulstone
// Sanctuary activated 33 times lists vigilance 33 times; Electroduplicate's
// copy of its own hasty copy lists haste twice).
type FeatKwReps [32]uint8

// add records grant m on top of the bits already granted.
func (r *FeatKwReps) add(granted, m kop.KwMask) {
	for x := granted & m; x != 0; x &= x - 1 {
		if b := bits.TrailingZeros32(uint32(x)); r[b] < 255 {
			r[b]++
		}
	}
}

// FeatGrantedRep is FeatGranted with the repeats (FeatKwReps).
func (s *State) FeatGrantedRep(o Obj) (granted kop.KwMask, rep FeatKwReps, printed bool) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return 0, rep, true
	}
	k := s.k()
	p := &s.perms[ob.slot]
	add := func(m kop.KwMask) {
		rep.add(granted, m)
		granted |= m
	}
	add(p.eotKw)
	add(p.eotKwRep[0])
	add(p.eotKwRep[1])
	sup := false
	if k.gen != nil && s.nkwx != 0 {
		sup = !s.genLost(k, ob.slot) // a loss ended: its suppressed grants are back
	}
	for i := 0; i < int(s.nkwx); i++ {
		if r := &s.kwx[i]; r.sl == ob.slot && (r.bit&kwSup == 0 || sup) {
			for j := 0; j < int(r.n); j++ {
				add(kop.KwMask(1) << (r.bit &^ kwSup))
			}
		}
	}
	add(p.ovr.kw)
	if s.fxM.has(ob.slot) {
		s.copyExcKwReps(k, ob.slot, &rep)
	}
	if kw := ctrKw(s, p); kw != 0 {
		add(kw)
	}
	if k.gen != nil {
		s.grantedKwGen(k, ob.slot, add)
		return granted, rep, !s.genLost(k, ob.slot)
	}
	if s.statM.any() {
		_, _, sk := s.staticPT(k, ob.slot)
		add(sk)
	}
	return granted, rep, p.ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0
}

// featCardMatch is filter id read on a card that is not a permanent (gorge
// matches an Affected$ spec against the object as it is): the type,
// controller, colour, predicate and subtype tests, and the conditions a
// card off the battlefield can meet. ctrl is the static's controller.
func (s *State) featCardMatch(k *prep, id kop.FilterID, ob *object, ctrl uint8, srcCard CardID, srcSl Slot) bool {
	for id != 0 {
		if s.featCardMatch1(k, id, ob, ctrl, srcCard, srcSl) {
			return true
		}
		id = k.t.Filters[id].Or
	}
	return false
}

func (s *State) featCardMatch1(k *prep, id kop.FilterID, ob *object, ctrl uint8, srcCard CardID, srcSl Slot) bool {
	f := &k.t.Filters[id]
	if f.CtrlTgt != 0 {
		return false // a live battlefield target's controller; a feature read has none
	}
	if k.gen != nil {
		// the generated matcher refuses a with-flying card test outright;
		// this function reads the F2 conditions itself (below)
		fl := *f
		fl.Or = 0
		fl.Flags2 &^= kop.F2WithFlying | kop.F2WithoutFlying | kop.F2Basic | kop.F2Legendary | kop.F2SameNameAsSource | kop.F2ChosenColor | kop.F2ChosenType
		if !s.matchCardOne(k, &fl, ob.card, ob.owner(), ctrl, NoObj) {
			return false
		}
	} else if !s.matchCard(k, id, ob.card, ob.owner(), ctrl) {
		return false
	}
	const permOnly = kop.FSelf | kop.FAttachedBySrc | kop.FTapped | kop.FAttacking | kop.FBlocking |
		kop.FAttackingOrBlocking | kop.FToken | kop.FSpell
	if f.Flags&permOnly != 0 {
		return false
	}
	pc := &k.cards[ob.card]
	if f.Flags&kop.FCreatureOrPW != 0 && pc.types&(kop.TCreature|kop.TPlaneswalker) == 0 {
		return false
	}
	switch f.Owner {
	case kop.CtrlYou:
		if ob.owner() != ctrl {
			return false
		}
	case kop.CtrlOpp:
		if ob.owner() == ctrl {
			return false
		}
	}
	subs := k.t.Cards[ob.card].Subtypes
	if f.Sub != 0 && !subs.Has(int(f.Sub-1)) {
		return false
	}
	if f.NotSub != 0 && subs.Has(int(f.NotSub-1)) {
		return false
	}
	for _, n := range f.NotSubs {
		if n != 0 && subs.Has(int(n-1)) {
			return false
		}
	}
	if f.Zone != kop.ZoneNone && f.Zone != ob.zone {
		return false
	}
	f2 := f.Flags2
	if f2&(kop.F2ChosenColor|kop.F2ChosenType) != 0 {
		// the static source's choice (Heraldic Banner's colour)
		if srcSl == NoSlot {
			return false
		}
		ch := s.perms[srcSl].chosen
		cd := &k.t.Cards[ob.card]
		if f2&kop.F2ChosenColor != 0 && (ch > 4 || cd.Colors&(kop.ColW<<ch) == 0) {
			return false
		}
		if f2&kop.F2ChosenType != 0 && (int(ch) >= len(k.t.Subtypes) || !cd.Subtypes.Has(int(ch))) {
			return false
		}
		f2 &^= kop.F2ChosenColor | kop.F2ChosenType
	}
	if f2&(kop.F2Kicked|kop.F2CastByYou|kop.F2EnteredThisTurn|kop.F2ChosenType|kop.F2ChosenColor|
		kop.F2Remembered|kop.F2ChosenCard|kop.F2AttachedToT0|kop.F2TargetedBy|kop.F2Emblem|kop.F2Opponent|kop.F2You|kop.F2WasDamaged|kop.F2Saddled|kop.F2BlockedBySrc|kop.F2DamagedBy|kop.F2CrewedSrc|kop.F2TgtPlayerCtrl|kop.F2DefendingCtrl) != 0 {
		return false
	}
	if f2&kop.F2WithFlying != 0 && pc.kw&kop.KwFlying == 0 {
		return false
	}
	if f2&kop.F2WithoutFlying != 0 && pc.kw&kop.KwFlying != 0 {
		return false
	}
	if f2&kop.F2Basic != 0 && pc.types&kop.TBasic == 0 || f2&kop.F2Legendary != 0 && pc.types&kop.TLegendary == 0 {
		return false
	}
	if f2&kop.F2SameNameAsSource != 0 && ob.card != srcCard {
		return false
	}
	return true
}

// featCtrl is the controller of object o off the battlefield: a spell's is
// its stack entry's (a card Etali or a graveyard permission casts from
// another player's zone), any other card's its owner.
func (s *State) featCtrl(o Obj) uint8 {
	ob := &s.objs[o]
	if ob.zone == kop.ZoneStack {
		for i := int(s.sp) - 1; i >= 0; i-- {
			if e := &s.stack[i]; e.kind == seSpell && e.obj == o {
				return e.ctrl
			}
		}
	}
	return ob.owner()
}

// FeatCDAPT is object o's characteristic-defining P/T off the battlefield
// (gorge's CDASetPT applies a CDA in every zone, CR 604.3): the card's
// StatCDAPT amounts, evaluated as the layer walk evaluates them on the
// battlefield (evalV), with o as the source and no battlefield slot; ok is
// false only when the card has no such ability.
func (s *State) FeatCDAPT(o Obj) (p, t int, ok bool) {
	ob := &s.objs[o]
	k := s.k()
	cd := &k.t.Cards[ob.card]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCDAPT {
			continue
		}
		ctx := evalCtx{src: o, srcSl: NoSlot, ctrl: s.featCtrl(o)}
		return s.evalV(k, a.Static.VA, ctx), s.evalV(k, a.Static.VB, ctx), true
	}
	return 0, 0, false
}

// FeatPT is battlefield object o's power and toughness as gorge's view shows it. A
// creature or a face-down permanent has its derived pair. Any other permanent has its
// printed one, so a Vehicle still shows its own and counters move nothing (CR 208.3).
func (s *State) FeatPT(o Obj) (p, t int) {
	k, sl := s.k(), s.objs[o].slot
	if !s.isCreature(k, sl) && !s.faceDown(k, sl) {
		cd := &k.t.Cards[s.cardAt(sl)]
		return int(cd.Power), int(cd.Toughness)
	}
	return s.pt(k, sl)
}

// FeatMayCount is the number of live mayPlay records (oracle lockstep logs
// the peak; plot and warp ride them, so it must stay well under maxMay).
func (s *State) FeatMayCount() int { return int(s.g.nmay) }

// FeatSource is the object the pending decision is about when its options
// name none themselves: the spell or ability being put on the stack (its
// modes) or the one resolving (a mid-resolution choice); NoObj otherwise.
func (s *State) FeatSource() Obj {
	switch s.ctl {
	case ctlBuild:
		return s.bld().e.obj
	case ctlResolve:
		return s.res.e.obj
	}
	return NoObj
}

// FeatBlocked reports whether battlefield object o is an attacker that
// became blocked this combat (gorge's non-empty BlockedBy, which keeps a
// blocker that has since died).
func (s *State) FeatBlocked(o Obj) bool {
	ob := &s.objs[o]
	return ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot && s.perms[ob.slot].flags&pfBlocked != 0
}

// FeatArrangeGraveyard reports whether a pending ChArrange decision is a
// surveil (the unpicked cards go to the graveyard) rather than a scry.
func (s *State) FeatArrangeGraveyard() bool {
	return s.ctl == ctlResolve && s.res.ask == raSurveil
}

// FeatArrangeDig reports whether the ChArrange decision is a dig's rest
// ordered onto the bottom (gorge's "dig_bottom" options).
func (s *State) FeatArrangeDig() bool {
	return s.ctl == ctlResolve && s.res.ask == raGenDigArrange
}

// FeatFirstStrikeCombat reports whether this combat has a first-strike
// damage step.
func (s *State) FeatFirstStrikeCombat() bool { return s.fsStep }

// FeatReplOrderCombat reports whether a replacement-order ask is posed on a
// combat damage event (else on a noncombat one, mid-resolution: priority
// stays with its holder).
func (s *State) FeatReplOrderCombat() bool { return s.g.ro.on }

// FeatActivations is object o's non-mana activations this turn (gorge's
// ActivatedThisTurn).
func (s *State) FeatActivations(o Obj) int { return int(s.objs[o].activations()) }

// FeatSickOff is the summoning-sick bit object o carried when it last left
// the battlefield (gorge's stale Object.SummonSick off the battlefield).
func (s *State) FeatSickOff(o Obj) bool { return s.objs[o].of&ofSickLeft != 0 }

// FeatPoolPays reports whether p's floating pool alone pays the cast of o
// in kernel cast mode (bit 0 kicked, bit 1 flashback, bit 3 the retrace
// cast, with bit 1) -- gorge offers a
// cast as a legacy option exactly then; otherwise only the planner pays it.
func (s *State) FeatPoolPays(p uint8, o Obj, mode uint8) bool {
	return s.FeatPoolPaysPlus(p, o, mode, nil)
}

// FeatPoolPaysPlus is FeatPoolPays over the pool plus extra mana (W U B R G C):
// what the pool would pay once the bot had floated that much by hand.
func (s *State) FeatPoolPaysPlus(p uint8, o Obj, mode uint8, extra *[6]int32) bool {
	k := s.k()
	pl := &s.pl[SeatIx(p)]
	a := s.poolAvailPlus(p, extra)
	c := s.objs[o].card
	if s.rbAny(p) {
		s.restrAvail(k, p&1, c, s.restrOriginOfObj(o), &a) // a spell its restriction allows sees it (gorge AvailableFor)
	}
	cost := k.cards[c].cost
	if mode&8 != 0 {
		cost = k.t.Cards[c].Cost // the retrace cast's printed cost, whatever flashback the card also holds (kw_retrace.go)
	} else if mode&2 != 0 {
		cost = s.flashbackCostOf(k, o)
	} else if mode&1 != 0 {
		cost = addCost(cost, k.t.Cards[c].Kicker)
	}
	if mode == 0 && k.gen != nil && k.gen.anyConvoke {
		cost = s.convokeCredit(k, p&1, c, o, cost)
	}
	tb := false
	if mode&4 != 0 || mode&1 != 0 && k.bargainRed && k.gen != nil && k.gen.anyOptCost && optCostCard(k, c) {
		cost = s.bargainedPrice(k, p&1, c, o, cost) // the bargained cast's price, or the optional-cost cast's (cost_reducespell.go)
	} else if s.statM.any() || k.gen != nil {
		// a reduction applies to the total cost, a flashback or kicked one
		// too (CR 601.2f; the kernel's buildCost): Archmage of Runes on a
		// flashback Inspiration from Beyond
		tb = cost.Hyb[0] == kop.HybTwobrid
		a.delve = s.tbDelveCredit(k, p&1, c, o, mode&2 == 0) // the faces' credit (CR 702.66a)
		cost = s.adjustFit(k, &a, p&1, c, o, cost, false)    // a twobrid cost is priced on the face the pool pays
	}
	if k.gen != nil && k.t.Cards[c].SpellWaterbend != 0 {
		cost = s.waterbendSpellCredit(k, p&1, c, o, cost) // act_waterbend.go
	}
	if mode&1 != 0 && k.gen != nil && k.t.Cards[c].OptWaterbend != 0 {
		// the kicked cast's OptionalCost Waterbend<N> credits with the own part (act_waterbend.go)
		cost = s.waterbendCreditN(k, p&1, NoSlot, s.waterbendOwnPart(k, p&1, c, o)+int(k.t.Cards[c].OptWaterbend), cost)
	}
	if mode&2 != 0 && k.t.Cards[c].Harmonize {
		cost = s.harmonizeCredit(k, p&1, cost) // gorge harmonizePayment prices the offer after the reductions
	}
	if cd := &k.t.Cards[c]; cd.HasAltAdd && mode&2 == 0 {
		pre := cost
		// gorge's gate is existential over the either-or parts: one whose
		// picks can be paid and whose mana the pool pays with the spell's
		ac := spellAddCost(k, c)
		if s.pickCostPayable(k, p&1, ac, NoObj) && (ac.Flags&kop.CostDiscard == 0 || int(pl.handN)-s.inHandN(o) >= int(ac.N)) && s.beholdPayable(k, p&1, c, o) {
			t := addCost(pre, ac.Mana)
			if a.delve == 0 || !tb { // the faces took the credit (tb: adjustFit priced them)
				t = s.delveOfferCredit(k, p&1, c, o, true, t)
			}
			if a.affords(&t, pipMaskOf(&t), 0) {
				return true
			}
		}
		if !s.pickCostPayable(k, p&1, &cd.AltAddCost, NoObj) {
			return false
		}
		cost = addCost(cost, cd.AltAddCost.Mana)
	}
	if mode&4 == 0 && (a.delve == 0 || !tb) {
		cost = s.delveOfferCredit(k, p&1, c, o, mode&2 == 0, cost)
	}
	return a.affords(&cost, pipMaskOf(&cost), 0)
}

// poolAvailPlus is p's floating pool plus extra as an avail.
func (s *State) poolAvailPlus(p uint8, extra *[6]int32) avail {
	pl := &s.pl[SeatIx(p)]
	var a avail
	for c := 0; c < kop.NumManaKinds; c++ {
		n := int32(pl.pool[c])
		if extra != nil && c < len(extra) {
			n += extra[c]
		}
		if n > 0 {
			a.addUnits(1<<c, uint8(n))
		}
	}
	return a
}

// FeatFacePoolPays is FeatPoolPaysPlus for the face cast of o whose action
// target is face: the price of that face, which gorge prices before it lists
// the cast.
func (s *State) FeatFacePoolPays(p uint8, o Obj, face uint8, extra *[6]int32) bool {
	k := s.k()
	c := s.objs[o].card
	if fs := k.t.Cards[c].Faces; int(face) < len(fs) && face != 0 {
		c = fs[face]
	}
	cost := s.adjustCost(k, p&1, c, o, k.cards[c].cost)
	a := s.poolAvailPlus(p, extra)
	return a.affords(&cost, pipMaskOf(&cost), 0)
}

// poolPays reports whether p's floating pool alone pays cost.
func (s *State) poolPays(p uint8, cost *kop.ManaCost) bool {
	pl := &s.pl[SeatIx(p)]
	var a avail
	for c := 0; c < kop.NumManaKinds; c++ {
		if pl.pool[c] > 0 {
			a.addUnits(1<<c, pl.pool[c])
		}
	}
	return a.affords(cost, pipMaskOf(cost), 0)
}

// poolPaysAnyColor is poolPays under the may-play any-colour rider.
func (s *State) poolPaysAnyColor(p uint8, cost *kop.ManaCost) bool {
	pl := &s.pl[SeatIx(p)]
	var a avail
	for c := 0; c < kop.NumManaKinds; c++ {
		if pl.pool[c] > 0 {
			a.addUnits(1<<c, pl.pool[c])
		}
	}
	return a.affordsAnyColor(cost, 0)
}

// FeatManaSources counts the permanents p can activate a mana ability of
// now: gorge's priority list carries one manual "activate" option for each,
// whether or not its autopay planner can use the source (a chosen-colour,
// dynamic-amount or restricted ability -- Heraldic Banner, Elvish
// Archdruid, Giada -- is still offered by hand).
func (s *State) FeatManaSources(p uint8) int {
	k := s.k()
	n := 0
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		if s.featManaActivatable(k, sl) {
			n++
		}
	}
	return n
}

// featManaActivatable reports whether slot sl has a mana ability whose
// cost can be paid now: printed (unless its abilities are lost) or
// granted; a {T} cost needs it untapped and, for a creature, not summoning
// sick (or hasty), and a cost that taps others needs enough of them untapped.
func (s *State) featManaActivatable(k *prep, sl Slot) bool {
	pm := &s.perms[sl]
	tapOK := pm.flags&pfTapped == 0 &&
		!(s.activationSick(k, sl))
	if k.gen != nil && s.manaGranted(k, sl) {
		return tapOK
	}
	if k.gen != nil {
		if s.genLost(k, sl) {
			return false
		}
	} else if pm.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
		return false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if a.Cost.Flags&kop.CostTap != 0 && !tapOK {
			continue
		}
		if a.Cost.Flags&kop.CostTapX != 0 && s.countUntapped(k, pm.ctrl, a.Cost.Filter, sl)-s.selfTapCounted(k, a, sl) < int(a.Cost.N) {
			continue
		}
		return true
	}
	return false
}

// FeatPlannedCast reports whether gorge's payment planner offers p's cast
// of o in kernel cast mode (0 plain, bit 1 flashback) as a payment action.
// It does not for a spell with an {X} cost, an either-or additional cost
// (Eaten Alive) or a target-conditional cost reduction (Luminous Rebuke), nor when
// the cast is payable only with a mana ability an effect grants
// (Imprisoned in the Moon's {T}: Add {C}): gorge's source census holds
// printed and intrinsic abilities only (rules/pay PaymentAbility: "grants,
// merged and foreign abilities are V1 exclusions"). gorge reaches those
// casts only by lowering a potential play.
func (s *State) FeatPlannedCast(p uint8, o Obj, mode uint8) bool {
	k := s.k()
	if s.objs[o].zone == kop.ZoneLibrary {
		return false // gorge plans the hand, command, exile and graveyard origins only (payment_plan.go): a top-of-library cast is always lowered
	}
	if mode&2 != 0 {
		if two, _, g := s.fbInstances(k, p&1, o); two && g {
			// the granted instance beside a printed one carries its own
			// cost (Option.Cost), which the payment-action builder never
			// plans (planCastFlashbackOption; pair 1841)
			return false
		}
	}
	cost, ok := s.plannedCastCost(k, p, o, mode)
	if !ok {
		return false
	}
	if mode&2 != 0 && !s.plannedAfterLeaving(k, p, o, &cost) {
		return false
	}
	if ac := spellAddCost(k, s.objs[o].card); ac.Flags&kop.CostSac != 0 && k.sacMana {
		// a source that sacrifices itself can use up what the part must
		// sacrifice (CR 601.2g-h; gorge planReserving)
		var av avail
		s.availFor(k, p, &av)
		if !s.costPartDisjoint(k, p, NoSlot, ac, &cost, &av, pipMaskOf(&cost)) {
			return false
		}
	}
	if k.gen == nil || len(k.gen.grantPC) == 0 {
		return true // no granted mana sources: the kernel's own offer gate decided
	}
	return s.affordsUngrantedFor(k, p, &cost, NoSlot, payDesc{class: pdSpell, card: s.objs[o].card, src: NoSlot, og: s.restrOriginOfObj(o)})
}

// planDeclinesAddCost reports a spell gorge's planner never prices: rules
// payment_plan.go paymentPlanCastShapeDetailScoped admits only a fixed sacrifice
// or discard beside the mana (pay.PlanNonManaAdmissible), so any other part of
// the spell's own Cost$ (Fumarole's PayLife<3>, Fear of Exposure's tapXType,
// Abhorrent Oculus's exile) is "shape:additional_cost", offered by the pool only.
func planDeclinesAddCost(k *prep, c CardID) bool {
	ac := spellAddCost(k, c)
	return ac.Flags&^(kop.CostSac|kop.CostDiscard|kop.CostDiscardFiltered) != 0 || ac.ExileBf || sacXCost(ac) // an Exile<N/Spec> part is "cost:exile" (cost_exilebf.go); an announced Sac<X/Spec> is not "cost:sacrifice" admissible (PlanNonManaAdmissible; cost_sacx.go)
}

// plannedCastCost is the cost gorge's cast planner prices for p's cast of
// o in kernel cast mode (0 plain, bit 1 flashback); ok is false for the
// casts it never plans (FeatPlannedCast).
func (s *State) plannedCastCost(k *prep, p uint8, o Obj, mode uint8) (kop.ManaCost, bool) {
	return s.plannedCastCostOf(k, p, o, mode, false, false)
}

// plannedCastCostOf is plannedCastCost with two prices the potential planner
// reads where the cast planner declines. hyb prices a hybrid cost: the
// potential planner resolves its pips (planHybridCost), so a scripted play of
// a hybrid spell is priced on the pips as printed. composed prices the cast
// that the planner declines only because a mana-spent reader is out ("shape:
// mana_spent_reader": the card's own SpendRead or SpentSelf, or a Tf2CastSpent
// listener on a battlefield) from its composed cost, as gorge's potential
// planner does (potential_plan.go manaSpentReaderComposed): the reader changes
// no price, and the cast then takes its place among the plays whose prefix
// searches potentialScriptPlays caps.
func (s *State) plannedCastCostOf(k *prep, p uint8, o Obj, mode uint8, hyb, composed bool) (kop.ManaCost, bool) {
	c := s.objs[o].card
	refused := planNeverPricesCore(k, c, hyb, composed, false) || s.boardLifeTgtDep(k, p, c, o) || s.stackTgtDep(k, c)
	optRefused := k.gen != nil && k.gen.anyOptCost && optCostCard(k, c)
	spendRefused := !composed && k.gen != nil && k.gen.anyCastSpent && s.spendReaderOut(k)
	tapRefused := !refused && !optRefused && !spendRefused && s.contribPays(k, p, c, o, s.objs[o].zone == kop.ZoneHand)
	if refused || optRefused || spendRefused || tapRefused {
		// gorge's planner declines the shape (modal_cost, additional_cost,
		// optional_cost, mana_spent_reader, contribution, target_dependent_cost)
		// except for a cast payable only by paying with a permanent its own
		// announcements need (targetPaidPlanned)
		if mode&3 != 0 || planNeverPricesCore(k, c, hyb, composed, true) || !s.targetPaidPlanned(k, p, o) {
			return kop.ManaCost{}, false
		}
	}
	if s.fbBeholdUnplanned(k, p, o, mode) {
		return kop.ManaCost{}, false
	}
	cost := k.cards[c].cost
	if mode&2 != 0 {
		cost = s.flashbackCostOf(k, o)
	}
	if cost.Phyrexian() {
		// gorge pay.PlanManaHalf: the witness funds the cost without its
		// Phyrexian pips (each face is announced afterwards)
		cost.Tb, cost.Hyb = 0, [len(kop.ManaCost{}.Hyb)]kop.ColorMask{}
	}
	if cd := &k.t.Cards[c]; cd.HasAltAdd && cd.BeholdN != 0 {
		// gorge's planCastPaymentChecked plans the behold-or-mana shape only
		// for a hand or may-play cast (`!flashback`), so a flashback face is
		// not priced here (its shape stays "shape:optional_cost").
		if mode&2 != 0 {
			return kop.ManaCost{}, false
		}
		// A behold-or-mana additional cost is chosen at announcement (CR
		// 601.2b/h); gorge's offer/planner price the spell with the part it
		// will charge: the printed cost when the behold part is payable,
		// otherwise the part's mana on top (potentialAltAddPlan,
		// planCastAltAdd). The part joins the total before the reductions.
		if !s.altAddFreePart(k, p, c, o) {
			cost = addCost(cost, s.altAddManaPart(k, c))
		}
	}
	if s.statM.any() || k.gen != nil {
		cost = s.featCost(k, p&1, c, o, cost) // a flashback cost is reduced too
	}
	if k.gen != nil && (s.g.mayPlayAny || mode&castMayPlay != 0 && s.mayPlayAnyType(k, p, o) || s.convAnyType(k, p&1, o)) {
		cost = anyTypePips(cost) // gorge's planner funds these pips as generic (pay.PlanManaUnderRider)
	}
	return cost, true
}

// FeatFaceDown reports whether battlefield object o is face down.
func (s *State) FeatFaceDown(o Obj) bool {
	ob := &s.objs[o]
	return ob.zone == kop.ZoneBattlefield && s.faceDown(nil, ob.slot)
}

// FeatPlanDeclinesShape reports a plain cast of p's object o that gorge's cast
// planner declines for its cost's shape, not the seat's mana (rules
// payment_plan.go announcedShape, decision.PotentialShapeCasts): every
// plannedCastCost refusal, the card's own shape and the board's (a convoke,
// improvise or delve contribution, an optional cost, a mana-spent reader out).
func (s *State) FeatPlanDeclinesShape(p uint8, o Obj) bool {
	_, ok := s.plannedCastCost(s.k(), p, o, 0)
	return !ok
}

// planNeverPricesOf is plannedCastCost's refusals that read only the card, with
// a hybrid cost priced when hyb and
// the card's own mana-spent read left out when composed (plannedCastCostOf).
func planNeverPricesOf(k *prep, c CardID, hyb, composed bool) bool {
	return planNeverPricesCore(k, c, hyb, composed, false)
}

// planNeverPricesCore is planNeverPricesOf with the shapes gorge's planner
// admits for a cast payable only by paying with its own only target lifted when
// modal is set (targetPaidPlanned): a Spree or Tiered face, an either-or or
// life/energy additional cost, a mana-spent read, a target-dependent price.
func planNeverPricesCore(k *prep, c CardID, hyb, composed, modal bool) bool {
	cd := &k.t.Cards[c]
	if cd.BeholdExile || cd.SpellWaterbend != 0 || cd.Cost.X != 0 || cd.Cost.Hyb[0] != 0 && !hyb && !cd.Cost.Phyrexian() ||
		!modal && (cd.HasAltAdd && cd.BeholdN == 0 || cd.AltAddOne || cd.HasModeCost || !composed && (cd.SpendRead || cd.SpentSelf)) {
		return true // (a hybrid cost: the planner's "cost:hybrid"; a Phyrexian one is priced by its mana half; SpendRead and SpentSelf, the card's own face reading Count$CastTotalManaSpent: "shape:mana_spent_reader"; BeholdExile: the RaiseCost behold folds into the composed cost, PlanCostDetail "cost:behold")
	}
	if k.gen != nil && k.gen.anyEvTargets && spellEvidenceTargets(cd) {
		return true // gorge's planner declines a CollectEvidence part ("cost:evidence", classify.go): offered by the pool only (cost_collectevidencex.go)
	}
	if modal {
		// gorge's PlanTargetPaidManaHalf funds the mana without life and energy
		return spellAddCost(k, c).Flags&^(kop.CostSac|kop.CostDiscard|kop.CostDiscardFiltered|kop.CostPayLife|kop.CostPayEnergy) != 0 || spellAddCost(k, c).ExileBf || sacXCost(spellAddCost(k, c))
	}
	if planDeclinesAddCost(k, c) {
		// The spell's own Cost$ is separate from an AlternateAdditionalCost
		// either-or part (Mudbutton Cursetosser's Behold<1/Goblin>:2), which
		// the cast flow's altAddAsk pays after the witness: a face carrying
		// both still has gorge's standalone "shape:additional_cost" part
		// (PayLife, Exile, tapXType, ...), so the gate applies to every face.
		return true
	}
	for i := range cd.Abilities {
		if tgtDepCost(&cd.Abilities[i]) {
			return true
		}
	}
	return false
}

// FeatPlannedActivation reports whether gorge's priority list carries
// object o's activated ability ab (a printed index) as an option: its
// planner prices the mana without the mana abilities effects grant (rules/
// pay PaymentAbility's V1 exclusions, as FeatPlannedCast), the source's own
// mana left out when the ability taps it. Otherwise gorge reaches it only
// by a scripted potential play the oracle lowers, which its list omits.
func (s *State) FeatPlannedActivation(p uint8, o Obj, ab uint8) bool {
	k := s.k()
	if k.gen == nil || len(k.gen.grantPC) == 0 || ab&0x80 != 0 {
		return true
	}
	cost, ex, ok := s.activationMana(k, o, ab)
	if !ok {
		return true
	}
	d := payDesc{class: pdAct, card: s.objs[o].card, src: NoSlot}
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		d.card, d.src = s.cardAt(ob.slot), ob.slot
	}
	return s.affordsUngrantedFor(k, p, &cost, ex, d)
}

// activationMana is the mana cost, after its reductions, of object o's
// printed activated ability ab, the price the offer and the charge read, and
// the slot its {T} cost taps (NoSlot when none); ok is false when the ability
// has no mana cost.
func (s *State) activationMana(k *prep, o Obj, ab uint8) (cost kop.ManaCost, ex Slot, ok bool) {
	ob := &s.objs[o]
	card := ob.card
	if ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		card = s.cardAt(ob.slot)
	}
	// a graveyard ability too (Reassembling Skeleton's {1}{B} with the
	// other untapped Swamp under Imprisoned in the Moon: no plan)
	abs := k.t.Cards[card].Abilities
	if int(ab) >= len(abs) {
		return kop.ManaCost{}, NoSlot, false
	}
	a := &abs[ab]
	cost = a.Cost.Mana
	if cost == (kop.ManaCost{}) {
		return cost, NoSlot, false
	}
	ex = NoSlot
	if a.Cost.Flags&kop.CostTap != 0 && ob.zone == kop.ZoneBattlefield {
		ex = ob.slot
	}
	if k.gen != nil && (k.gen.anyOwnReduce && a.OwnReduce != (kop.ValueRef{}) || k.gen.anyAbReduce) {
		cost = s.activationReduced(k, a, o)
	}
	return cost, ex, true
}

// activationReduced is ability a's mana cost for source o after its own
// ReduceCost$ and the reductions statics apply to it.
//
//go:noinline
func (s *State) activationReduced(k *prep, a *kop.Ability, o Obj) kop.ManaCost {
	ob := &s.objs[o]
	ctrl := ob.owner()
	if ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		ctrl = s.perms[ob.slot].ctrl
	}
	cost := a.Cost.Mana
	if k.gen.anyOwnReduce && a.OwnReduce != (kop.ValueRef{}) {
		cost = s.ownReduced(k, a, o, ctrl)
	}
	if k.gen.anyAbReduce && ob.zone == kop.ZoneBattlefield {
		cost = s.abReduced(k, a, ob.slot, ctrl, cost)
	}
	return cost
}

// affordsUngrantedFor: p's pool and untapped printed mana sources (not ex) afford
// cost, for the payment d describes: the restricted sources it may spend count
// too, as does the floating restricted mana that payment may spend (gorge's
// planner pool for it is AvailableFor, which holds the batches RestrictValidMatches
// admits; pdNone: none).
func (s *State) affordsUngrantedFor(k *prep, p uint8, cost *kop.ManaCost, ex Slot, d payDesc) bool {
	var av avail
	var ms modalSet
	s.ungrAvail(k, p, ex, &av, &ms)
	rm := s.restrFloatAdmits(k, p&1, &d)
	// restrSrcNone precondition: restrSrcAffords runs only once av.affords failed (||).
	for c := 0; c < 1<<ms.n; c++ {
		b := ms.modes(av, c)
		if b.affords(cost, pipMaskOf(cost), 0) || k.restrSrc && d.class != pdNone && s.restrSrcAffords(k, p&1, d, cost, &b, ex, 0) {
			return true
		}
		if rm != 0 {
			t := b
			s.restrAvailAdd(p&1, rm, &t)
			if t.affords(cost, pipMaskOf(cost), 0) {
				return true
			}
			// the floating restricted batches and the restricted sources
			// pay together, as the cast window counts them (castAffordsR):
			// Helga's floating creature mana beside Oakhollow Village's
			// creature-only {G} (CR 106.6)
			if k.restrSrc && d.class != pdNone && s.restrSrcAffords(k, p&1, d, cost, &t, ex, 0) {
				return true
			}
		}
	}
	return false
}

// ungrAvail fills av (and ms) with p's pool and untapped printed mana sources
// other than ex: the planner's census, without the mana abilities effects
// grant.
func (s *State) ungrAvail(k *prep, p uint8, ex Slot, av *avail, ms *modalSet) {
	pl := &s.pl[SeatIx(p)]
	for i := 0; i < kop.NumManaKinds; i++ {
		if pl.pool[i] > 0 {
			av.addUnits(1<<i, pl.pool[i])
		}
	}
	defer_ := s.deferLive(k)
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		pm := &s.perms[sl]
		if sl == ex || pm.flags&pfTapped != 0 || s.grantMay(k, sl) && s.manaGranted(k, sl) {
			continue
		}
		if defer_ && s.planDeferred(k, p, sl) {
			continue // the census never lists a deferred source (mana_rider.go)
		}
		pc := s.manaPCGen(k, sl)
		if pc.manaAb < 0 && pm.ovr.flags&ovCopy == 0 || !s.canTapForManaPC(k, sl, pc) {
			continue
		}
		if k.painMana && pc.painCol != 0 {
			s.addPainAvail(k, sl, pc, p&1, av, ms, false) // the planner's census has a pain ability (mana_pain.go)
			continue
		}
		av.addUnits(s.srcMaskAt(pc, sl), s.srcYieldAt(k, sl, pc))
		if pc.manaOne {
			s.addOneColour(av, pc.manaN)
		}
		if k.gen.anyTapExtra {
			xv := s.srcExtra(k, sl, pc) // the planner prices a tap's extra too
			for c := range xv {
				if xv[c] > 0 {
					av.addUnits(1<<c, xv[c])
				}
			}
		}
	}
}

// affordsUngrantedPlay is affordsUngrantedFor for play a: an activated
// ability whose cost also sacrifices or taps permanents is priced only when
// the printed plan leaves those parts payable (gorge planComposedCost's
// planLeavesCostPayable). A plan that spends the ability's own sacrifice
// candidates is no plan: the granted mana is a prefix activated first (CR
// 601.2g-h), and the printed plan then pays with the candidates still there.
func (s *State) affordsUngrantedPlay(k *prep, p uint8, a Action, cost *kop.ManaCost, ex Slot, d payDesc) bool {
	if a.Kind() != AkActivate || k.t == nil {
		if k.t != nil && s.fixedTapsCast(k, a) {
			// the composed cost's tap part keeps its candidates apart (cost_fixedtaps.go)
			return s.affordsUngrantedReserving(k, p, cost, ex, d, spellAddCost(k, s.objs[a.Source()].card), NoSlot)
		}
		return s.affordsUngrantedFor(k, p, cost, ex, d)
	}
	o := Obj(a.Source())
	card := s.objs[o].card
	src := NoSlot // the ability's source, for a sacrifice filter that reads it (Aura.Attached)
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		card = s.cardAt(ob.slot)
		src = ob.slot
	}
	ab := int(a.Mode())
	if ab < 0 || ab >= len(k.t.Cards[card].Abilities) {
		return s.affordsUngrantedFor(k, p, cost, ex, d)
	}
	ca := &k.t.Cards[card].Abilities[ab]
	f := ca.Cost.Flags
	if f&(kop.CostTapX|kop.CostSac) == 0 || f&kop.CostSac != 0 && !k.sacMana {
		return s.affordsUngrantedFor(k, p, cost, ex, d)
	}
	return s.affordsUngrantedReserving(k, p, cost, ex, d, &ca.Cost, src)
}

// manaGranted reports whether slot sl's mana ability is one an effect
// grants it (manaPCGen's granted case).
func (s *State) manaGranted(k *prep, sl Slot) bool {
	g := k.gen
	if g == nil || len(g.grantPC) == 0 || !s.grantMay(k, sl) {
		return false
	}
	return s.grantIdx(k, sl) >= 0 && !s.ownManaUsable(k, sl)
}

// FeatTargetGroup reports whether the pending target decision's spec needs
// its targets controlled by different players (gorge labels each such
// target option with its controller's Group).
func (s *State) FeatTargetGroup() bool {
	if s.ctl != ctlBuild || s.bld().stage != bsTargets || int(s.bld().ti) >= kop.MaxTargets {
		return false
	}
	e := &s.bld().e
	t := tspecE(s.k(), e, int(s.bld().ti))
	return t != nil && t.TFlags&kop.TgfDiffController != 0
}

// FeatDerivedLive reports whether some object's derived characteristics
// can depend on other objects: a static ability on the battlefield, an
// emblem, an attached permanent, or a characteristic-defining P/T or a mana
// ability that taps other permanents in the table. When
// it is false, an object whose own fields are unchanged has its derived
// P/T and keywords unchanged too.
func (s *State) FeatDerivedLive() bool {
	if s.statM.any() || s.k().cda || s.k().tapMana || s.g.nemb[0]+s.g.nemb[1] != 0 {
		return true
	}
	for m := s.occ; m.any(); m = m.rest() {
		if s.perms[m.first()].attached != NoSlot {
			return true
		}
	}
	return false
}

// FeatZoneList appends player p's hand, graveyard or exile objects in zone
// order to dst. A token there is left out: it has ceased to exist (CR
// 111.7) and gorge's view lists no Ephemeral object, though the zone's
// count holds it until the state-based check (a token sacrificed to
// Liliana, Dreadhorde General's -4 while the next player chooses).
func (s *State) FeatZoneList(p uint8, z kop.Zone, dst []Obj) []Obj {
	pl := &s.pl[SeatIx(p)]
	var src []Obj
	switch z {
	case kop.ZoneHand:
		src = pl.hand[:pl.handN]
	case kop.ZoneGraveyard:
		src = pl.grave[:pl.graveN]
	case kop.ZoneExile:
		src = pl.exile[:pl.exileN]
	}
	for _, o := range src {
		// ofToken, not the card's type: a token copy of a card (Strix
		// Lookout copied) carries the copied card's id.
		if s.objs[o].of&ofToken != 0 {
			continue
		}
		dst = append(dst, o)
	}
	return dst
}

// FeatZoneIndex is object o's position in its owner's hand, library (from
// the top), graveyard or exile; 255 elsewhere.
func (s *State) FeatZoneIndex(o Obj) uint8 { return s.zoneIndex(o) }

// featCost is card c's cast cost for p after reductions, exactly as the
// kernel's priority walk prices it (adjustCost).
func (s *State) featCost(k *prep, p uint8, c CardID, o Obj, base kop.ManaCost) kop.ManaCost {
	return s.adjustCost(k, p, c, o, base)
}

// FeatPayManual reports whether gorge's pending trigger-cost ask (ChGenPay)
// also offers its manual route ("trigger_cost_pay_manual", after decline):
// a payable mana cost the floating pool does not cover yet.
func (s *State) FeatPayManual() bool {
	if s.ctl != ctlResolve || s.res.ask != raGenPay || s.g.pickF&(pfNoPay|pfWard|pfModes) != 0 || s.g.pickZ != kop.PayMana {
		return false
	}
	k := s.k()
	in := &s.prog(k)[s.res.pc]
	c := manaOf(int(s.g.pickLeft), s.numTimesPips(k, in.B.N))
	var av avail
	pl := &s.pl[SeatIx(s.g.pickP)]
	for i := 0; i < kop.NumManaKinds; i++ {
		if pl.pool[i] > 0 {
			av.addUnits(1<<i, pl.pool[i])
		}
	}
	return !av.affords(&c, pipMaskOf(&c), 0)
}

// FeatManaGranted reports whether battlefield object o's mana ability is one
// an effect grants it (Imprisoned in the Moon's {T}: Add {C}).
func (s *State) FeatManaGranted(o Obj) bool {
	ob := &s.objs[o]
	k := s.k()
	return ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot && k.gen != nil && s.manaGranted(k, ob.slot)
}

// FeatViewTypes is battlefield object o's derived type line as gorge's view
// shows it (gorge rules.Engine.ViewTypes, develop dca0c4384: the layer-4
// table's entry, one for an object whose derived type words differ from its
// face): ok when its card types, supertypes or table subtypes differ from
// the printed ones, or an effect set its subtypes. An all-creature-types
// grant changes the line only with a card-type change (a printed changeling
// keeps its spelling). types is its type mask, subs its table-subtype mask,
// all whether it has every creature type, setSub whether an effect replaced
// its subtypes (the printed subtypes outside the table are then gone too).
func (s *State) FeatViewTypes(o Obj) (types kop.TypeMask, subs kop.SubMask, all, setSub, ok bool) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return 0, kop.SubMask{}, false, false, false
	}
	const cardTypes = kop.TLand | kop.TCreature | kop.TArtifact | kop.TEnchantment | kop.TPlaneswalker |
		kop.TInstant | kop.TSorcery | kop.TBattle | kop.TKindred
	const lineTypes = cardTypes | kop.TLegendary | kop.TBasic
	k := s.k()
	sl := ob.slot
	c := s.cardAt(sl)
	types = s.typesAt(k, sl)
	cardChange := (types^k.cards[c].types)&cardTypes != 0
	printed := k.t.Cards[c].Subtypes
	subs = printed
	if k.gen != nil {
		if s.fxM.has(sl) && k.gen.cpxSub {
			subs = subs.Or(s.copyExcSubs(k, sl)) // layer 1, before the overrides (hasSubGen)
		}
		s.eachOverride(k, sl, func(ov *kop.Override, _ CardID) {
			if ov.HasSub {
				subs, all, setSub = subs.And(ov.KeepSub).Or(ov.SetSub), false, true
			}
			if ov.RemoveCreatureSub && !DebugRmCreatSubKeep {
				subs, all, setSub = subs.AndNot(k.gen.creatSub), false, true
			}
			subs = subs.Or(ov.AddSub)
			if ovrAllCreat(ov) {
				all = true
			}
		})
		// hasSubGen per subtype, in one walk: "all creature types" has
		// every one, otherwise a static subtype grant (Skyknight Squire)
		// adds its subtypes
		if all {
			for i := 0; i < len(k.t.Subtypes) && i < kop.MaxSubtypes; i++ {
				subs = subs.With(i)
			}
		} else if s.statM.any() || s.g.nemb[0]+s.g.nemb[1] != 0 {
			s.eachStaticC(k, sclsSub, func(st statSrc) bool {
				a := st.a
				if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous {
					return true
				}
				add := a.Static.AddSub
				if k.gen.anyAddChosen {
					add = s.chosenGrantMask(k, a, st.sl, add) // act_choosetype.go
				}
				if st.sl == sl {
					keep, _ := s.selfGrantKept(k, sl, st.sl) // static_selfgrant.go
					add = add.And(keep)
				}
				if add == (kop.SubMask{}) || subs.Or(add) == subs {
					return true
				}
				if s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerSub) {
					subs = subs.Or(add)
				}
				return true
			})
		}
	}
	if k.gen != nil && k.gen.anyImpending && s.impendDormant(k, sl) {
		subs, all = subs.AndNot(k.gen.creatSub), false // CR 205.1a: gone with the creature type
	}
	if all && !cardChange {
		return 0, kop.SubMask{}, false, false, false
	}
	if !cardChange && (types^k.cards[c].types)&lineTypes == 0 && subs == printed && !setSub && !all {
		return 0, kop.SubMask{}, false, false, false
	}
	return types, subs, all, setSub, true
}

// FeatStaticKey fingerprints what the statics in play read beyond the
// objects they apply to: each static's condition truth, its dynamic
// amounts and dynamic filter bounds (evaluated as its source would), the
// emblems, the characteristic-defining P/T of every card outside the
// libraries, and which permanents can pay a mana ability that taps others.
func (s *State) FeatStaticKey() uint64 {
	k := s.k()
	if k.gen == nil {
		return 0 // the vanilla statics read nothing beyond their objects
	}
	h := uint64(0x7374617469636b79)
	h = hmix(h, uint64(s.g.nemb[0])|uint64(s.g.nemb[1])<<8)
	for p := uint8(0); p < s.nSeats(); p++ { // every seat, a departed one included: a key
		for i := 0; i < int(s.g.nemb[p]); i++ {
			h = hmix(h, uint64(s.embIdx(p, i)))
			h = hmix(h, uint64(s.embNum(p, i)))   // the frozen chosen number
			h = hmix(h, uint64(s.embStamp(p, i))) // the layer timestamp
		}
	}
	if s.statM.any() || s.g.nemb[0]+s.g.nemb[1] != 0 {
		s.eachStatic(k, func(st statSrc) bool {
			a := st.a
			if a.Kind != kop.AbStatic {
				return true
			}
			sa := &a.Static
			if sa.Cond2 != 0 {
				c := uint64(1)
				if s.statCond(k, st) {
					c = 2
				}
				h = hmix(h, c)
			}
			if sa.StFlags&kop.StDynamic != 0 {
				ctx := s.ctxStatic(st)
				h = hmix(h, uint64(uint32(s.evalV(k, sa.VA, ctx)))|uint64(uint32(s.evalV(k, sa.VB, ctx)))<<32)
			}
			for f := sa.Filter; f != 0 && int(f) < len(k.t.Filters); f = k.t.Filters[f].Or {
				if fl := &k.t.Filters[f]; featPredVDynamic(fl.Pred) {
					h = hmix(h, uint64(uint32(s.evalV(k, fl.PredV, s.ctxCur()))))
				}
			}
			return true
		})
	}
	if k.cda {
		for o := 0; o < int(s.nobj); o++ {
			ob := &s.objs[o]
			if ob.zone == kop.ZoneLibrary || ob.zone == kop.ZoneNone || !k.gen.has(ob.card, gfCDAPT) {
				continue
			}
			var p, t int
			if ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
				p, t = s.basePTGen(k, ob.slot)
			} else {
				p, t, _ = s.FeatCDAPT(Obj(o))
			}
			h = hmix(h, uint64(o)|uint64(uint32(p))<<8|uint64(uint32(t))<<40)
		}
	}
	if k.tapMana {
		for m := s.occ; m.any(); m = m.rest() {
			if sl := m.first(); s.featManaActivatable(k, sl) {
				h = hmix(h, uint64(sl))
			}
		}
	}
	return h
}

// featPredVDynamic: the predicate tests a value read from the game
// (Filter.PredV), so the filter's current value enters the view hash.
func featPredVDynamic(p kop.Pred) bool {
	return p == kop.PredCMCLEV || p == kop.PredPowerGEV || p == kop.PredToughnessLEV
}

// featFilterDynamic reports whether filter f (or an Or alternative) tests
// a value read from the game (a mana value against an expression).
func featFilterDynamic(k *prep, f kop.FilterID) bool {
	for f != 0 && int(f) < len(k.t.Filters) {
		fl := &k.t.Filters[f]
		if featPredVDynamic(fl.Pred) {
			return true
		}
		f = fl.Or
	}
	return false
}

// FeatObjHash is object o's content hash (ObjHash) read as ViewHashCached
// reads it: from base (ObjHash of every object at the epoch, nil for none)
// when the object is unmarked, from c when computed on this line of play
// and unwritten since, else computed (and kept in c along a line).
func (s *State) FeatObjHash(o Obj, base *[MaxObj]uint64, c *ObjHashCache) uint64 {
	if base != nil && s.chg[o>>6]&(1<<(o&63)) == 0 {
		return base[o]
	}
	if c == nil || s.line == 0 {
		return s.ObjHash(o)
	}
	if c.line != s.line {
		c.line = s.line
		c.ok = [MaxObj]bool{}
		c.part = [2]objPart{}
	}
	if c.ok[o] && !s.ChangedSince(o, c.at[o]) {
		return c.h[o]
	}
	h := s.ObjHash(o)
	c.ok[o], c.at[o], c.h[o] = true, s.touchN, h
	return h
}

// FeatDerivedContext keys what an object's derived characteristics read
// beyond its own fields: 0 when nothing can (FeatDerivedLive false), else
// FeatStaticKey mixed with featCtxPerm of every battlefield permanent whose
// card has static abilities or that is attached to another (base and c are
// unread since featCtxPerm replaced the sources' whole content hashes: a
// static source tapping, taking damage or gaining a counter no longer
// changes every view's key). Derived characteristics read the object's own
// fields, the static abilities in play (their sources; their filters on
// the object's own fields; their conditions, dynamic amounts and emblems,
// which FeatStaticKey fingerprints), the characteristic-defining P/T of
// cards (fingerprinted too) and the object's attachments and override
// records: two States with the same context and an object hashing the
// same give it the same derived P/T, keywords, types and name.
func (s *State) FeatDerivedContext(base *[MaxObj]uint64, c *ObjHashCache) uint64 {
	if !s.FeatDerivedLive() {
		return 0
	}
	k := s.k()
	h := hmix(0x64637478, s.FeatStaticKey())
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		pm := &s.perms[sl]
		if pm.attached != NoSlot || k.cards[s.objs[pm.obj].card].hasStat || k.cards[s.cardAt(sl)].hasStat {
			h = hmix(h, s.featCtxPerm(sl))
		}
	}
	return h | 1
}

// featCtxPerm hashes what another object's derived characteristics read
// of the static source or attached permanent at sl: its object, card and
// owner, slot, controller, layer timestamp, attachment, "as this enters"
// choice, override records and override-derived types, and its
// until-end-of-turn control and animation. A static's condition truth,
// dynamic amounts and dynamic filter bounds are FeatStaticKey's. Its own
// tapped, sick and combat state, damage, counters, pumps and keyword grants
// are left out: a static reads them only through those (and the object's
// own view key holds them).
func (s *State) featCtxPerm(sl Slot) uint64 {
	p := &s.perms[sl]
	ob := &s.objs[p.obj]
	h := hmix(uint64(p.obj)<<56|uint64(sl)<<48|uint64(p.card)<<40|uint64(p.ctrl)<<32|uint64(p.attached)<<24|
		uint64(p.chosen)<<16|uint64(p.ts),
		uint64(ob.card)|uint64(ob.owner())<<8|uint64(p.gty)<<16|uint64(p.flags&(pfCtrlEOT|pfAnimEOT))<<32)
	h = hbytes(h, unsafe.Pointer(&p.ovr), unsafe.Sizeof(p.ovr))
	if s.hasOvRec(sl) || s.fxM.has(sl) && s.copyExcHash(sl) != 0 {
		r := s.ovRecs(sl)
		h = hbytes(h, unsafe.Pointer(&r[0]), unsafe.Sizeof(r[0])*4)
		if r[4].idx != 0 {
			h = hbytes(h, unsafe.Pointer(&r[4]), unsafe.Sizeof(r[0])*4)
		}
		h = hmix(h, s.copyExcHash(sl))
	}
	return h
}

// FeatProjObj writes object o's projection into po (Project's, but for o
// alone and without its zone position: FeatZoneIndex).
func (s *State) FeatProjObj(o Obj, po *ProjObj) { s.projObj(s.k(), o, po) }

// FeatProjGlobals writes the projection's non-object fields into p (the
// object count, life totals, zone sizes, pools, the stack, the turn and
// step, the active and priority players), leaving p.Objs as it is.
func (s *State) FeatProjGlobals(p *Projection) {
	p.NObj = s.nobj
	for pi := 0; pi < int(s.nSeats()); pi++ { // every seat, a departed one included, as projRest
		pl := &s.pl[pi]
		p.Life[pi] = pl.life
		p.Lands[pi] = s.landsProj(uint8(pi))
		p.Pool[pi] = s.poolProj(uint8(pi))
		p.LibN[pi] = uint8(pl.libLen())
		p.HandN[pi] = pl.handN
		p.GraveN[pi] = pl.graveN
		p.ExileN[pi] = pl.exileN
	}
	p.NStack = s.sp
	for i := 0; i < int(s.sp); i++ {
		e := &s.stack[i]
		p.Stack[i] = ProjStack{Kind: e.kind, Source: e.obj, Card: e.card, Ability: e.ab, Ctrl: e.ctrl, Targets: e.tgt, Mode: e.mode, X: e.x}
		if e.x != 0 && e.kind == seTrigger {
			p.Stack[i].X = projTrigX(s.k(), e) // a bound X is not the object's own (trig_xpaid.go)
		}
		if e.ab == abDelayed {
			p.Stack[i].Ability = 255
		}
		if e.xi != 0 {
			x := s.xtOf(e)
			p.Stack[i].NExtra = x.n
			p.Stack[i].Extra = x.code
			p.Stack[i].ExtraSlot = x.slot
		}
	}
	p.Turn, p.Step, p.Active, p.Prio = s.turn, s.step, s.active, s.prio
	if s.turn == 0 && !s.over {
		// gorge's Game.Active is the starting player during the pregame (the
		// mulligans and opening-hand asks; CR 103.4-103.6 run from the
		// starting player)
		p.Active = s.startPl
	}
	p.Draws = s.rng.Draws
	p.Over, p.Winner = s.over, s.win
}

// FeatObjLinks is object o's card as its projection names it (a
// permanent's: its slot's, an override or copy can change it), and on the
// battlefield the objects it is attached to and blocks (NoObj none) --
// the projection's Card, Attached and Blocks without its derived P/T.
func (s *State) FeatObjLinks(o Obj) (card CardID, attached, blocks Obj) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return ob.card, NoObj, NoObj
	}
	pm := &s.perms[ob.slot]
	attached, blocks = NoObj, NoObj
	if pm.attached != NoSlot {
		attached = s.perms[pm.attached].obj
	} else if s.g.lingerM.has(ob.slot) {
		// its host left mid-resolution: gorge's view still shows it
		// attached until the next state-based check (Project's rule)
		attached = s.lingerHost(ob.slot)
	}
	if pm.flags&pfBlocking != 0 {
		if a := s.blockProjected(ob.slot); a != NoSlot {
			blocks = s.perms[a].obj
		}
	}
	return s.cardAt(ob.slot), attached, blocks
}

// FeatManaActivatable reports whether battlefield object o is one of the
// sources FeatManaSources counts (a pure function of its own fields and
// the derived context, FeatDerivedContext).
func (s *State) FeatManaActivatable(o Obj) bool {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return false
	}
	return s.featManaActivatable(s.k(), ob.slot)
}

// FeatTypeKey orders creature-type option v (gorge's vocabulary index, as
// the ChGenType option carries it) the way gorge lists them
// (rules/cast_etbchoice.go creatureTypeOptions): the printed types of the
// creature cards player p owns first, by name (a changeling's printed ones,
// not every type it has), then the rest in vocabulary order.
func (s *State) FeatTypeKey(p uint8, v int) uint32 {
	k := s.k()
	t := k.t
	if v >= len(t.TypeVocab) {
		return 1<<16 | uint32(v)
	}
	var own kop.SubMask
	for i := 0; i < int(s.nobj); i++ {
		if ob := &s.objs[i]; ob.owner() == p {
			if cd := &t.Cards[ob.card]; cd.Types&kop.TCreature != 0 {
				if cd.AllCreatureTypes {
					own = own.Or(cd.PrintedSubtypes) // the printed types, as gorge's Face().Types
				} else {
					own = own.Or(cd.Subtypes)
				}
			}
		}
	}
	si := t.TypeVocab[v]
	if si == 255 || !own.Has(int(si)) {
		return 1<<16 | uint32(v)
	}
	rank := uint32(0)
	for j := range t.Subtypes {
		if own.Has(j) && t.Subtypes[j] < t.Subtypes[si] {
			rank++
		}
	}
	return rank
}

// CastFreeIndex is free-cast candidate o's index in gorge's modes list: the
// cards the OpCastFree pose accepted (still in the zone, a nonland, CR
// 601.2c feasible), frozen at the pose — the ones already cast keeping
// their places (gorge does not re-index after a cast: Etali).
//
// gorge's KModes option Index is the candidate's POSITION in the list it
// scanned (a remembered set's remember order, a zone's insertion order),
// not the object-id rank. The kernel fills freeAll in that same scan order,
// so the position is the canonical index. (the P2/P3/P4
// populations are not object-id sorted, and a rank-by-id index picked the
// wrong card there.)
func (s *State) CastFreeIndex(o Obj) uint8 {
	if i, ok := s.freeRepeatIndex(o); ok {
		return i // AllowRepeats$: the lowest place of o not yet picked (cast_freerepeat.go)
	}
	for i := 0; i < int(s.g.nfreeAll); i++ {
		if s.g.freeAll[i] == o {
			return uint8(i)
		}
	}
	// Not in the frozen list (should not happen): fall back to the rank so
	// the answer is still deterministic.
	n := uint8(0)
	for i := 0; i < int(s.g.nfreeAll); i++ {
		if s.g.freeAll[i] < o {
			n++
		}
	}
	return n
}

// flashbackCostOf is object o's flashback cast's mana cost: the card's own
// flashback cost, or its mana cost when the flashback is granted (Sphinx of
// Forgotten Lore), printed beam me up or not.
func (s *State) flashbackCostOf(k *prep, o Obj) kop.ManaCost {
	cd := &k.t.Cards[s.objs[o].card]
	if cd.Keywords&kop.KwFlashback != 0 || cd.Harmonize || cd.Mayhem || cd.BeamMeUp && !s.fbGranted(k, o) {
		return cd.AltCost
	}
	if cd.Retrace && !s.fbGranted(k, o) && !s.fbLive(k, o) {
		return cd.Cost // the retrace cast's printed cost (kw_retrace.go; FeatPoolPays mode 2)
	}
	return s.fbGrantCost(k, s.objs[o].owner(), o, s.fbGranted(k, o)) // the mana cost, or a live grant's own (kw_fbgrant.go)
}

// FeatGrantedOff is the keywords an object off the battlefield has been
// granted: a graveyard card's flashback until end of turn (Sphinx of
// Forgotten Lore; gorge lists it after the printed keywords).
func (s *State) FeatGrantedOff(o Obj) kop.KwMask {
	g, _ := s.FeatGrantedOffRep(o)
	return g
}

// FeatGrantedOffRep is FeatGrantedOff with the repeats (FeatKwReps): a
// graveyard card granted flashback twice or three times (Sphinx of
// Forgotten Lore, one a Self-Reflection copy) is listed once per grant,
// as gorge lists it.
func (s *State) FeatGrantedOffRep(o Obj) (kop.KwMask, FeatKwReps) {
	var rep FeatKwReps
	if !s.g.on {
		return 0, rep
	}
	k := s.k()
	n := s.fbGrants(k, o)
	if s.fbLive(k, o) {
		n++ // a live static's grant (kw_fbgrant.go), listed after the stamped ones
	}
	if n == 0 {
		return 0, rep
	}
	if n > 1 {
		bit := bits.TrailingZeros32(uint32(kop.KwFlashback))
		rep[bit] = n - 1
	}
	return kop.KwFlashback, rep
}

// seeCtr records the counter kinds p holds now (perm.cseen): called as
// a counter is removed, so a kind removed to none stays listed, as gorge's
// counter list keeps it at 0 while the permanent stays.
func (p *perm) seeCtr(s *State) {
	for i := range p.cn {
		if p.cn[i] != 0 && p.ck[i] < uint8(kop.NumCounterKinds2) { // CtrLevel is no counter (kw_class.go)
			p.cseen |= 1 << i
		}
	}
	if p.p1 != 0 {
		p.cseen |= cseenP1
	}
	if p.ck[ctrSlots-1] != 0 {
		s.ctrOverSee(p.obj)
	}
}

// FeatCtrDeathtouched is the counter index of gorge's deathtouch mark
// beside the kernel's counter kinds: gorge records CR 702.2b's mark as a
// "Deathtouched" counter (state.InternalCounterMarker), which its view
// lists with the counters -- 1 while the permanent is marked (pfDeathtouched),
// and at 0 once the cleanup or a regeneration removes it, for as long as the
// permanent stays (perm.cseen bit cseenDT). FeatCtrKinds
// sizes a counter array that holds it.
const (
	FeatCtrDeathtouched = int(kop.NumCounterKinds2)
	FeatCtrKinds        = FeatCtrDeathtouched + 1
)

// ctrSeenMask is a uint16 over the counter kinds and the mark.
var _ = [16 - FeatCtrKinds]struct{}{}

// FeatDeathtouchMark is gorge's "Deathtouched" counter on object o: one
// per deathtouch damage event since the cleanup (the kernel keeps two:
// pfDeathtouched, pfDtMore).
func (s *State) FeatDeathtouchMark(o Obj) uint8 {
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot && s.perms[ob.slot].flags&pfDeathtouched != 0 {
		if s.perms[ob.slot].flags&pfDtMore != 0 {
			return 2
		}
		return 1
	}
	return 0
}

// FeatCountersListed is object o's counter kinds gorge's counter list
// holds: on the battlefield, every kind it holds or held since it entered
// (a removed kind stays listed at 0) and the deathtouch mark
// (FeatCtrDeathtouched); elsewhere, the kinds it holds.
func (s *State) FeatCountersListed(o Obj, ctr *[kop.NumCounterKinds2]uint16) uint16 {
	var m uint16
	for c, n := range ctr {
		if n != 0 {
			m |= 1 << c
		}
	}
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		p := &s.perms[ob.slot]
		m |= p.ctrSeenMask(s)
		if p.flags&pfDeathtouched != 0 {
			m |= 1 << FeatCtrDeathtouched
		}
	}
	return m
}

// FeatAbilityPoolPays reports whether p's floating pool alone pays the
// mana part of object o's activated ability ab (a printed index; a granted
// one, 0x80|index, names the granting card's ability, at host): gorge's
// priority list carries an ability whose mana it would otherwise have to
// tap for only as a potential play the oracle lowers, not as an option.
func (s *State) FeatAbilityPoolPays(p uint8, o Obj, ab uint8, host Obj) bool {
	k := s.k()
	c := s.objs[o].card
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		c = s.cardAt(ob.slot)
	}
	if ab&0x80 != 0 {
		// a granted ability: its cost is the granting card's (the action's
		// target), not the source's
		if host == NoObj || s.objs[host].zone != kop.ZoneBattlefield || s.objs[host].slot == NoSlot {
			return false
		}
		ab &^= 0x80
		c = s.grantCardAt(k, s.objs[host].slot, ab)
	}
	abs := k.t.Cards[c].Abilities
	if int(ab) >= len(abs) {
		return false
	}
	cost := abs[ab].Cost.Mana
	return s.poolPays(p, &cost)
}

// FeatCastMayPlay is the AkCast mode bit of a cast through a play
// permission or a play-from-zone static.
const FeatCastMayPlay = castMayPlay

// FeatCastFree is the AkCast mode bit of a cast whose mana cost a
// permission waives (CR 118.9's alternative cost).
const FeatCastFree = castFree

// FeatCastPlotted is the AkCast mode bit of a plotted card's free cast from
// exile (CR 702.170b; kw_plot.go).
const FeatCastPlotted = castPlotted

// FeatCastWarp is the AkCast mode bit of a hand cast for its warp cost
// (CR 702.185a; dirac/kw_warp.go).
const FeatCastWarp = castWarp

// FeatCastWarpRecast is the AkCast mode bit of a warp-exiled card's cast
// from exile (CR 702.185a; dirac/kw_warp.go).
const FeatCastWarpRecast = castWarpRecast

// FeatCastFaceDown is the AkCast mode bit of a face-down morph-family cast
// (CR 702.37/702.168/702.169). The family is the card's MorphFamily.
const FeatCastFaceDown = castFaceDown

// FeatFaceDownFamily is o's printed face-down keyword family (kop.Morph*;
// 0 when the card has none), used to canonicalise a face-down cast.
func (s *State) FeatFaceDownFamily(o Obj) uint8 { return s.k().t.Cards[s.ObjCard(o)].MorphFamily }

// FeatMayPlayUntyped reports whether p may cast object o through an
// untyped permission -- a play permission record (an impulse draw's "you
// may play it this turn") or a play-from-zone static without a once-per-
// turn budget (Tinybones' stash) -- which gorge's cast planner prices as a
// may-play cast; a cast only a typed permission allows (Muldrotha's
// once-per-turn per-type statics, gorge's MayPlayPerm) it reaches by
// lowering a potential play instead.
func (s *State) FeatMayPlayUntyped(p uint8, o Obj) bool {
	ob := &s.objs[o]
	k := s.k()
	for i := 0; i < int(s.g.nmay); i++ {
		if r := *s.mayAt(i); r.obj == o && r.ctrl == p && r.ver == ob.ver && !ownWalkerRecord(r) {
			if k.gen != nil && k.gen.anyMayLimit && s.mayLimitSpent(r) {
				continue // the record's cap is reached; another grant may still offer
			}
			if k.gen != nil && k.gen.anyMayWhileSrc && s.maySrcDead(r) {
				continue // its source left the battlefield (cast_whilesource.go)
			}
			return true
		}
	}
	untyped := false
	if k.gen != nil {
		s.mayPlayStatics(k, p, o, func(st statSrc) bool {
			// Quilled Greatwurm's "by removing six counters" is an additional
			// cost gorge's cast planner does not price (no payment action)
			if st.a.Static.StFlags&(kop.StOncePerTurn|kop.StRemove6Counters) == 0 {
				untyped = true
				return false
			}
			return true
		})
	}
	return untyped
}

// FeatMayPlayTyped names the typed permission p may cast object o through when no
// untyped one allows it: the card holding the static, and how many play-from-zone
// statics come before it on that card.
func (s *State) FeatMayPlayTyped(p uint8, o Obj) (c CardID, nth int, ok bool) {
	k := s.k()
	if k.gen == nil || s.FeatMayPlayUntyped(p, o) {
		return 0, 0, false
	}
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		if st.sl == NoSlot {
			return true
		}
		c, ok = s.cardAt(st.sl), true
		for i := range k.t.Cards[c].Abilities {
			a := &k.t.Cards[c].Abilities[i]
			if a == st.a {
				break
			}
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom {
				nth++
			}
		}
		return false
	})
	return c, nth, ok
}

// FeatPayBlight reports whether the pending pay pick is a Blight cost
// (CR 701.68) and whether it is an unless-style cost (gorge's blightcost)
// rather than a trigger's own cost (trigger_cost_blight).
func (s *State) FeatPayBlight() (blight, modes bool) {
	return s.g.pickZ == payPickBlight, s.g.pickF&pfModes != 0
}

// FeatSubCtrWildcard reports whether the pending ChCostSubCtr pick is a
// RemoveAnyCounter ("Any" kind) ask, where the counter kind is part of the
// choice; a fixed-kind ask has the same kind on every option.
func (s *State) FeatSubCtrWildcard() bool {
	if s.FeatPaySubCounter() {
		return true
	}
	if s.buildLive() && s.bld().subCtr != 0 && s.bld().e.kind == seSpell {
		return true // Quilled Greatwurm's wildcard cast cost
	}
	k := s.k()
	if k == nil {
		return false
	}
	a := s.buildAbility(k)
	return a != nil && a.Cost.Flags&kop.CostSubAmong != 0 && a.Cost.Ctr == kop.CtrAll
}

// FeatPaySubCounter reports whether the pending pick is a resolution
// SubCounter cost removing an "Any" kind (Slumbering Walker's
// trigger_cost_counter ask).
func (s *State) FeatPaySubCounter() bool {
	return s.res.ask == raGenPayPick && s.g.pickZ == kop.PaySubCounter
}
