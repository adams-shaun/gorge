package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The as-enters hand reveal (Reveal | RevealValid$ ... | [AnyNumber$/Optional$]
// | RememberRevealed$, kgen lower_reveal.go): gorge runs effReveal in the
// replacement body, an Optional$ reveal_optional yes/no first when a matching
// card qualifies, then a ChGenReveal pick over the matching hand cards
// (min 0 with AnyNumber$, else one). The kernel runs the same decisions when
// the enters program reaches OpChooseMove with SrReveal and a zone-none
// destination. A picked card joins the entry's remembered set (SrRemember),
// which the next instruction's RememberedCount reads (a counter amount or the
// tap gate), and Cleanup ClearRemembered$ ends it.
//
// The pick is posed one answer at a time, as the generic search ask is; the
// Done option is offered only where the reveal may stop (AnyNumber$, no
// SrMandatory). A pool no larger than the mandatory count is taken unasked,
// and an other replacement with no resolution run keeps gorge's no-host
// fallback (the first matching cards).

// enterRevealIn is the OpChooseMove an entry-reveal ask is answering.
func enterRevealIn(k *prep, a *askRec) *kop.Instr {
	return &k.t.Cards[a.card].Abilities[a.ab].Prog[a.pc-1]
}

// enterRevealCands counts (and, when d is set, lists) the chooser's hand cards
// matching in, skipping cards this reveal already picked (they are in the
// entry's remembered set).
func (s *State) enterRevealCands(k *prep, in *kop.Instr, p uint8, d *Decision) int {
	pl := &s.pl[p]
	n := 0
	for _, o := range pl.hand[:pl.handN] {
		if s.inSet(&s.g.rem, s.g.nrem, o) {
			continue
		}
		if !s.matchCardGen(k, in.F, s.objs[o].card, p, p, o) {
			continue
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
	return n
}

// enterRevealPickable reports whether the reveal poses a pick: the matching
// pool is larger than the count it must show, or AnyNumber$ (min 0).
func (s *State) enterRevealPickable(k *prep, in *kop.Instr, n int) bool {
	if in.C>>3&kop.SrMandatory == 0 {
		return true
	}
	return int(s.value(k, in.A)) < n
}

// enterRevealRemember remembers every matching hand card: the unasked reveal
// of a pool no larger than its count.
func (s *State) enterRevealRemember(k *prep, in *kop.Instr, p uint8) {
	s.enterRevealRememberFirst(k, in, p, -1)
}

// enterRevealRememberFirst remembers the first want matching hand cards in
// hand order (-1: all).
func (s *State) enterRevealRememberFirst(k *prep, in *kop.Instr, p uint8, want int) {
	if in.C>>3&kop.SrRemember == 0 {
		return
	}
	pl := &s.pl[p]
	n := 0
	for _, o := range pl.hand[:pl.handN] {
		if s.inSet(&s.g.rem, s.g.nrem, o) || !s.matchCardGen(k, in.F, s.objs[o].card, p, p, o) {
			continue
		}
		if want >= 0 && n >= want {
			return
		}
		s.remember(o)
		n++
	}
}

// enterRevealStart is runReplProgAs's OpChooseMove arm: true when it posed an
// ask and the program must wait.
func (s *State) enterRevealStart(k *prep, sl Slot, c CardID, ab, pc uint8, in *kop.Instr) bool {
	if in.C&7 != 0 || in.C>>3&kop.SrReveal == 0 {
		return false
	}
	p := s.perms[sl].ctrl
	n := s.enterRevealCands(k, in, p, nil)
	if n == 0 {
		return false
	}
	if s.entryNoRun() {
		// gorge's no-host fallback: an optional reveal reveals, and the pick
		// takes the first cards it would have offered (one for AnyNumber$,
		// whose amount defaults to 1 when nobody answers)
		want := 1
		if v := int(s.value(k, in.A)); v > 0 && v < n {
			want = v
		}
		s.enterRevealRememberFirst(k, in, p, want)
		return false
	}
	if in.C>>3&kop.SrConfirm != 0 {
		s.queueAskAt(sl, akEnterReveal, c, ab, pc+1)
		return true
	}
	if s.enterRevealPickable(k, in, n) {
		s.queueAskAt(sl, akEnterRevealPick, c, ab, pc+1)
		return true
	}
	s.enterRevealRemember(k, in, p)
	return false
}

// enterRevealOptions builds the yes/no or the hand pick.
func (s *State) enterRevealOptions(k *prep, d *Decision, kind uint8) {
	a := &s.g.ask[0]
	s.enterRevealOptionsIn(k, d, kind, enterRevealIn(k, a), a.sl)
}

// enterRevealOptionsIn is enterRevealOptions over an op at hand (the no-run
// path builds from the instruction, not the queue).
func (s *State) enterRevealOptionsIn(k *prep, d *Decision, kind uint8, in *kop.Instr, sl Slot) {
	p := s.perms[sl].ctrl
	if kind == akEnterReveal {
		d.Sub = ChGenConfirm
		d.add(MakeAction(AkChoose, 0, 0, 0)) // no
		d.add(MakeAction(AkChoose, 0, 0, 1)) // yes
		return
	}
	d.Sub = ChGenReveal
	s.enterRevealCands(k, in, p, d)
	if in.C>>3&kop.SrMandatory == 0 {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
}

// enterRevealAnswer applies one answer; false keeps the ask queued.
func (s *State) enterRevealAnswer(k *prep, ask askRec, a Action) bool {
	in := enterRevealIn(k, &ask)
	p := s.perms[ask.sl].ctrl
	if ask.kind == akEnterReveal {
		if a.Mode() == 0 {
			return true // declined: nothing revealed, nothing remembered
		}
		n := s.enterRevealCands(k, in, p, nil)
		if n == 0 || !s.enterRevealPickable(k, in, n) {
			s.enterRevealRemember(k, in, p)
			return true
		}
		s.requeueEnterReveal(ask, akEnterRevealPick)
		return false
	}
	if a.Kind() == AkDone {
		return true
	}
	o := Obj(a.Source())
	if in.C>>3&kop.SrRemember != 0 {
		s.remember(o)
	}
	if in.C>>3&kop.SrMandatory == 0 && s.enterRevealCands(k, in, p, nil) != 0 {
		s.requeueEnterReveal(ask, ask.kind)
		return false // AnyNumber$: the next pick
	}
	return true
}

// requeueEnterReveal puts the popped reveal ask back at the queue's head as
// kind; a full queue is a declared stall.
func (s *State) requeueEnterReveal(ask askRec, kind uint8) {
	if int(s.g.nask) >= len(s.g.ask) {
		s.capFull(StallAsk)
		return
	}
	copy(s.g.ask[1:s.g.nask+1], s.g.ask[:s.g.nask])
	s.g.nask++
	ask.kind = kind
	s.g.ask[0] = ask
}

// replAskOp is runReplProgAs' OpChooseBranch and OpChooseMove: the siege's
// mode (face_siegemode.go) and the as-enters Reveal (enterRevealStart) queue
// their ask and report true, the program stopping there; any other form runs
// in place.
func (s *State) replAskOp(k *prep, sl Slot, c CardID, ab uint8, i int, in *kop.Instr) bool {
	if in.Op == kop.OpChooseBranch {
		if in.C != 2 {
			s.execGen(k, in)
			return false
		}
		if s.g.pre.state == 2 && s.g.pre.kind == akMode && s.g.pre.obj == s.perms[sl].obj {
			s.siegeModeStore(sl, uint8(s.g.pre.val)) // asked in exile as the card returned (face_siegemode.go returnAskKind)
			return false
		}
		s.siegeModeRepl(sl, c, ab, i)
		return true
	}
	if in.C&7 == 0 && in.C>>3&kop.SrReveal != 0 {
		return s.enterRevealStart(k, sl, c, ab, uint8(i), in)
	}
	s.execGen(k, in)
	return false
}
