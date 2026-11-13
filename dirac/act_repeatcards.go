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
// The body is run here, inline, in one call (the way execUnion runs one
// instruction over its objects), so the loop needs no State of its own: the
// frame is the Go stack and eachObj, which a resolution starts afresh. A
// body instruction that asks (exec false) cannot be resumed from inside the
// loop, so it stalls loudly; kgen lowers only a Role mint here, which asks
// nothing (createAttachedToken), and an Aura-token replacement that would
// ask stalls the mint itself (StallRoleMulti).
//
// ChangeZoneTable$ True: the zone changes of every pass reach the batch
// listeners ("one or more ...") once, after the last pass, so the loop runs
// inside one batch and one entry hold, as execUnion's does.
//
//go:noinline
func (s *State) repeatEachCards(k *prep, in *kop.Instr) {
	var list [MaxPerm]Obj
	n := s.eachCardList(k, in, &list)
	body := int(in.B.N)
	r := &s.res
	pc := int(r.pc)
	r.pc += uint8(body) // the header's own step is the caller's
	if n == 0 || body == 0 {
		return
	}
	prog := s.prog(k)
	if k.gen.anyBatch {
		defer s.batchClose(s.batchOpen())
	}
	hr := s.holdOpen(k)
	defer s.holdClose(k, hr)
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

// eachCardList lists OpRepeatEach's subjects (see repeatEachCards).
func (s *State) eachCardList(k *prep, in *kop.Instr, list *[MaxPerm]Obj) int {
	e := &s.res.e
	n := 0
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
