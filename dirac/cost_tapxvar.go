package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// tapXType<X/Spec> as a printed trigger's own cost (Myr Battlesphere: "you may
// tap X untapped Myr you control. If you do, ..."). gorge's triggered-cost
// window (cumulative.go triggeredTapAsk) poses one KChoose over the untapped
// permanents matching the spec, Min 0 and Max the candidate count, with no
// pay / decline ask in front of it (the trigger's OptionalDecider$ folds into
// it): the empty answer is the decline, any other taps the chosen permanents
// and binds their count as the body's Count$xPaid. With no candidate the
// window declines without posing anything.
//
// The cost is OpIfPaid with PayTapX and B.N PayBSubTapXVar, F the spec. The
// pick is the literal tapXType's (raGenPayPick, ChGenPayTap, one
// permanent per answer, tapped as it is chosen) with an AkDone closing it: pickLeft
// counts the taps so far, Done with none declines, Done after some pays with
// X = pickLeft. Cold and out of line.

// tapXVarPick reports whether the pending pick is a tapXType<X/Spec> one.
func (s *State) tapXVarPick(k *prep) bool {
	return s.res.ask == raGenPayPick && s.g.pickZ == kop.PayTapX && s.prog(k)[s.res.pc].B.N == kop.PayBSubTapXVar
}

// execTapXVar starts the cost: false = suspended on the pick, true = no
// candidate, declined (the IfPaid body skipped).
//
//go:noinline
func (s *State) execTapXVar(k *prep, p uint8, skip uint16, f kop.FilterID) bool {
	if s.countUntapped(k, p, f, s.resSrcSlot(k)) == 0 {
		s.res.pc += uint8(skip)
		return true
	}
	s.g.pickF = skip
	s.g.pickZ = kop.PayTapX
	s.g.pickLeft = 0
	s.g.pickP = p
	s.res.ask, s.res.askP = raGenPayPick, p
	return false
}

// tapXVarDone offers Done beside the candidates of a tapXType<X/Spec> pick.
//
//go:noinline
func (s *State) tapXVarDone(k *prep, d *Decision) {
	if s.tapXVarPick(k) {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
}

// applyTapXVarPick answers one pick: a permanent taps (the count grows), Done
// closes the cost, paid when anything was tapped.
//
//go:noinline
func (s *State) applyTapXVarPick(k *prep, a Action) {
	if a.Kind() == AkDone {
		s.payDone(k, s.g.pickLeft > 0)
		return
	}
	if o := Obj(a.Source()); s.objs[o].zone == kop.ZoneBattlefield {
		s.tapCostPerm(k, s.objs[o].slot, o)
	}
	s.g.pickLeft++
}

// tapXVarBind makes the count tapped the resolving trigger's Count$xPaid.
//
//go:noinline
func (s *State) tapXVarBind(k *prep) {
	if s.prog(k)[s.res.pc].B.N == kop.PayBSubTapXVar {
		s.res.e.x = s.g.pickLeft
	}
}
