package dirac

// The keep-tapped election of the untap step (CR 502.3; ReplMayNotUntap, the
// keyword line "You may choose not to untap CARDNAME during your untap
// step."). gorge finishUntapStep walks the active player's battlefield in
// order and, at each tapped permanent carrying the keyword, asks its
// controller "untap" (option 0) or "keep_tapped" (option 1) before the untap
// event meets any replacement or stun counter; a kept permanent is left alone
// and the scan goes on. untapStepGen (repl_untapstep.go) suspends the same
// scan on that ask: the scan index, the slots untapped so far and the asked
// slot ride genState.untapNext, untapUp and untapSl, and the step runs again
// from the asked permanent once the answer is stored in untapAns.

// untapAsk suspends the untap step on the ask for the tapped permanent at
// slot sl, scan index i, with up the slots untapped before it.
func (s *State) untapAsk(i int, sl Slot, up slotMask) {
	s.g.untapOn, s.g.untapNext, s.g.untapSl, s.g.untapUp = true, uint8(i), sl, up
	s.g.ctlRet, s.ctl = ctlStepBegin, ctlGenAsk
}

// untapAskOptions poses the election: untap (mode 0) first, then keep tapped
// (mode 1), over the permanent, to its controller.
func (s *State) untapAskOptions(d *Decision) {
	p := &s.perms[s.g.untapSl]
	d.Kind, d.Sub, d.Player = DkChoose, ChGenKeepTapped, p.ctrl
	d.add(MakeAction(AkChoose, TCode(p.obj), 0, 0))
	d.add(MakeAction(AkChoose, TCode(p.obj), 0, 1))
}

// untapAnswer stores the answer and returns to the untap step, which resumes
// the scan at the asked permanent.
func (s *State) untapAnswer(a Action) {
	s.g.untapAns = 1
	if a.Mode() == 1 {
		s.g.untapAns = 2
	}
	s.ctl = s.g.ctlRet
}
