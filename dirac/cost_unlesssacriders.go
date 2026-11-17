package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An UnlessCost$ whose Sac<N/Spec> part rides beside PayLife<L> and/or
// Draw<D/Who> (Tresserhorn's Lord, Returned): one PaySac instruction whose B.N
// packs L and D (kop.PaySacRidersB) and whose W is the drawing player. gorge
// poses ONE pay / decline election, asks the Sac part's picks, then charges
// life, the sacrifices and the draws in that order (pay/unlessflow.go
// AdvanceUnless; rules/unless_payment.go settleUnlessStep draws after the
// payment). The kernel runs the same sequence on its ordinary PaySac flow:
//
//	offer     canPayGen's PaySac arm adds sacRidersPayable (life >= L, D > 0)
//	picks     the ordinary PaySac picks, unchanged
//	life      payRidersLife, ahead of the sacrifices' commit (payCommitSacs,
//	          and autoPayPick's own commit when exactly N candidates exist)
//	draws     payRidersDraw, from payDone's paid branch, after the commit

// sacRidersAt returns the riders of the OpUnlessPaid / OpUnlessSwitched being
// resolved (Tresserhorn's script is UnlessSwitched$ True); ok is false for any
// other instruction (a lone Sac, a cumulative or trigger cost).
func (s *State) sacRidersAt(k *prep) (in *kop.Instr, life, draw int, ok bool) {
	if !s.res.active {
		return nil, 0, 0, false
	}
	prog := s.prog(k)
	if int(s.res.pc) >= len(prog) {
		return nil, 0, 0, false
	}
	in = &prog[s.res.pc]
	if in.Op != kop.OpUnlessPaid && in.Op != kop.OpUnlessSwitched || in.C&0xF != kop.PaySac || !kop.IsPaySacRidersB(in.B.N) {
		return nil, 0, 0, false
	}
	life, draw = kop.PaySacRidersFrom(in.B.N)
	return in, life, draw, true
}

// sacRidersPayable is the offer gate's rider half (gorge unlessComponentsPayable
// prices every part together): the payer holds the life, and a draw part names
// a positive count. b is the instruction's B.N; any other value has no riders.
func (s *State) sacRidersPayable(p uint8, b int16) bool {
	if !kop.IsPaySacRidersB(b) {
		return true
	}
	life, _ := kop.PaySacRidersFrom(b)
	return int(s.pl[p].life) >= life
}

// payRidersLife charges the life part once the Sac picks are settled and
// before any sacrifice is committed (gorge's PayManaConv precedes
// PaySacrificeCost).
func (s *State) payRidersLife(k *prep) {
	if _, life, _, ok := s.sacRidersAt(k); ok && life > 0 {
		s.loseLife(k, s.g.pickP, life)
	}
}

// payRidersDraw draws the draw part after the sacrifices are committed: the
// instruction's W player, the payer when W names none.
func (s *State) payRidersDraw(k *prep) {
	in, _, draw, ok := s.sacRidersAt(k)
	if !ok || draw == 0 {
		return
	}
	dp := s.g.pickP
	if in.W != kop.RefNone {
		dp = s.payerOf(k, in.W)
	}
	s.payDraw(dp, draw)
}
