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
//go:noinline
func (s *State) phyPay(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj) {
	if s.phyAllLife(k) {
		s.tapFor(k, p, cost, x, exclude)
		return
	}
	_, held := s.windowTapRiderUseful(k, p, cost, cost, true, false)
	for c := range held {
		s.pl[p].pool[c] += held[c]
	}
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
// one colour, where that is the same set of assignments.
//
//go:noinline
func (s *State) phyActAffords(k *prep, sl Slot, a *kop.Ability, p uint8, c kop.ManaCost, av *avail) bool {
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
