package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The other-sacrifice payment-plan tier (the Ashnod's Altar family): gorge's
// payment planner prices a "Sacrifice a <creature>: Add" mana source as a
// LAST-RESORT step with the victims it sacrifices named in the witness
// (pay.PaymentPlanShapeTierOf's Sac arm -> Consequence{SacrificeOthers: true},
// pay.AssignPlanVictims, pay.PaymentPlanSacrificeVictims). The kernel mirrors
// the tier here: the source joins plan()'s phase 1 as a unit of the ability's
// production (planLastResort holds it out of phase 0), the found plan must be
// fundable with victims (an unfundable plan is no plan), the execution
// sacrifices exactly those victims with no ask, and the (source, victim)
// pairing rides the State so the plan witness the adapter builds carries the
// victims trailer (decision.appendPaymentPlanCanonical).
//
// The ability stays a potential-plan GAP too (gorge's manaAbilityCovered keeps
// a repeatable other-sacrifice source a gap, and the kernel's tapX search
// models it, mana_tapanother.go): the plan tier prices casts, the gap model
// prices potential plays, and gorge does both for the same board.
//
// Deferred by shape (open tickets): a ManaEach production (kop.SacXEachManaAb,
// Bolg's Company's {B}{R}), a spend-restricted one (kop.SacXRestrManaAb), and
// a sacrifice ability that is a card's LATER mana ability beside a plain {T}
// first one (kop.SecondSacTapManaAb, Phyrexian Tower).

// planSacPair is one planned step's (source, victim) pairing.
type planSacPair struct{ src, vic Obj }

// planSacLog is the planned other-sacrifice witness: the (source, victim)
// pairing of the planned steps the last Apply executed, in step order, and
// the count in use (Apply resets it: clearPlanSacWitness). It is a cold region without a view header, not a
// stateCore field: only a table whose cards plan an other-sacrifice source
// has one (TestNoNewFixedStateFields), and the adapter reads it after the
// apply, outside the transition's hot path.
type planSacLog struct {
	n     uint8
	pairs [planSacSteps]planSacPair
}

// planSacSteps bounds the planned other-sacrifice steps one plan's witness
// records. The log is per Apply, not per game (clearPlanSacWitness), and each
// step is a distinct source with a distinct victim, so a plan past it needs
// five Altar-family permanents and five creatures at once; the
// log stays small enough for the pools' State bar (TestEveryPoolFitsTheStateBar),
// and a plan past it is a declared capacity stall (planSacWitness), never a
// truncated witness.
const planSacSteps = 4

// coldPlanSac is the witness log, sized for a table whose census plans an
// other-sacrifice source. The log records at most one pairing per step.
var coldPlanSac = newCold[planSacLog]("planSac", func(g *genPrep) int {
	if g.uses.has(useSacOtherPlan) {
		return 1
	}
	return 0
})

// planSacNoTap reports whether the planned other-sacrifice ability abi of the
// card at sl taps nothing as it sacrifices (Ashnod's Altar's repeatable
// "Sacrifice a creature: Add {C}{C}"): its source stays untapped.
func (s *State) planSacNoTap(k *prep, sl Slot, abi int8) bool {
	if abi < 0 || int(abi) >= len(k.t.Cards[s.cardAt(sl)].Abilities) {
		return false
	}
	return k.t.Cards[s.cardAt(sl)].Abilities[abi].Cost.Flags&kop.CostTap == 0
}

// useSacOtherPlan: some card's census mana ability is the planned
// other-sacrifice shape (kop.OtherSacPlanAb); its detector holds for none.
var useSacOtherPlan = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			if kop.OtherSacPlanAb(t, &c.Abilities[j]) {
				return true
			}
		}
	}
	return false
})

// otherSacAb reports whether the mana facts pc name the source's census
// ability as the planned other-sacrifice shape: its manaAb is that ability
// (a later ability beside a plain {T} first one stays the tapX search's).
func (s *State) otherSacAb(k *prep, sl Slot, pc *pcard) (*kop.Ability, bool) {
	if k.gen == nil || !k.gen.uses.has(useSacOtherPlan) || pc.manaAb < 0 {
		return nil, false
	}
	c := s.cardAt(sl)
	if int(pc.manaAb) >= len(k.t.Cards[c].Abilities) {
		return nil, false
	}
	if ab := &k.t.Cards[c].Abilities[pc.manaAb]; kop.OtherSacPlanAb(k.t, ab) {
		return ab, true
	}
	return nil, false
}

// otherSacUsable: the source at sl can activate its planned other-sacrifice
// mana ability now, the gate scriptTapX reads for a sacrifice-another source
// (untapped and unsick only when the cost carries {T>; a no-{T} repeatable
// altar activates tapped, mana_tapanother.go tapXSources).
func (s *State) otherSacUsable(k *prep, sl Slot, pc *pcard) bool {
	ab, ok := s.otherSacAb(k, sl, pc)
	if !ok {
		return false
	}
	pm := &s.perms[sl]
	if ab.Cost.Flags&kop.CostTap != 0 && (pm.flags&pfTapped != 0 || s.activationSick(k, sl)) {
		return false
	}
	if s.genLost(k, sl) || k.gen != nil && k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return false
	}
	return true
}

// otherSacFundable reports whether the planned other-sacrifice source at sl
// has a victim besides itself on the board now: gorge's census never hands the
// search a step no plan could fund (planalts.go otherSacFundable).
func (s *State) otherSacFundable(k *prep, p uint8, sl Slot, pc *pcard) bool {
	ab, ok := s.otherSacAb(k, sl, pc)
	if !ok {
		return false
	}
	_, ok = s.otherSacVictim(k, p, sl, ab, slotMask{})
	return ok
}

// otherSacCensusVictim reports whether the source at sl is one of the plain
// planner census's units now (availForMode / ungrAvail) and names the victim
// it would take: gorge's payment census lists a "Sacrifice another permanent:
// Add" ability as a last-resort unit, so a cast it alone completes is on the
// offer list, not only one the scripted gaps walk prices. reserved holds the
// victims an earlier census unit of the same walk already named: gorge's
// planner activates each source at most once per plan and gives each step a
// distinct victim (pay.AssignPlanVictims), so two altars beside one creature
// are not two units. The scripted walks keep their own account of the source
// (availScript's tapX units), so the plain census arm is the only one that
// counts it here.
func (s *State) otherSacCensusVictim(k *prep, p uint8, sl Slot, pc *pcard, reserved slotMask) (Slot, bool) {
	if k.gen == nil || s.g.availX&(availGaps|availAny) != 0 || !k.gen.uses.has(useSacOtherPlan) {
		return NoSlot, false
	}
	ab, ok := s.otherSacAb(k, sl, pc)
	if !ok || !s.otherSacUsable(k, sl, pc) {
		return NoSlot, false
	}
	return s.otherSacVictim(k, p, sl, ab, reserved)
}

// addOtherSacCensusAvail adds every planned other-sacrifice source's plain
// census unit to a: the cast offer's affordability read prices the source,
// while the general availability walk (the scripted walks') does not. Each
// unit spends a distinct victim, as gorge's planner gives each step its own
// (pay.AssignPlanVictims).
func (s *State) addOtherSacCensusAvail(k *prep, p uint8, a *avail) {
	s.addOtherSacCensusAvailKeeping(k, p, a, slotMask{})
}

// addOtherSacCensusAvailKeeping is addOtherSacCensusAvail with kept naming the
// permanents the payment's own cost spends (an activation's self-sacrifice):
// gorge's plan never takes one as a victim (pay.AssignPlanVictims). It reports
// whether any unit was added.
func (s *State) addOtherSacCensusAvailKeeping(k *prep, p uint8, a *avail, kept slotMask) bool {
	reserved, added := kept, false
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		pc := s.manaPCGen(k, sl)
		v, ok := s.otherSacCensusVictim(k, p, sl, pc, reserved)
		if !ok {
			continue
		}
		reserved = reserved.with(v)
		s.availPlainUnit(k, sl, pc, a)
		added = true
	}
	return added
}

// actOtherSacAfford is canActivate's other-sacrifice read: the activation's
// mana cost, which the plain census av does not pay, is paid with the planned
// other-sacrifice units added and a payment plan exists. gorge's activation
// planner (planAbilityPayment) keeps the cost's own sacrificed source out of the
// victims (kept), and assigns each step a victim that is no plan source, so
// the dry plan (the activating source's own {T} out of its sources) must find
// one. An activation whose cost has another sacrifice part is left to the
// ordinary read: gorge reserves that part's candidates too.
func (s *State) actOtherSacAfford(k *prep, sl Slot, a *kop.Ability, mana kop.ManaCost, av *avail) bool {
	if k.gen == nil || !k.gen.uses.has(useSacOtherPlan) {
		return false
	}
	f := a.Cost.Flags
	if f&kop.CostSac != 0 && f&kop.CostSacSelf == 0 || f&kop.CostWaterbend != 0 || k.phyMana && mana.Phyrexian() {
		return false
	}
	p := s.perms[sl].ctrl
	cost := xFloorCost(mana, costXMin(&a.Cost))
	kept := actOtherSacKept(&a.Cost, sl)
	var withSac avail
	withSac = *av
	if !s.addOtherSacCensusAvailKeeping(k, p, &withSac, kept) || !s.abilityAffords(k, sl, a, &cost, &withSac) {
		return false
	}
	return s.actOtherSacPlans(k, p, sl, f, &cost, kept)
}

// otherSacKeepsSource is the cost parts that spend or mark the activating
// source: a self-sacrifice, {T}, {Q}, a counter removed from it or put on it.
const otherSacKeepsSource = kop.CostSacSelf | kop.CostTap | kop.CostUntapSelf |
	kop.CostRemoveCtr | kop.CostSubAmong | kop.CostLoyaltyX | kop.CostAddCtr

// actOtherSacKept is the activation's victim reservation, the one home of
// gorge's rule for it: gorge keeps a sacrificed source (planCostParts'
// selfKept) and a source the cost marks (pay.PlanCostMarksSource: a
// SubCounter, Exert or self AddCounter part), and reserves a {T}/{Q} source
// (planComposedCostFor's tapped, pay.AssignPlanVictims), so none is an
// other-sacrifice step's victim. The dry probes and the live payment
// (buildOtherSacKept) read it alike.
func actOtherSacKept(c *kop.Cost, sl Slot) slotMask {
	var kept slotMask
	if sl != NoSlot && (c.Flags&otherSacKeepsSource != 0 || c.Exert) {
		kept = kept.with(sl)
	}
	return kept
}

// buildOtherSacKept is actOtherSacKept for the activation being built: the
// live payment's plan reserves its source as the offer's dry probe did.
func (s *State) buildOtherSacKept(k *prep) slotMask {
	if !s.buildLive() || s.bld().e.kind != seAbility {
		return slotMask{}
	}
	a := s.buildAbility(k)
	if a == nil {
		return slotMask{}
	}
	return actOtherSacKept(&a.Cost, s.srcSlot())
}

// actOtherSacPlans is the activation's dry payment plan (the source's own {T}
// out of its sources): every other-sacrifice step names a victim outside kept.
func (s *State) actOtherSacPlans(k *prep, p uint8, sl Slot, f kop.CostFlags, cost *kop.ManaCost, kept slotMask) bool {
	exclude := NoObj
	if f&kop.CostTap != 0 {
		exclude = s.perms[sl].obj // the activation's own {T} takes the source out of the plan
	}
	var pk planner
	pk.try, pk.dry, pk.noGrant, pk.kept = true, true, true, kept
	s.tapForPK(k, p, cost, 0, exclude, false, false, &pk)
	return pk.found
}

// otherSacCensusPlans reports whether the plain census av, which does not pay
// cost alone, pays it with the planned other-sacrifice units added AND the
// cast has a payment plan: the cast's planner (o excluded, the spell's own
// card) finds a plan whose other-sacrifice steps each name a distinct victim
// that is no plan source (planSacFundable). The census unit prices the altar
// without knowing which sources the plan taps, so a mana creature that is the
// only fodder pays {W} and is also the altar's only victim; gorge assigns
// victims outside the plan's sources (pay.AssignPlanVictims) and prices no plan
// there, and the spell's own target in that spot is consumed by the potential
// route's script instead (CR 601.2c, cast_tgtkeep.go). av gains the units.
func (s *State) otherSacCensusPlans(k *prep, p uint8, o Obj, cost *kop.ManaCost, av *avail) bool {
	if k.gen == nil || !k.gen.uses.has(useSacOtherPlan) {
		return false
	}
	s.addOtherSacCensusAvail(k, p, av)
	if !av.affords(cost, pipMaskOf(cost), 0) {
		return false
	}
	var pk planner
	pk.try, pk.dry, pk.noGrant = true, true, true
	s.tapForPK(k, p, cost, 0, o, false, false, &pk)
	return pk.found
}

// otherSacSourceUsable: sl is a source planOrder may price as a planned
// other-sacrifice unit: activatable now and with a victim (the planSourceUsable
// arm).
func (s *State) otherSacSourceUsable(k *prep, p uint8, sl Slot) bool {
	if k.gen == nil || !k.gen.uses.has(useSacOtherPlan) {
		return false
	}
	pc := s.manaPCGen(k, sl)
	return s.otherSacUsable(k, sl, pc) && s.otherSacFundable(k, p, sl, pc)
}

// otherSacVictim picks the permanent the planned step sacrifices: the
// source's sacrifice candidates (sacXCands: p controls, on the battlefield,
// matches the filter, not blocked, not a Convoke/Waterbend pick and not the
// build's self-tap, battlefield order) that are not reserved, ranked tokens
// first, then lowest mana value, ties by object id -- gorge's fixed ranking
// (pay.PaymentPlanSacrificeVictims), a total order so the two engines cannot
// disagree. reserved holds the plan's own sources, the payment's kept slots
// and the earlier steps' victims; the step's own source is never a victim.
func (s *State) otherSacVictim(k *prep, p uint8, sl Slot, ab *kop.Ability, reserved slotMask) (Slot, bool) {
	var cs [MaxPerm]Slot
	n := s.sacXCands(k, p, sl, ab, &cs)
	best, found := Slot(0), false
	for _, x := range cs[:n] {
		if x == sl || reserved.has(x) {
			continue
		}
		if !found {
			best, found = x, true
			continue
		}
		if s.planSacVictimLess(k, x, best) {
			best = x
		}
	}
	return best, found
}

// planSacVictimLess is the ranking of two candidates, total on (token, mana
// value, object id). The mana value is the kernel's permanent read (mvPerm:
// the copied face's printed value, an embalm or copied-back-face permanent at
// 0), which mirrors gorge's state.ManaValue for a permanent. A continuous
// mana-value change is not modelled on either side; no supported card makes
// one a planned victim today.
func (s *State) planSacVictimLess(k *prep, x, y Slot) bool {
	xo, yo := s.perms[x].obj, s.perms[y].obj
	xt, yt := s.objs[xo].of&ofToken != 0, s.objs[yo].of&ofToken != 0
	if xt != yt {
		return xt
	}
	xm, ym := s.mvPerm(k, x), s.mvPerm(k, y)
	if xm != ym {
		return xm < ym
	}
	return xo < yo
}

// planSacReserved is the victim exclusion set of the found plan: every unit's
// source slot and the payment's kept slots, the dry probe's or the live
// activation's (pay.AssignPlanVictims never takes a plan source, a kept or
// gone object, or an earlier step's victim; a taken victim has left the
// battlefield and drops out of the walk by itself).
func (s *State) planSacReserved(k *prep, pk *planner, bn int) slotMask {
	res := pk.kept.or(s.buildOtherSacKept(k))
	for i := 0; i < bn; i++ {
		res = res.with(pk.units[pk.bestSeq[i]])
	}
	return res
}

// planSacFundable reports whether every other-sacrifice unit of the found
// plan can name a distinct victim, in run order (gorge assigns victims after
// the search, pay.AssignPlanVictims, and an unfundable plan is not offered).
// It runs for the dry probe and the live pass alike: the offer stands or falls
// on it in gorge.
func (s *State) planSacFundable(k *prep, p uint8, pk *planner, bn int) bool {
	res := s.planSacReserved(k, pk, bn)
	for i := 0; i < bn; i++ {
		ty := &pk.types[pk.bestTy[i]]
		if !ty.sacO {
			continue
		}
		sl := pk.units[pk.bestSeq[i]]
		pc := s.manaPCGen(k, sl)
		ab, ok := s.otherSacAb(k, sl, pc)
		if !ok {
			return false
		}
		vic, ok := s.otherSacVictim(k, p, sl, ab, res)
		if !ok {
			return false
		}
		res = res.with(vic)
	}
	return true
}

// planSacLogOf is s's witness log, nil when the table plans no
// other-sacrifice source.
func (s *State) planSacLogOf() *planSacLog {
	if r := coldPlanSac.of(s); len(r) != 0 {
		return &r[0]
	}
	return nil
}

// planSacWitness appends a planned step's (source, victim) pairing for the
// plan witness tapsOf builds (dirac/adapter/decision.go): the victim trailer
// of the canonical plan identity reads it off the scratch State.
func (s *State) planSacWitness(src, vic Obj) {
	l := s.planSacLogOf()
	if l == nil {
		return
	}
	if int(l.n) >= len(l.pairs) {
		s.capFull(StallSac) // more planned victims than the witness log holds (planSacSteps)
		return
	}
	l.pairs[l.n] = planSacPair{src: src, vic: vic}
	l.n++
}

// clearPlanSacWitness empties the planned-victim pairing. Apply calls it
// first, so the pairing belongs to that one answer's payment alone: the live
// State never carries an earlier payment's steps into the planSacSteps bound,
// and the adapter's scratch apply (tapsOf) reads only its own cast's pairing.
func (s *State) clearPlanSacWitness() {
	if l := s.planSacLogOf(); l != nil {
		l.n = 0
	}
}

// PlanSacWitness is the pairing of the last Apply's planned steps: parallel
// slices of source and victim object ids (kernel Obj indexes), in plan step
// order.
func (s *State) PlanSacWitness() (srcs, vics []Obj) {
	l := s.planSacLogOf()
	if l == nil {
		return nil, nil
	}
	for i := 0; i < int(l.n); i++ {
		srcs = append(srcs, l.pairs[i].src)
		vics = append(vics, l.pairs[i].vic)
	}
	return srcs, vics
}

// affordsUngrantedActOtherSac is affordsUngrantedPlay's other-sacrifice read of
// an activation: the planner's census (ungrAvail) beside the planned
// other-sacrifice units, none taking the cost's own sacrificed source as its
// victim (actOtherSacAfford is the offer's own read of the same payment).
func (s *State) affordsUngrantedActOtherSac(k *prep, p uint8, a Action, cost *kop.ManaCost, ex Slot, d payDesc) bool {
	if a.Kind() != AkActivate || k.gen == nil || !k.gen.uses.has(useSacOtherPlan) {
		return false
	}
	ob := &s.objs[Obj(a.Source())]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return false
	}
	sl, ab := ob.slot, int(a.Mode())
	abs := k.t.Cards[s.cardAt(sl)].Abilities
	if ab >= len(abs) {
		return false
	}
	f := abs[ab].Cost.Flags
	if f&kop.CostSac != 0 && f&kop.CostSacSelf == 0 {
		return false
	}
	kept := actOtherSacKept(&abs[ab].Cost, sl)
	var av avail
	var ms modalSet
	s.ungrAvail(k, p, ex, &av, &ms, slotMask{})
	return s.addOtherSacCensusAvailKeeping(k, p, &av, kept) && s.affordsUngrantedAv(k, p, cost, ex, d, &av, &ms) &&
		s.actOtherSacPlans(k, p, sl, f, cost, kept)
}

// affordsUngrantedCastOtherSac is affordsUngrantedPlay's other-sacrifice read
// of a cast: gorge's planner census lists the printed "Sacrifice a creature:
// Add" source as a last-resort unit (pay.PaymentPlanShapeTierOf), so a cast it
// prices with the planned other-sacrifice units is a planned payment with no
// prefix, however many granted gaps stand beside it (CR 601.2g-h). A spell
// whose additional cost sacrifices a permanent (Bone Splinters, a sacrifice
// kicker) is refused here, as the activation arm refuses another-sacrifice
// cost: the census victim it ranks could be that cost's pick (CR 601.2h).
func (s *State) affordsUngrantedCastOtherSac(k *prep, p uint8, a Action, cost *kop.ManaCost, ex Slot, d payDesc) bool {
	if a.Kind() != AkCast || k.gen == nil || !k.gen.uses.has(useSacOtherPlan) {
		return false
	}
	if c := &k.t.Cards[s.objs[Obj(a.Source())].card]; (c.SpellCost.Flags|c.AltAddCost.Flags|c.AltAddCost2.Flags|c.KickerCost.Flags)&kop.CostSac != 0 {
		return false
	}
	var av avail
	var ms modalSet
	s.ungrAvail(k, p, ex, &av, &ms, slotMask{})
	if !s.addOtherSacCensusAvailKeeping(k, p, &av, slotMask{}) || !s.affordsUngrantedAv(k, p, cost, ex, d, &av, &ms) {
		return false
	}
	var pk planner
	pk.try, pk.dry, pk.noGrant = true, true, true
	s.tapForPK(k, p, cost, 0, Obj(a.Source()), false, false, &pk)
	return pk.found
}

// planSacOUnit reports whether plan's census position i at sl is a planned
// other-sacrifice source (Ashnod's Altar's family): gorge's census lists it
// only as a last-resort alternative (pay.PaymentPlanShapeTierOf), and only
// when a victim besides the source stands on the board (its planalts.go
// otherSacFundable gate). A sacrificing, granted or restricted position is
// not one.
func (s *State) planSacOUnit(k *prep, p uint8, sl Slot, sac, granted bool, rk uint8) bool {
	return !sac && !granted && rk == 0 && s.otherSacSourceUsable(k, p, sl)
}

// planDropUnfundable clears pk.found when the found plan's other-sacrifice
// steps cannot be funded with distinct victims: gorge assigns the plan's
// victims after the search (pay.AssignPlanVictims) and does not offer a cast
// it cannot fund.
func (s *State) planDropUnfundable(k *prep, p uint8, pk *planner) {
	if pk.found && !s.planSacFundable(k, p, pk, pk.bestN) {
		pk.found = false
	}
}

// planTapsSource reports whether plan's executed step taps its source sl: a
// life-only census gap's unit pays its life, not a tap (mana_lifegap.go;
// lifeGapPlanCost pays it), a sacrifice-only unit is not tapped, and neither
// is a no-{T} other-sacrifice source (Ashnod's Altar).
func (s *State) planTapsSource(k *prep, p uint8, sl Slot, ty *utype, abi int8) bool {
	return !s.lifeGapPlanCost(k, p, sl) && !ty.sacOnly && !(ty.sacO && s.planSacNoTap(k, sl, abi))
}

// planSacOStep executes a planned other-sacrifice step at source sl: it
// sacrifices exactly the victim the ranking names, with no ask, and records
// the pairing on the State so the plan witness can name it. It returns the
// victim exclusion set with the victim added.
func (s *State) planSacOStep(k *prep, p uint8, sl Slot, res slotMask) slotMask {
	ab, ok := s.otherSacAb(k, sl, s.manaPCGen(k, sl))
	vic, ok2 := Slot(0), false
	if ok {
		vic, ok2 = s.otherSacVictim(k, p, sl, ab, res)
	}
	if !ok || !ok2 {
		panic("kernel: planned other-sacrifice step has no victim") // planSacFundable priced it at the search
	}
	res = res.with(vic)
	vo := s.perms[vic].obj
	s.planSacWitness(s.perms[sl].obj, vo)
	s.fireSac(k, vic)
	s.leaveForMana(k, vic)
	if s.g.on && s.objs[vo].of&ofToken != 0 && s.objs[vo].zone == kop.ZoneGraveyard && s.buildLive() && (s.ctl != ctlBuild || s.bld().bfl&bflManaStep == 0) {
		s.ceaseManaToken(vo) // a token victim ceases at once, as gorge pays it
	}
	s.sbaAfterManaSacrifice(k)
	return res
}
