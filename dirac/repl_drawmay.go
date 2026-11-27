package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Optional$ True Draw replacements (kop.ReplDrawOptional, the Obstinate
// Familiar shape): where exactly ONE may replacement matches a draw, gorge
// parks the Draw event and asks its player "Apply CARDNAME's optional draw
// replacement?" (engine/rules/replacement_choice.go replChoiceDraw, options
// apply / decline). The kernel poses the same election (ChGenDrawMay, yes
// canonically ActYes and no ActNo with the replacement's object, canon.go's
// KReplacement apply / decline) as the raGenDrawMay ask -- the kernel side of
// gorge's effects.AskTape at the draw.
//
// An accept runs the replacement body (drawProgramTake: the allowlist program
// or the bodyless Prevent$ skip); a decline draws the card through
// drawExcl with the declined replacement in the draw's lineage, so the same
// replacement cannot catch its own decline's draw (CR 614.5) while the other
// replacements still meet it. The ask is posed at the sites the CR 616.1
// order ask (repl_draworder.go) is: drawOrderSite is the one home of "this
// draw can be suspended", and the continuation is the order ask's own
// (res.left, g.pickZ/pickLeft at an OpDraw card; ctlGenAsk at the turn
// draw), finished by the same drawAskTail. Every other site -- a draw
// inside a lineage, a replacement program's own draw, an effect draw outside
// OpDraw -- has no frame to resume, so a lone may candidate there stalls
// (StallDrawRepl) rather than guess gorge's order. Two or more candidates
// pose the order ask first; a picked may candidate then poses this election
// (gorge scheduleSelectedOptionalDraw promotes the pick to its own
// apply / decline).

// ChGenDrawMay is the Optional$ True Draw replacement election (gorge
// replChoiceDraw, engine/rules/replacement_choice.go): a KReplacement of
// apply (mode 1) / decline (mode 0), the option source the replacement
// permanent.
const ChGenDrawMay ChoiceKind = 218

// drawReplAsked reports the draw just made posed a replacement ask (the
// order ask or the may election): the draw's site suspends on it.
func (s *State) drawReplAsked() bool {
	return s.res.ask == raGenDrawOrder || s.res.ask == raGenDrawMay
}

// drawMayPose parks the may candidate sel/abi on coldDraw and poses the
// election at a resumable site (drawOrderSite). False: no site, the caller
// stalls.
func (s *State) drawMayPose(k *prep, p uint8, sel Slot, abi uint8, ch drawChain) bool {
	if ch.n != 0 || p >= MaxSeats {
		return false // a lineage's card has no resumable frame here (CR 616.1e)
	}
	ds := s.drawState()
	if len(ds) == 0 || ds[0].active || ds[0].may {
		return false // the program's own draw, or an ask already parked
	}
	op, turn := s.drawOrderSite(k, p)
	if !op && !turn {
		return false
	}
	ds[0] = drawProgram{may: true, p: p, sel: sel, abi: abi}
	s.res.ask, s.res.askP = raGenDrawMay, p
	if turn {
		s.g.ctlRet, s.ctl = ctlStepBegin, ctlGenAsk
	}
	return true
}

// drawMayOptions poses gorge's apply / decline election (the ChGenDrawMay
// yes-first option list, both options naming the replacement permanent).
//
//kmanifest:askopts raGenDrawMay
func (s *State) drawMayOptions(k *prep, d *Decision) {
	ds := s.drawState()
	if len(ds) == 0 {
		return
	}
	r := &ds[0]
	d.Kind, d.Sub, d.Player = DkChoose, ChGenDrawMay, r.p
	o := s.perms[r.sel].obj
	d.add(MakeAction(AkChoose, TCode(o), 0, 1)) // yes: apply the replacement
	d.add(MakeAction(AkChoose, TCode(o), 0, 0)) // no: the ordinary draw
}

// drawMayAnswer applies the election. Accept runs the body in the card's
// place (drawProgramTake: the chain, the skip and the ask-live record are
// its own); decline draws the card with the declined replacement in the
// lineage. Then the site's tail runs (drawAskTail).
//
//kmanifest:askapply raGenDrawMay
func (s *State) drawMayAnswer(k *prep, a Action) {
	ds := s.drawState()
	if len(ds) == 0 || !ds[0].may {
		s.res.ask = raNone
		return
	}
	d := ds[0]
	ds[0] = drawProgram{} // the answer's own effects run unposed
	s.res.ask = raNone
	turn := !s.res.active
	if turn {
		s.ctl = ctlStepBegin
	}
	left, rest, n := int(s.res.left), s.g.pickZ, int(s.g.pickLeft)
	n0 := s.npend
	p, sel, abi := d.p, d.sel, d.abi
	if a.Mode() == 1 {
		ab := &k.t.Cards[s.cardAt(sel)].Abilities[abi]
		s.drawProgramTake(k, p, sel, abi, ab, drawChain{})
	} else {
		ch := drawChain{n: 1}
		ch.sl[0] = sel            // CR 614.5: the declined replacement does not meet its own decline's draw
		s.cfgx |= cfgxDredgeAsked // the dredge ask already ran for this card (its decline drew plainly)
		s.drawExcl(p, ch)
		s.cfgx &^= cfgxDredgeAsked
	}
	if ds := s.drawState(); s.res.ask != raNone || len(ds) > 0 && ds[0].active {
		// the body posed or queued an ask and its frame is live: that answer
		// resumes the body, not this site's tail, which no frame carries
		// past it, so declare the gap rather than run the tail under it
		s.capFull(StallDrawRepl)
		return
	}
	s.drawAskTail(k, turn, p, left, rest, n, n0)
}

// drawAskTail carries a draw replacement ask's site on once its answer has
// replaced (or made) the draw: the turn draw finishes the draw step, the
// step's own triggers ahead of the draw's (n0: the pending count before the
// answer); an OpDraw card resumes its loop past the asked card.
func (s *State) drawAskTail(k *prep, turn bool, p uint8, left int, rest uint8, n int, n0 uint8) {
	if turn {
		s.g.drawFirst = false
		if s.over {
			return
		}
		n1 := s.npend
		s.firePhase(k, kop.StepDraw)
		s.pendFirst(n0, n1) // the step's own triggers ahead of the draw's
		s.openPriority()
		return
	}
	in := &s.prog(k)[s.res.pc]
	if s.dredgeDrawLoop(k, in, p, left-1, rest, n) {
		s.continueRes(k, 0)
	}
}
