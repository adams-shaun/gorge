package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The announced tapXType<X/Spec> of a printed activated ability ("tap X
// untapped Knights you control": Aryel, Knight of Windgrace; CR 107.3,
// 601.2b, 602.2b). The table encodes it as kop.CostTapX with Cost.N 0 (a
// literal tapXType<N/Spec> has N >= 1, the CostSac N 0 convention). gorge:
// tapPermanentCostAsk's X-form election runs before xAsk and the targets
// and announces X as the count chosen: one KChoose "tapcost", Min 0, Max the
// untapped matching permanents the payer controls (the source excluded when
// a {T} in the same cost taps it); no candidate settles X = 0 unasked. The
// offer gate judges an X-bound target pending (costAnnouncesX counts the
// part). The kernel poses it at bsTapCost as sequential pkTapX picks closed
// by Done (offered at any count, X = 0 included) or, when every candidate is
// picked, by itself (gorge's answer is complete at Max picks), as the oracle
// walks a multi-pick decision. The entry's X is the count picked; payPicks
// taps them with the rest of the cost. Cold code: reached only behind
// the useTapXAnn bit.

// DebugTapXAnnUnbound is the mutation switch: the count tapped never binds X.
var DebugTapXAnnUnbound bool

// useTapXAnn: some activated ability costs an announced tapXType<X/Spec>.
var useTapXAnn = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if tapXAnnCost(&t.Cards[i].Abilities[j].Cost) {
				return true
			}
		}
	}
	return false
})

// tapXAnnCost reports whether c taps the announced X permanents.
func tapXAnnCost(c *kop.Cost) bool {
	return c != nil && c.Flags&kop.CostTapX != 0 && c.N == 0
}

// tapXAnnOptions poses the next pick of the entry's announced tap part
// (bsTapCost). false: the part is settled (or the entry has none) and the
// build moves on.
//
//go:noinline
func (s *State) tapXAnnOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if e.kind != seAbility || s.bld().donePicks&pkTapX != 0 {
		return false
	}
	pc := s.buildPickCost(k)
	if !tapXAnnCost(pc) {
		return false
	}
	src := s.srcSlot()
	picked := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkTapX {
			picked++
		}
	}
	if !DebugTapXAnnUnbound {
		e.x = objWord(picked) // the announced X so far: the count picked
	}
	nc := 0
	d.N = 0
	for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
		sl := m.first()
		if s.picked(s.perms[sl].obj) || s.perms[sl].flags&pfTapped != 0 ||
			sl == src && pc.Flags&kop.CostTap != 0 || !s.matchPerm(k, pc.Filter, sl, e.ctrl, src) {
			continue
		}
		d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
		nc++
	}
	if nc == 0 || int(s.bld().pickN) >= len(s.bld().picks) {
		// no candidate left: gorge settles X = 0 unasked, or the answer is
		// complete at Max picks (no Done); a pick past the record's
		// capacity stalls
		if nc > 0 {
			s.capFull(StallTapPick)
		}
		d.N = 0
		s.tapPowerDone()
		return false
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
	s.bld().pickK = pkTapX
	d.Kind, d.Sub, d.Player = DkChoose, ChCostTap, e.ctrl
	return true
}
