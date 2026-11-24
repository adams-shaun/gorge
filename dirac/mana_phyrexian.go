package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Phyrexian mana (CR 107.4f, 601.2b): a printed {W/P} pip is
// paid with its colour or two life. kgen encodes the pips as twobrid's
// (kop.ManaCost.Tb, Hyb[0] HybTwobrid) with Hyb[1] HybPhyrexian, so the
// twobrid announcement, offer and resolution paths carry them
// (mana_twobrid.go): the caster announces each pip's face (gorge
// pay.ManaAsk: pay_<colour>, then pay_life, each offered when a complete
// assignment stays feasible), the life face costs no mana (ResolveTb) and
// at most the payer's life (affordsTb, avail.life), and it counts one
// toward the mana value. gorge's planner prices only the cost's mana half
// (pay.PlanManaHalf) and plans again for the cost the pips settled
// (rearmAutoPay); the manual window pays when no plan exists (manaStep). The
// two life per life face are paid with the rest of the cost
// (payPhyrexianLife).

// phyrexianLife is the announcement mode of a Phyrexian pip paid with two
// life (gorge pay_life; the oracle's canonical mode 7).
const phyrexianLife = 7

// phyLifeAnnounced is the number of the build's announced pips paid with
// life (answerPipTb's bits).
func (s *State) phyLifeAnnounced() int {
	n := min(int(s.bld().pipN), 8)
	return bits.OnesCount8(s.bld().pipC[0] & uint8(1<<n-1))
}

// phyBuild reports whether the spell or activation being built carries
// Phyrexian pips (an activated ability's cost:).
func (s *State) phyBuild(k *prep) bool {
	return s.bld().e.kind != seTrigger && s.buildCost0(k).Phyrexian()
}

// phyAllLife reports whether every announced pip of a Phyrexian spell was
// paid with life (the planned witness then pays the mana half).
//
//go:noinline
func (s *State) phyAllLife(k *prep) bool {
	return s.phyBuild(k) && s.phyLifeAnnounced() == int(s.bld().pipN)
}

// payPhyrexianLife pays two life per Phyrexian pip announced as life.
//
//go:noinline
func (s *State) payPhyrexianLife(k *prep) {
	if !s.phyBuild(k) {
		return
	}
	if n := s.phyLifeAnnounced(); n > 0 && !DebugNoPhyrexianLife {
		s.loseLife(k, s.bld().e.ctrl, 2*n)
	}
}

// phyCastPlanSettles reports whether gorge's plan pays a Phyrexian cast's settled
// cost. The seat's plan prices the mana half, none at all for a cost of pips alone,
// and gorge plans again once the pips are answered (rearmAutoPay).
func (s *State) phyCastPlanSettles(k *prep, e *stackEntry, half, settled kop.ManaCost) bool {
	var chk planner
	chk.dry = true
	s.tapForPK(k, e.ctrl, &half, 0, NoObj, false, false, &chk)
	if chk.needT != 0 && !chk.found {
		return false
	}
	chk = planner{dry: true}
	s.tapForPK(k, e.ctrl, &settled, int(e.x), NoObj, false, false, &chk)
	return chk.found || chk.needT == 0
}

// phyPay taps the sources of a Phyrexian cost no plan settles: the planned
// witness when every pip was paid with life, else gorge's manual CR 601.2g
// window over the resolved cost.
//
// phyPlanHalf is gorge pay.PlanManaHalf (payment_plan.go): the cost without
// its Phyrexian pips. A pip's colour face and its two-life face are both
// answered afterwards by the CR 601.2b pip ask, so the cast planner prices
// this half alone and the offer gate reads affordable when it alone is
// payable.
func phyPlanHalf(cost kop.ManaCost) kop.ManaCost {
	half := cost
	half.Tb, half.Hyb = 0, [len(kop.ManaCost{}.Hyb)]kop.ColorMask{}
	return half
}

// phyCastGateAffords is the offer gate's affordability read for p's cast of
// card c at cost over cav: the printed cost when its own is affordable, else
// — for a Phyrexian cost — the cost again over the cast window's census
// (availCastWindowFor, availWinCol) when the pip-free mana half, gorge
// pay.PlanManaHalf, is affordable at all (CR 601.2b: each pip's face is
// announced afterwards, so the offer prices the whole cost with the
// assignment the announcement will find).
//
// CR 107.4f / CR 119.4 bound a two-life face by the payer's life (the
// affordsTb guard), so at 1 life every pip must be paid with its colour. A
// colour face is still payable from a damage-painful source the offer
// census holds out at 1 life — paying the pain is legal, if lethal — and the
// window's census counts it (dr-mbc-baab pair 141, Porcelain Legionnaire at
// 1 life). The pip-free half alone is not enough: a cost whose pips need
// colour units the census does not have withholds the cast even when its
// generic half is affordable (phyact seed 510, Moltensteel Dragon's two
// {R/P} at 1 life from three Mountains and an Island).
func (s *State) phyCastGateAffords(k *prep, p uint8, cav *avail, cost *kop.ManaCost) bool {
	if cav.affords(cost, pipMaskOf(cost), 0) {
		return true
	}
	if !cost.Phyrexian() {
		return false
	}
	h := phyPlanHalf(*cost)
	if !cav.affords(&h, pipMaskOf(&h), 0) {
		return false
	}
	if !k.painMana {
		return false // no damage-painful source the offer census dropped
	}
	var w avail
	s.availCastWindowFor(k, p, &w)
	return w.affords(cost, pipMaskOf(cost), 0)
}

// phyHalfPlanned reports whether gorge's planner prices plain cast a's Phyrexian
// cost over the printed sources: it prices the cost's pip-free mana half alone
// (pay.PlanManaHalf; the pips are answered after it, CR 601.2b), so the autopay
// surface keeps a cast whose half the printed sources pay even when the whole
// cost needs a census gap's unit (a Saruli Caretaker's relaxed any-colour unit
// for the fifth mana of {4}{G/P}{G/P} at two life). The prefix search then
// decides the cast like any other potential play.
func (s *State) phyHalfPlanned(k *prep, p uint8, a Action) bool {
	o := Obj(a.Source())
	c := s.objs[o].card
	if !k.cards[c].cost.Phyrexian() {
		return false
	}
	// The half the cast planner funds is its COMPOSED cost without the pips
	// (plannedCastCost: the printed cost re-priced by every live cost static,
	// then the Phyrexian bits cleared), not the printed cost: Sphere of
	// Resistance's surcharge is part of the half, and pricing the printed
	// half let the autopay surface keep a cast gorge's planner calls
	// insufficient (Pith Driller's {5} half beside five net sources).
	half, ok := s.plannedCastCost(k, p, o, 0)
	if !ok {
		return false
	}
	saved := s.g.availX
	s.g.availX = availWalk
	var av avail
	s.availFor(k, p, &av)
	s.g.availX = saved
	return av.affords(&half, pipMaskOf(&half), 0)
}

// phyAbilityMana pays a Phyrexian activation: the plan for the cost its pips
// settled, armed on the mana half; the manual window when none exists
// (mana_phyrexian.go). It reports whether the payment settled. A cost the
// window cannot fund (a colour face whose only funder is a census gap the
// window skips, as at 1 life with no source of the pip's colour) is left to
// the caller to reverse, as gorge reverses an activation it cannot pay
// (CR 733.1): the pip ask offered the face, but the payment is not owed.
//
//go:noinline
func (s *State) phyAbilityMana(k *prep, e *stackEntry, cost *kop.ManaCost, saved *[len(build{}.picks)]uint8) bool {
	a := &k.t.Cards[e.card].Abilities[e.ab]
	half := a.Cost.Mana
	half.Tb, half.Hyb = 0, [len(kop.ManaCost{}.Hyb)]kop.ColorMask{}
	if s.abilityPlanSettles(k, e, half, half, *cost, saved) {
		s.tapFor(k, e.ctrl, cost, int(e.x), NoObj)
		return true
	}
	return s.phyPay(k, e.ctrl, cost, int(e.x), NoObj)
}

// phyPay pays a Phyrexian cost through the manual window and reports whether
// the cost is settled: a window that ran out of useful sources leaves the
// payment owed (an activation's caller reverses it, keeping the window's taps
// and floating mana, CR 733.1).
func (s *State) phyPay(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj) bool {
	if s.phyAllLife(k) {
		s.tapFor(k, p, cost, x, exclude)
		return true
	}
	if !s.phyWindowReach(k, p, cost) && !s.phyFilterReach(k, p, cost) {
		return false // a face only a filter could make: gorge's window never opens, it reverses
	}
	// The window's pick is the first useful source in battlefield order and
	// its owed colour (or the restricted ability that makes it), as gorge's
	// manual window answers. The clamp is the window's offer gate: a source
	// that makes no colour the cost still owes does not tap for nothing.
	ok, held := s.windowTapRiderUseful(k, p, cost, cost, true, false)
	for c := range held {
		s.pl[p].pool[c] += held[c]
	}
	if s.winPaused() {
		return true // a tap-another pick is pending: the payment runs on when it answers
	}
	return ok && s.windowSpellSettled(k, p, cost, false)
}

// phyFilterReach: some filter (Bog Initiate's "{1}: Add {B}.", Selesnya
// Signet's "{1}, {T}: Add {W}{G}.") makes the potential pool pay cost, so
// gorge's stranded check lets the window open (its PotentialMana fixpoint
// prices the filter's own production, potential_repeat_fee.go) though the
// printed sources do not reach it. The window then lists no filter that cannot help (windowFeeStalls)
// and the cast is reversed when it ends short (CR 733.1; phyFilterStranded).
func (s *State) phyFilterReach(k *prep, p uint8, cost *kop.ManaCost) bool {
	if k.gen == nil || !k.gen.anyFilt {
		return false
	}
	var av avail
	s.availPotential(k, p, &av)
	return av.affords(cost, pipMaskOf(cost), 0)
}

// phyFilterStranded: the Phyrexian cost is reachable only through a repeatable
// filter (phyFilterReach), so a window that leaves it unpaid reverses the cast,
// as gorge's does when the player is done (CR 733.1).
func (s *State) phyFilterStranded(k *prep, p uint8, cost *kop.ManaCost) bool {
	return !s.phyAllLife(k) && !s.phyWindowReach(k, p, cost) && s.phyFilterReach(k, p, cost)
}

// phyWindowReach reports whether the payment's own pool, the untapped printed
// sources, the deferred sources the window can tap (a TriggersWhenSpent$ mana
// ability the planner's census omits, mana_deferred.go), the restricted mana it
// may spend and the restricted sources whose mana it admits reach cost. The
// check is the window's opening condition, not
// the offer's: a colour face only a paid filter could make (pair 11's
// Souleater beside School of the Unseen) leaves gorge's window unopened, so
// the activation reverses with no taps; the filter never pays a pip the
// planner did not plan.
func (s *State) phyWindowReach(k *prep, p uint8, cost *kop.ManaCost) bool {
	var av avail
	s.availFor(k, p, &av)
	// the planner's census omits a deferred source (Gilanra's {G}), but the
	// window can tap it, and the pip ask that announced the face counted it
	// (mana_deferred.go)
	s.addDeferredUnits(k, p, &av)
	s.addPipTapsDefer(k, &s.bld().e, &av) // a Taps-deferred source too (mana_pipstapsdefer.go)
	// a Combo Any source (Axebane Guardian) is no planner unit, but the window
	// taps it for its amount of any colour (windowComboTap, mana_comboany.go)
	if k.gen != nil && k.gen.anyComboAny && s.g.availX&3 == 0 {
		s.availCA(k, p, &av)
	}
	if av.affords(cost, pipMaskOf(cost), 0) {
		return true
	}
	if s.rbAny(p) {
		d := s.payDescNow(NoObj)
		if m := s.restrFloatAdmits(k, p, &d); m != 0 {
			s.restrAvailAdd(p, m, &av)
		}
	}
	if av.affords(cost, pipMaskOf(cost), 0) {
		return true
	}
	// an untapped restricted source (Gwenna's creature-only {R}) is the
	// clamped window's pick: the pending payment admits its mana (CR 106.6),
	// whether a plain restricted source or a wide any-colour one.
	if k.restrSrc && s.restrFaceAffords(k, p, cost, &av, 0) {
		return true
	}
	return k.gen != nil && k.gen.anyMultiR && s.wideFaceAffords(k, p, cost, &av, 0)
}

// DebugNoPhyrexianLife drops the two-life payment (the phy mutation test).
var DebugNoPhyrexianLife bool

// phyActAffords is a Phyrexian activation's offer gate (CR
// 107.4f, CR 119.4): offered when some assignment of its pips, each paid
// with its colour or two life, is payable: the life faces at most the
// activator's life (two each; a player can pay life only up to their life
// total) and the resolved mana cost affordable from av. gorge (from
// cr-phyrexian-activation-low-life, 81fb97473) plans the cost with its first
// k pips as life for each affordable k; every corpus activation's pips share
// one colour, where that is the same set of assignments. gorge's witness
// prices the mana half with its planner, which reaches a census-gap source's
// unit through the script route (Hall of Tagsin's "{1}, {T}: Add one mana of
// any color"), so the gate counts what av carries, gap units included.
//
// A storage battery (Subterranean Hangar) is a census gap, but its live
// counters are real mana and the planner prices them (mana_countercost.go
// storeGap): the real offer's gate reads them beside the plain units, or a
// second activation its counters pay is withheld (pair 149). The plain
// probes (availPlain: plainOptions and the other probe walks, which model
// gorge's planner) read without them — the planner reaches a battery only
// through the script route, so a play its probes list runs unscripted and
// the battery is never activated (pair 149).
//
//go:noinline
func (s *State) phyActAffords(k *prep, sl Slot, a *kop.Ability, p uint8, c kop.ManaCost, av *avail) bool {
	if k.gen != nil && s.g.availX&availPlain == 0 {
		var store avail
		store = *av
		if s.phyStoreUnits(k, p, &store) {
			av = &store
		}
	}
	n := min(c.TbN(), 5)
	life := int(s.pl[p].life)
	base := c
	base.Tb, base.Hyb = 0, [len(kop.ManaCost{}.Hyb)]kop.ColorMask{}
	for m := 0; m < 1<<n; m++ { // bit i set: pip i paid with two life
		if 2*bits.OnesCount8(uint8(m)) > life {
			continue
		}
		r := base
		for i := 0; i < n; i++ {
			if m&(1<<i) == 0 {
				r.Pips[c.TbCol(i)]++
			}
		}
		if s.abilityAffords(k, sl, a, &r, av) {
			return true
		}
	}
	return false
}

// phyStoreUnits adds p's untapped storage batteries' live counters to a: each
// removes any number of its counters for that many mana of its colour, and its
// announced X is the counters it holds (mana_countercost.go storeGap). It
// reports whether it added any.
func (s *State) phyStoreUnits(k *prep, p uint8, a *avail) bool {
	added := false
	for b := s.scriptCtr(k, p); b.any(); b = b.rest() {
		sl := b.first()
		ab := k.ctrAbOf(s.cardAt(sl))
		if ab == nil || !kop.StoreManaAb(ab) {
			continue
		}
		if n := min(s.perms[sl].ctrN(s, int(ab.Cost.Ctr)), 255); n > 0 {
			a.addUnits(uint8(ab.ManaColors)&0x3f, uint8(n))
			added = true
		}
	}
	return added
}
