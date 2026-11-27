package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// repeatEachCards is OpRepeatEach, gorge's api:RepeatEach over cards
// (RepeatCards$ <filter> with Zone$ Battlefield, or DefinedCards$ Targeted;
// effects/repeateach.go repeatedCards, effRepeatEach). The subjects are listed
// once, before the first pass, as gorge lists them: for a filter (Obj
// RefNone) every battlefield permanent F admits, the resolving controller's
// first and then each other seat's, each in battlefield order; for a target
// slot (Obj RefTarget<i>) the slot's legal objects in pick order. The next
// B.N instructions then run once per subject with RefEach bound to it
// (resCtx.eachObj; gorge binds the subject as Remembered), and the resolution
// goes on after them.
//
// The plain loop (C kop.RepeatPlain) is a resumable frame
// (repeatEachCardsFrame), so its body may ask. The batched loop's body is run
// here, inline, in one call (the way execUnion runs one instruction over its
// objects), so that loop needs no State of its own: the frame is the Go stack
// and eachObj, which a resolution starts afresh. A body instruction that asks
// (exec false) cannot be resumed from inside it, so it stalls loudly; kgen
// lowers only a Role mint there, which asks nothing (createAttachedToken), and
// an Aura-token replacement that would ask stalls the mint itself
// (StallRoleMulti).
//
// ChangeZoneTable$ True: the zone changes of every pass reach the batch
// listeners ("one or more ...") once, after the last pass, so the loop runs
// inside one batch and one entry hold, as execUnion's does. Without it (C
// kop.RepeatPlain) gorge opens no zone batch around the loop, so neither is
// opened here: each pass's body is its own (Balan, Wandering Knight).
//
//go:noinline
func (s *State) repeatEachCards(k *prep, in *kop.Instr) {
	var list [MaxPerm]Obj
	n := s.eachCardList(k, in, &list)
	body := int(in.B.N)
	r := &s.res
	pc := int(r.pc)
	if n > 0 && body > 0 {
		if _, tgt := tgtSlot(in.Obj); in.C&kop.RepeatPlain != 0 && !tgt && !s.g.inLoop && r.loopD < uint8(len(r.loopN)) {
			s.repeatEachCardsFrame(in, &list, n, pc, body) // the caller's step moves pc onto the body
			return
		}
	}
	r.pc += uint8(body) // the header's own step is the caller's
	if n == 0 || body == 0 {
		return
	}
	prog := s.prog(k)
	if in.C&kop.RepeatPlain == 0 {
		if k.gen.anyBatch {
			defer s.batchClose(s.batchOpen())
		}
		hr := s.holdOpen(k)
		defer s.holdClose(k, hr)
	}
	saveLoop := s.g.inLoop
	s.g.inLoop = true
	for i := 0; i < n && !s.over; i++ {
		r.eachObj = list[i]
		for j := 1; j <= body; j++ {
			if !s.exec(k, &prog[pc+j]) {
				s.capFull(StallRoleMulti) // a body that asks: not resumable from here
				break
			}
		}
	}
	r.eachObj = NoObj
	s.g.inLoop = saveLoop
}

// repeatEachCardsFrame runs the plain loop (C kop.RepeatPlain) as a resumable
// frame of the resolution's loop stack (resCtx.loopS/loopPC, the machinery of
// OpRepeatWhile), not inline, so a body instruction that asks (Cleansing's
// "destroy that land unless any player pays 1 life", asked each pass)
// suspends the resolution and the answer carries the loop on: loopBack,
// reaching the body's end, binds the next subject (eachCardAdvance) and jumps
// back to the body's first instruction. The subjects after the first are held
// as a slot mask in armRes.eachRem, in the order eachCardList listed them
// (battlefield order, seats from the controller): a subject that has left the
// battlefield by its turn is skipped. The caller's step moves pc to the
// body's first instruction.
func (s *State) repeatEachCardsFrame(in *kop.Instr, list *[MaxPerm]Obj, n, pc, body int) {
	r := &s.res
	rem := &s.resArm().eachRem
	*rem = slotMask{}
	if in.C&kop.RepeatRemembered != 0 {
		s.eachRememberedMask(rem) // the subjects after the first, as g.rem indices
	} else {
		for _, o := range list[1:n] {
			*rem = rem.with(s.objs[o].slot)
		}
	}
	if in.C&kop.RepeatDmgBatch != 0 {
		s.dmgBatchOpen() // closed by eachCardClose, behind the last pass
	}
	d := r.loopD
	r.loopN[d], r.loopS[d], r.loopPC[d] = 1, uint8(pc+1), uint8(pc+1+body)
	r.loopD++
	r.eachObj = list[0]
	s.g.inLoop = true
}

// eachCardAdvance binds the plain loop's next subject, the first of the
// remaining ones in eachCardList's order that is still on the battlefield, and
// reports whether there was one (loopBack; repeatEachCardsFrame). A
// remembered-set loop (in.C kop.RepeatRemembered) walks g.rem instead: the
// next index still in the mask, bound whether or not the card is on the
// battlefield.
func (s *State) eachCardAdvance(in *kop.Instr) bool {
	rem := &s.resArm().eachRem
	if in.C&kop.RepeatRemembered != 0 {
		if !rem.any() {
			return false
		}
		i := rem.first()
		*rem = rem.without(i)
		s.res.eachObj = s.g.rem[i]
		return true
	}
	q := s.aliveFrom(s.res.e.ctrl)
	var ord [MaxPerm]Slot
	for i := uint8(0); i < q.n; i++ {
		p := q.s[i]
		hit := rem.and(s.occ).and(s.ctrlM[p])
		if n := s.battlefieldOrder(p, hit, ord[:], 0); n > 0 {
			*rem = rem.without(ord[0])
			s.res.eachObj = s.perms[ord[0]].obj
			return true
		}
	}
	return false
}

// eachRememberedMask sets rem to the remembered-set loop's subjects after the
// first, as indices of g.rem: every remembered object but the first one, in
// remembered order (eachCardList lists the same objects).
func (s *State) eachRememberedMask(rem *slotMask) {
	first := true
	for i, o := range s.g.rem[:s.g.nrem] {
		if o == NoObj {
			continue
		}
		if first {
			first = false
			continue
		}
		*rem = rem.with(Slot(i))
	}
}

// eachCardClose ends the plain loop whose header is in, once its last pass is
// done: the loop's damage batch (DamageMap$ True, kop.RepeatDmgBatch), which
// stayed open across every pass's ask, closes.
func (s *State) eachCardClose(in *kop.Instr) {
	if in.C&kop.RepeatDmgBatch != 0 {
		s.dmgBatchClose()
	}
}

// eachCardList lists OpRepeatEach's subjects (see repeatEachCards).
func (s *State) eachCardList(k *prep, in *kop.Instr, list *[MaxPerm]Obj) int {
	e := &s.res.e
	n := 0
	if in.C&kop.RepeatRemembered != 0 {
		// the remembered set, in remembered order (gorge repeatedCards over
		// hostRemembered): a card that has left the battlefield is a subject
		// too, the loop binds it as RefEach (act_repeatremembered.go)
		for _, o := range s.g.rem[:s.g.nrem] {
			if o != NoObj && n < len(list) {
				list[n] = o
				n++
			}
		}
		return n
	}
	if sl, ok := tgtSlot(in.Obj); ok {
		var codes [maxXTgt + 1]TCode
		m := s.divTargets(k, e, uint8(sl), &codes)
		for i := 0; i < m && n < len(list); i++ {
			if isObjTarget(codes[i]) {
				list[n] = Obj(codes[i])
				n++
			}
		}
		return n
	}
	src := s.objRef(k, kop.RefSelf)
	q := s.aliveFrom(e.ctrl)
	var ord [MaxPerm]Slot
	for i := uint8(0); i < q.n; i++ {
		p := q.s[i]
		var hit slotMask
		for m := s.occ.and(s.ctrlM[p]); m.any(); m = m.rest() {
			if sl := m.first(); s.matchPermGen(k, in.F, sl, e.ctrl, src, layerFull) {
				hit = hit.with(sl)
			}
		}
		for _, sl := range ord[:s.battlefieldOrder(p, hit, ord[:], 0)] {
			list[n] = s.perms[sl].obj
			n++
		}
	}
	return n
}
