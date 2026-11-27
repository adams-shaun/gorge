package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A play only gorge's payment planner funds by activating a plain-creature
// sacrifice-another source (pay.PlanOtherSacShape, autoPayOffer's last-resort
// arm) is paid at the payment step, after the cast's announcement and its
// target asks (docs/design/2026-09-27-announce-then-pay.md; CR 601.2g-h), not
// by the prefix script before the play. The prefix searches price such a play
// at announce; planSacDeferStep keeps its one activation out of the prefix and
// planSacPay makes it at manaStep, so the victim and the floating mana appear
// where gorge's do.

// planSacDeferStep leaves a found line's single activation st (the whole
// line) for the cast's payment (planSacPay) and reports it did; false: the
// line is the prefix's, applied at announce. Only a hand cast with a required
// target defers (no ask lies between announcement and payment otherwise, so the
// two orders are indistinguishable) and only a source with no {T} cost (a {T}
// source's own tap is the planner's tap too: the prefix keeps it).
func (s *State) planSacDeferStep(k *prep, a Action, st *tapXStep) bool {
	if a.Kind() != AkCast || st.kind != tapXAct || !spellRequiresTarget(k, s.castFaceCard(k, a)) ||
		!s.planOtherSacSource(k, st.sl) || k.tapXAbOf(s.cardAt(st.sl)).Cost.Flags&kop.CostTap != 0 {
		return false
	}
	s.g.planSac = [3]uint8{uint8(st.sl) + 1, uint8(st.cand), st.col}
	return true
}

// planSacPay activates the deferred activation, if any (the cast's mana step
// runs it before the planner taps the printed sources).
func (s *State) planSacPay(k *prep) {
	d := s.g.planSac
	if d[0] == 0 {
		return
	}
	s.g.planSac = [3]uint8{}
	sl := Slot(d[0] - 1)
	if !s.occ.has(sl) || !s.occ.has(Slot(d[1])) {
		return
	}
	s.tapXDo(k, s.bld().e.ctrl, tapXStep{sl: sl, cand: Slot(d[1]), cand2: NoSlot, col: d[2], kind: tapXAct})
}

// planSacDeferGrant is planSacDeferStep for the grant search's found line.
func (s *State) planSacDeferGrant(k *prep, a Action, g *grantSrch) bool {
	st := &g.path[0]
	return g.np == 1 && st.kind == stepAct && st.gi == stepSacX &&
		s.planSacDeferStep(k, a, &tapXStep{sl: st.sl, cand: st.cand, col: st.col, kind: tapXAct})
}
