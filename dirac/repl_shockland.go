package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The shock lands' as-enters pay-life election: the entry
// replacement's program is UnlessPaid(PayLife N, You) then EnterTapped, so
// the permanent enters tapped unless its controller pays N life (CR 614.1c,
// 614.12, 119.4, 118.3). gorge poses the election through Resolve's generic
// unless gate for the entering permanent's controller (effects/unless.go
// poseUnlessAsk, ResumeKind "unless_pay"), and offers pay only while
// life >= N (engine/rules/unless_payment.go). The answer rides preAsk.val
// (1 paid, 0 declined) on the land-play and permanent-spell paths and the
// queued ask (#akPayLife) mid-resolution; no State byte is added.

// DebugShockFree makes the election offer pay at any life total.
// TestShockLockstep's mutation: it must diverge on a decline-only ask.
var DebugShockFree bool

// askRec.ts of a queued election.
const (
	askLandPlay    uint16 = 1 << iota // the answer completes a land play
	askEntryTapped                    // the permanent taps once the election is answered
	askUntapWins                      // a competing untap was applied after the election's tap (CR 616.1): declining taps nothing
)

// shock result of shockAt.
const (
	shockNo     = iota // not the shock election: the ordinary pay op runs
	shockApply         // applied from the answered pre-ask
	shockQueued        // posed as a queued as-enters ask (suspended)
)

// electionPC is the pc of an enters program's as-enters pay-life election:
// 0 for the shock lands' opener (an OpUnlessPaid PayLife, or the reveal
// election repl_revealland.go lowers), 1 behind an as-enters basic-land ask
// (the ask-then-elect chain, Multiversal Passage: the opener's answer queues
// the election, and the play's deferred count moves with it, answerQueuedAsk),
// or -1 when the program has no such election.
func electionPC(a *kop.Ability) int {
	if len(a.Prog) == 0 {
		return -1
	}
	switch a.Prog[0].Op {
	case kop.OpUnlessPaid:
		if int(a.Prog[0].C&0xF) == kop.PayLife || revealElection(&a.Prog[0]) {
			return 0
		}
	case kop.OpChooseType:
		if a.Prog[0].B.N == kop.ChooseTypeBasicLand &&
			len(a.Prog) > 1 && a.Prog[1].Op == kop.OpUnlessPaid && int(a.Prog[1].C&0xF) == kop.PayLife {
			return 1
		}
	case kop.OpChooseMove:
		// an as-enters hand reveal (Fortified Beachhead, Temple of the Dragon
		// Queen): its answer completes the land play, so the count waits
		// (repl_enterreveal.go)
		if a.Prog[0].C&7 == 0 && a.Prog[0].C>>3&kop.SrReveal != 0 {
			return 0
		}
	}
	return -1
}

// shockCardAmount finds card c's as-enters pay-life election amount.
func (s *State) shockCardAmount(k *prep, c CardID) int {
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersProgram {
			if ep := electionPC(a); ep >= 0 {
				if n := s.value(k, a.Prog[ep].A); n > 0 {
					return n
				}
			}
		}
	}
	return 0
}

// shockCard is the card whose enter program posed the pending akPayLife ask.
func (s *State) shockCard() CardID {
	if s.g.pre.kind == akPayLife {
		// state 1 (posed before a land play / spell resolution), preReturn
		// (a card entering from exile) or 0 (a token copy's ask)
		return s.objs[s.g.pre.obj].card
	}
	if s.g.nask > 0 && s.g.ask[0].kind == akPayLife {
		return s.g.ask[0].card
	}
	return NoCard
}

// shockOptions poses the election as the resolution unless ask encodes it
// (ChGenPayMode): decline always, pay only while the payer's life is at
// least N (CR 119.4).
func (s *State) shockOptions(k *prep, d *Decision) {
	n := s.shockCardAmount(k, s.shockCard())
	d.Sub = ChGenPayMode
	d.add(MakeAction(AkChoose, 0, 0, 0)) // decline
	if n > 0 && (DebugShockFree || int(s.pl[d.Player].life) >= n) {
		d.add(MakeAction(AkChoose, 0, 0, 1)) // pay N life
	}
}

// shockAt applies the election when the entering permanent's replacement
// program reaches its OpUnlessPaid: an answered pre-ask pays the life and
// skips the Tap body, or declines and lets it run. Without an answer (a
// permanent put onto the battlefield mid-resolution) the ask is queued at
// this instruction, as gorge defers it behind the suspended resolution. The
// election is the program's opener (the shock lands) or sits behind its
// as-enters basic-land ask (the ask-then-elect chain, electionPC).
func (s *State) shockAt(k *prep, sl Slot, c CardID, ab uint8, in *kop.Instr, pc int) (res, skip int) {
	if pc == 0 && revealElection(in) {
		s.queueElection(sl, akReveal, c, ab, 0) // repl_revealland.go
		return shockQueued, 0
	}
	if pc != electionPC(&k.t.Cards[c].Abilities[ab]) ||
		in.Op != kop.OpUnlessPaid || int(in.C&0xF) != kop.PayLife {
		return shockNo, 0
	}
	n := s.value(k, in.A)
	if n < 1 {
		return shockNo, 0
	}
	skip = int(in.C>>4) & 0x7ff
	ctrl := s.perms[sl].ctrl
	if s.g.pre.state == 2 && s.g.pre.kind == akPayLife && s.g.pre.obj == s.perms[sl].obj {
		paid := s.g.pre.val == 1
		s.g.pre.state = 0
		if paid && (DebugShockFree || int(s.pl[ctrl].life) >= n) {
			s.loseLife(k, ctrl, n) // the payment: lifeLost and its triggers (CR 118.3)
			return shockApply, skip
		}
		return shockApply, 0
	}
	s.queueElection(sl, akPayLife, c, ab, uint8(pc))
	return shockQueued, 0
}

// queueElection queues an as-enters election. A permanent an effect puts onto the
// battlefield tapped stays untapped until the answer: gorge runs the election
// inside the move and emits Tapped$ after it (engine/effects/zone_change.go).
func (s *State) queueElection(sl Slot, kind uint8, c CardID, ab, pc uint8) {
	n := s.g.nask
	s.queueAskAt(sl, kind, c, ab, pc)
	if s.g.nask > n && s.perms[sl].flags&pfTapped != 0 {
		s.perms[sl].flags &^= pfTapped
		s.g.ask[n].ts = askEntryTapped
	}
}

// shockLandCount marks the play's queued election (askLandPlay) so
// answerQueuedAsk takes the land count when the answer completes the play;
// a play that queued no ask (the ask table's capacity stall) takes it now.
// The ask-then-elect chain's opener ask carries the mark until its answer
// queues the election, which answerQueuedAsk moves it onto: the count waits
// for the whole entry's asks, as gorge's settleLandPlay does.
func (s *State) shockLandCount(k *prep, o Obj, p uint8) {
	if s.g.nask > 0 {
		a := &s.g.ask[s.g.nask-1]
		if a.sl == s.objs[o].slot && (a.kind == akPayLife || a.kind == akReveal || a.kind == akEnterReveal || a.kind == akEnterRevealPick || s.shockChainOpener(k, a)) {
			a.ts |= askLandPlay
			return
		}
	}
	s.pl[p].landsPlayed++
}

// shockChainOpener reports whether ask a is an as-enters choice that opens an
// ask-then-elect entry (electionPC 1): the election still ahead of its
// program, so the land play's deferred count is not yet due.
func (s *State) shockChainOpener(k *prep, a *askRec) bool {
	if a.card == NoCard || a.pc != 1 {
		return false
	}
	t := &k.t.Cards[a.card]
	if int(a.ab) >= len(t.Abilities) {
		return false
	}
	ab := &t.Abilities[a.ab]
	return ab.Kind == kop.AbReplacement && ab.Repl.Kind == kop.ReplEntersProgram && electionPC(ab) == 1
}

// shockAnswer applies the queued election's answer and runs the rest of the
// enter program: pay N life and skip skip instructions (the Tap body), or
// run them (the permanent enters tapped).
func (s *State) shockAnswer(k *prep, ask askRec, a Action) {
	prog := k.t.Cards[ask.card].Abilities[ask.ab].Prog
	pc := int(ask.pc)
	if pc >= len(prog) {
		return
	}
	in := &prog[pc]
	n := s.value(k, in.A)
	skip := int(in.C>>4) & 0x7ff
	next := pc + 1
	ctrl := s.perms[ask.sl].ctrl
	if a.Mode() == 1 && n > 0 && (DebugShockFree || int(s.pl[ctrl].life) >= n) {
		s.loseLife(k, ctrl, n)
		next += skip
	} else if ask.ts&askUntapWins != 0 {
		next += skip // the tap body ran before the untap that wins: nothing taps
	}
	saveRep := s.g.replaced
	s.g.replaced = s.perms[ask.sl].obj
	s.runReplProg(k, ask.sl, ask.card, uint8(ask.ab), uint8(next))
	s.g.replaced = saveRep
}

// shockUntapWins marks the permanent at sl's queued election: a body that
// untaps it was applied after the election's tap (CR 616.1, the order the
// affected player chose), so the answer taps nothing, though it is still asked.
func (s *State) shockUntapWins(sl Slot) {
	for i := 0; i < int(s.g.nask); i++ {
		if a := &s.g.ask[i]; a.kind == akPayLife && a.sl == sl {
			a.ts |= askUntapWins
		}
	}
}

// shockTapWins undoes shockUntapWins for the permanent at sl's queued election:
// the late re-ask (CR 616.1f) left the election's own tap as the body applied
// last, so a decline taps it after all.
func (s *State) shockTapWins(sl Slot) {
	for i := 0; i < int(s.g.nask); i++ {
		if a := &s.g.ask[i]; a.kind == akPayLife && a.sl == sl {
			a.ts &^= askUntapWins
		}
	}
}

// shockEntryOrdered: the permanent at sl entered under an order ask (CR 616.1)
// with a Tapped$ move: the move's tapped bit is set before the bodies, so a
// body the order puts first (an untap) clears it, and a tap applies it again
// through the election's own Tap body. The election no longer taps after its
// answer.
func (s *State) shockEntryOrdered(sl Slot) {
	for i := 0; i < int(s.g.nask); i++ {
		if a := &s.g.ask[i]; a.kind == akPayLife && a.sl == sl {
			a.ts &^= askEntryTapped
		}
	}
}
