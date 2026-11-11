package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The as-enters copy, mirroring gorge's ETB
// clone (rules/cast_etbchoice.go collectETBChoices kind "copy" and
// etbOptions, effects/clone.go effClone with CloneEnter.ETB): while a cast
// permanent spell whose enters replacement opens with OpBecomeCopy is about
// to resolve, its controller is asked which battlefield permanent matching
// the op's filter it enters as a copy of (seat 0's battlefield, then seat
// 1's, in entry order), then "enter as itself" when the keyword line is
// :Optional or nothing matches. The pick rides s.g.pre like the colour and
// type asks; as the permanent enters, OpBecomeCopy makes it a copy: the
// override's copy-of card (cardAt) is the template's copiable card (its own
// copy, if any), the template's copy exceptions are inherited
// (copyExcInherit), and the op's own exceptions (flying, one added
// subtype) are applied. The permanent's trigger and static subscriptions
// move to the copied card before the ETB triggers fire. A return to the
// battlefield during a resolution (a reanimation, a blink) asks the same
// election with the card still in its zone (copy_cloneenter.go). A
// token copy of a clone card (a Feldon copy of the graveyard Clone, a Cackling
// Counterpart copy of a declined one) is asked the same election as it is
// minted (raGenTokChoice, gorge applies the answer); there
// is no spell, so the filter reads no mana spent.
//
// Not modelled: a choice among the copied card's as-enters replacements
// is asked after the copy entered (the queued ask), not with the clone
// choice on the stack; flying as a copiable exception of a copy OF the
// clone (a token copier inherits haste, 1/1 and the subtype only).
//
// Every function here is cold and out of line: the sites are the
// as-enters ask (preAskFor, askOptions, applyGen), the replacement program
// walk (runReplProgAs) and the cast's history fold (histCast, behind
// genPrep.anySpent).

// cloneAskKind is akClone when card c's enters replacement opens with
// OpBecomeCopy, else 0.
//
//go:noinline
func (s *State) cloneAskKind(k *prep, c CardID) uint8 {
	if s.cloneOp(k, c) != nil {
		return akClone
	}
	return 0
}

// cloneOp is card c's OpBecomeCopy (the head of an enters replacement
// program), or nil.
func (s *State) cloneOp(k *prep, c CardID) *kop.Instr {
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersProgram && len(a.Prog) > 0 && a.Prog[0].Op == kop.OpBecomeCopy {
			return &a.Prog[0]
		}
	}
	return nil
}

// cloneAnswer is the s.g.pre value of a clone answer: the chosen object,
// or NoObj for "enter as itself".
func cloneAnswer(a Action) uint8 {
	if a.Mode() != 0 {
		return uint8(NoObj)
	}
	return a.Source()
}

// cloneOptions fills the clone ask for the spell s.g.pre.obj on top of the
// stack.
//
//go:noinline
func (s *State) cloneOptions(k *prep, d *Decision) {
	d.Sub = ChGenClone
	o := s.g.pre.obj
	in := s.cloneOp(k, s.objs[o].card)
	if in == nil {
		d.add(MakeAction(AkChoose, uint8(o), 0, 1))
		return
	}
	// the filter's bounds read the spell as the resolving entry (its mana
	// spent, ExCastManaSpent)
	saved := s.res
	for i := int(s.sp) - 1; i >= 0; i-- {
		if s.stack[i].kind == seSpell && s.stack[i].obj == o {
			s.res.e = s.stack[i]
			break
		}
	}
	s.res.active = true
	if s.res.ask == raGenTokChoice || s.res.ask == raGenRetChoice {
		s.res.e.trigAmt, s.res.e.x, s.res.e.spc, s.res.e.etbSrc = 0, 0, 0, 0 // a minted token or a returned card was never cast: no mana spent, no colours (gorge reads 0)
	}
	ctrl := s.res.e.ctrl
	if s.res.ask == raGenRetChoice || s.res.ask == raGenTokChoice {
		ctrl = s.res.askP // a returned card's or a copy token's controller, not the resolving spell's
	}
	var ord [MaxPerm]Slot
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		n := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), ord[:], 0)
		for _, sl := range ord[:n] {
			t := s.perms[sl].obj
			if s.objs[t].of&ofPending != 0 {
				continue
			}
			if s.matchPermGen(k, in.F, sl, ctrl, NoSlot, layerFull) {
				d.add(MakeAction(AkChoose, uint8(t), 0, 0))
			}
		}
	}
	s.res = saved
	if in.C&1 != 0 || d.N == 0 {
		d.add(MakeAction(AkChoose, uint8(o), 0, 1)) // "enter as itself"
	}
}

// becomeCopyEnter is OpBecomeCopy as the permanent at sl enters: it
// becomes a copy of the permanent chosen on the stack, if any.
//
//go:noinline
func (s *State) becomeCopyEnter(k *prep, sl Slot, in *kop.Instr) {
	p := &s.perms[sl]
	if s.g.pre.state != 2 || s.g.pre.obj != p.obj || s.g.pre.kind != akClone {
		return // no election was asked: it enters as itself
	}
	s.g.pre.state = 0
	t := Obj(s.g.pre.val)
	if t == NoObj || s.objs[t].zone != kop.ZoneBattlefield {
		return
	}
	if s.g.pre.ask2 != 0 {
		// the template's choice was answered on the stack: its enters
		// program takes it below (runReplProgAs)
		s.g.pre.val, s.g.pre.state, s.g.pre.ask2 = s.g.pre.val2, 2, 0
	}
	src := s.objs[t].slot
	c := s.cardAt(src)
	s.touchSl(sl)
	s.subscribe(sl, &k.cards[s.cardAt(sl)], false)
	p.ovr.flags |= ovCopy
	p.ovr.copyOf = c
	s.subscribe(sl, &k.cards[c], true)
	// the template's copy exceptions, then the op's own (CR 707.9b)
	s.copyExcAdd(k, sl, src, uint8(in.C>>1))
	s.copyGrantsAdd(k, sl, src)
	if in.A.K == kop.VConst && in.A.N > 0 {
		// AddAbilities$ (Mercurial Pretender): a copiable value (CR 707.9b),
		// from the entering card's own ability table, for as long as the copy
		// stays (kgen lower_cloneentergrant.go; grantCardAt)
		s.applyOverrideFl(k, sl, int(in.A.N)-1, p.card, ovCopyGrant)
	}
	if k.gen.anyCloneCopy {
		s.clonePermNote(sl, t, c, uint8(in.C>>1), s.tick()) // the marker a later cleanup settle re-bases from
	}
	// the copied card's own enters replacements apply (CR 707.5; gorge's
	// entry plan reads the clone template): enters tapped, with counters,
	// and its enters programs (a choice among them is asked once it is on
	// the battlefield, the queued as-enters ask)
	pc := &k.cards[c]
	if pc.enterTap {
		p.flags |= pfTapped
	}
	if pc.enterCtr != 0 && !(k.gen.anyEntryMerge && s.entryAcc(int(pc.enterCK), int(pc.enterCtr))) {
		s.placeEntryCounters(k, sl, int(pc.enterCK), int(pc.enterCtr), p.ctrl)
	}
	if in.B.K == kop.VConst && in.B.N&kop.BecomeCopyCtr != 0 {
		// RememberCloneOrigin$ True + EachFromSource$ Remembered (Dominion
		// Saboteur): the copy carries the origin's counters as it enters,
		// before any enters trigger matches (and on top of the template's own
		// printed counters just placed, CR 707.5).
		s.copyEnteredCounters(k, sl, src)
	}
	if k.gen.has(c, gfEnterRepl) {
		cd := &k.t.Cards[c]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram || len(a.Prog) == 0 || a.Prog[0].Op == kop.OpBecomeCopy {
				continue
			}
			ctx := evalCtx{src: p.obj, srcSl: sl, ctrl: p.ctrl}
			if s.res.active {
				ctx.e = &s.res.e
			}
			if a.Repl.Cond == 0 || s.condEval(k, a.Repl.Cond, ctx) {
				s.runReplProg(k, sl, c, uint8(i), 0)
			}
		}
	}
}

// recordSpent records the mana spent to cast the spell o in its stack
// entry (gorge FlagManaSpent's Object.ManaSpent): a spell entry's
// trigger amount is otherwise unused.
//
//go:noinline
func (s *State) recordSpent(o Obj, spent int) {
	for i := int(s.sp) - 1; i >= 0; i-- {
		if e := &s.stack[i]; e.kind == seSpell && e.obj == o {
			e.trigAmt = int16(spent)
			return
		}
	}
}
