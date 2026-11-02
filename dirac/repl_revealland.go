package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The reveal lands' as-enters election (Choked Estuary, Gilt-Leaf Palace): the
// entry replacement's program is UnlessPaid(Reveal 1, You) then EnterTapped, so
// the land enters tapped unless its controller reveals a matching card from
// their hand (CR 614.12, 701.20a). gorge poses it as the shock lands' election
// (effects/unless.go poseUnlessAsk), offering pay only with a candidate, then
// rules/pay/unlessflow.go AdvanceUnless takes a lone candidate unasked and asks
// "revealcost" over several. The card stays in hand: nothing but the Tap body
// depends on the answer.

// ChCostReveal is gorge's "revealcost" KChoose: source = the card revealed.
const ChCostReveal ChoiceKind = 212

func revealElection(in *kop.Instr) bool {
	return in.Op == kop.OpUnlessPaid && int(in.C&0xF) == kop.PayDiscard && in.B.N == kop.PayBReveal
}

// revealFilter is the filter of the election at the queue's head.
func (s *State) revealFilter(k *prep) kop.FilterID {
	a := &s.g.ask[0]
	return k.t.Cards[a.card].Abilities[a.ab].Prog[a.pc].F
}

// revealCands counts player p's hand cards matching f, adding each to d when d
// is set.
func (s *State) revealCands(k *prep, p uint8, f kop.FilterID, d *Decision) int {
	pl := &s.pl[p]
	n := 0
	for i := 0; i < int(pl.handN); i++ {
		if o := pl.hand[i]; s.matchCardGen(k, f, s.objs[o].card, p, p, o) {
			n++
			if d != nil {
				d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			}
		}
	}
	return n
}

// revealOptions poses the queue head's election (encoded as ChGenPayMode, with
// pay only while a candidate exists) or its pick.
func (s *State) revealOptions(k *prep, d *Decision, kind uint8) {
	f := s.revealFilter(k)
	if kind == akRevPick {
		d.Sub = ChCostReveal
		s.revealCands(k, d.Player, f, d)
		return
	}
	d.Sub = ChGenPayMode
	d.add(MakeAction(AkChoose, 0, 0, 0)) // decline
	if s.revealCands(k, d.Player, f, nil) > 0 {
		d.add(MakeAction(AkChoose, 0, 0, 1)) // reveal
	}
}

// revealAnswer applies an answer of the popped ask, and runs the rest of the
// enter program: the Tap body on a decline, past it on a reveal. Paying with
// several candidates queues the pick in the election's place instead and
// reports false.
func (s *State) revealAnswer(k *prep, ask askRec, a Action) bool {
	prog := k.t.Cards[ask.card].Abilities[ask.ab].Prog
	in := &prog[ask.pc]
	next := int(ask.pc) + 1
	if ask.kind == akRevPick || a.Mode() == 1 {
		if ask.kind == akReveal && s.revealCands(k, s.perms[ask.sl].ctrl, in.F, nil) > 1 {
			copy(s.g.ask[1:s.g.nask+1], s.g.ask[:s.g.nask])
			s.g.nask++
			ask.kind = akRevPick
			s.g.ask[0] = ask
			return false
		}
		next += int(in.C>>4) & 0x7ff
	}
	saveRep := s.g.replaced
	s.g.replaced = s.perms[ask.sl].obj
	s.runReplProg(k, ask.sl, ask.card, ask.ab, uint8(next))
	s.g.replaced = saveRep
	return true
}
