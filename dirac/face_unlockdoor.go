package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DB$ UnlockDoor (CR 709.5f-g; gorge
// effects/unlockdoor.go effUnlockDoor) and Inquisitive Glimmer's "unlock costs
// you pay cost {1} less" (CR 601.2f; gorge rules/rooms.go unlockMods).
//
// The effect is exactly the printed shape kgen lowers: one target slot
// (ValidTgts$ Room.YouCtrl, 0..1 or one target). The pool is that target,
// never the wider Choices$/all-Rooms fallbacks (gorge unlockDoorPool): a
// declined Min-0 slot does nothing. Mode$ Unlock unlocks a locked door;
// Mode$ LockOrUnlock first asks "Lock or unlock a door?" (option 0 = unlock,
// gorge askUnlockDoorHalf), then picks a door when more than one door
// qualifies (gorge "Choose a Room door", candidates in face order). The
// chosen door's designation flips without paying; an unlock fires the
// fully-unlock watchers and the door's own UnlockDoor triggers exactly as
// the paid special action does, a lock only clears the designation (gorge
// events.apply_misc foldDoorLock) and fires nothing.
//
// This is cold: reached only when the table holds a Room (prep.anyRoom), and
// the reduction walk below only when a table holds an unlock-cost static.

// DebugUnlockDoorOff is the oracle mutation (TestDskroomsMutation): the
// effect flips the wrong door (an already-unlocked door for an unlock, a
// locked one for a lock) and the unlock-cost reduction is dropped.
var DebugUnlockDoorOff bool

// The two mid-resolution asks: the lock-or-unlock half, then the door pick.
const (
	raUnlockHalf uint8 = 254
	raUnlockDoor uint8 = 255
)

// extUnlockDoor runs one ExtUnlockDoor instruction. true: the effect is
// done (or does nothing); false: it suspended on an ask.
//
//go:noinline
func (s *State) extUnlockDoor(k *prep, in *kop.Instr) bool {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return true
	}
	pm := &s.perms[sl]
	if pm.ctrl != s.res.e.ctrl || !roomDef(k, pm.card) || s.objs[pm.obj].card != pm.card || s.faceDown(k, sl) {
		return true
	}
	s.res.peek[0] = Obj(sl) // code-ok: a slot rides the peek list
	if in.C&1 != 0 { // seat-ok: an instruction flag bit
		s.res.ask, s.res.askP = raUnlockHalf, s.res.e.ctrl
		return false
	}
	if s.unlockDoorStep(k, sl, false) {
		return false // suspended on the door pick
	}
	return true
}

// unlockDoorHalf is the LockOrUnlock answer: 0 unlocks, 1 locks (gorge's
// option order). It resumes the resolution itself (an apply handler is
// outside runProg's loop).
//
//go:noinline
func (s *State) unlockDoorHalf(k *prep, a Action) {
	sl := Slot(s.res.peek[0]) // code-ok: the slot unlockDoorAsk parked
	s.res.ask = raNone
	if !s.unlockDoorStep(k, sl, a.Mode() == 1) {
		s.continueRes(k, 0)
	}
}

// unlockDoorStep picks the door and applies the change: no candidate does
// nothing, one is taken unasked, more than one is gorge's "Choose a Room
// door" ask in face order. It reports whether it posed an ask (the caller
// then leaves the resolution suspended); a false return has applied or done
// nothing and the caller resumes.
//
//go:noinline
func (s *State) unlockDoorStep(k *prep, sl Slot, lock bool) bool {
	if sl == NoSlot || !s.unlockDoorCand(k, sl, lock) {
		return false
	}
	pm := &s.perms[sl]
	m := roomMask(k, pm.card)
	n, only := 0, uint8(0)
	for fi := uint8(0); fi < 2; fi++ {
		if s.unlockDoorFi(m, fi, lock) {
			n++
			only = fi
		}
	}
	if n == 0 {
		return false // no door qualifies for this half: gorge does nothing
	}
	if n == 1 {
		s.unlockDoorApply(k, sl, lock, only)
		return false
	}
	s.res.left = 0
	if lock {
		s.res.left = 1
	}
	s.res.ask, s.res.askP = raUnlockDoor, s.res.e.ctrl
	return true
}

// unlockDoorCand reports whether the Room at sl can change a door for this
// half; the target must still be a battlefield Room its controller controls.
func (s *State) unlockDoorCand(k *prep, sl Slot, lock bool) bool {
	pm := &s.perms[sl]
	if pm.ctrl != s.res.e.ctrl || !roomDef(k, pm.card) || s.objs[pm.obj].card != pm.card || s.faceDown(k, sl) {
		return false
	}
	m := roomMask(k, pm.card)
	return s.unlockDoorFi(m, 0, lock) || s.unlockDoorFi(m, 1, lock)
}

// unlockDoorFi reports whether face fi is a candidate for the half: an
// unlock takes a locked door, a lock an unlocked one (gorge
// DoorUnlocked(fi) == lock). The mutation inverts it.
func (s *State) unlockDoorFi(m uint8, fi uint8, lock bool) bool {
	want := lock
	if DebugUnlockDoorOff {
		want = !want
	}
	return m&(1<<fi) != 0 == want
}

// unlockDoorApply flips door fi of Room sl to the chosen half, swapping the
// permanent to the state def for the new door mask (transform's subscription
// swap; gorge emits no timestamp event). An unlock fires the fully-unlock
// watchers then the door's UnlockDoor triggers, in the paid action's order;
// a lock fires nothing.
func (s *State) unlockDoorApply(k *prep, sl Slot, lock bool, fi uint8) {
	pm := &s.perms[sl]
	cd := &k.t.Cards[pm.card]
	m := roomMask(k, pm.card)
	bit := uint8(1) << fi
	if (m&bit != 0) != lock {
		return // not a candidate any more (the mutation picks a no-op door)
	}
	nm := m | bit
	if lock {
		nm = m &^ bit
	}
	s.touchSl(sl)
	s.subscribe(sl, &k.cards[pm.card], false)
	c := cd.Faces[kop.RoomState(cd.Face, nm)]
	ob := &s.objs[pm.obj]
	ob.card, pm.card = c, c
	s.subscribe(sl, &k.cards[c], true)
	s.subscribeGrants(k, sl)
	s.refreshTy(k)
	if lock {
		return
	}
	if k.anyFullyUnlock {
		if after := roomMask(k, ob.card); m != 3 && after == 3 {
			s.fireFullyUnlock(k, sl, pm.obj, s.res.e.ctrl)
		}
	}
	s.fireUnlockDoor(k, sl, pm.obj, fi)
}

// askOptionsUnlockDoor builds the half ask and the door pick (the tail of
// askOptionsBreach).
//
//go:noinline
func (s *State) askOptionsUnlockDoor(k *prep, d *Decision) bool {
	if s.res.ask != raUnlockHalf && s.res.ask != raUnlockDoor {
		return s.askOptionsStackRewrite(k, d) // act_stackrewrite.go
	}
	if s.res.ask == raUnlockHalf {
		d.Sub = ChRoomDoorHalf
		d.add(MakeAction(AkChoose, 0, 0, 0)) // gorge's options: unlock, lock
		d.add(MakeAction(AkChoose, 0, 0, 1))
		return true
	}
	d.Sub = ChRoomDoor
	sl := Slot(s.res.peek[0]) // code-ok: the slot unlockDoorAsk parked
	if sl == NoSlot {
		return true
	}
	pm := &s.perms[sl]
	m := roomMask(k, pm.card)
	ord := uint8(0)
	lock := s.res.left != 0
	for fi := uint8(0); fi < 2; fi++ {
		if !s.unlockDoorFi(m, fi, lock) {
			continue
		}
		d.add(MakeAction(AkChoose, TCode(pm.obj), TCode(ord), fi)) // code-ok: the door's place in the options
		ord++
	}
	return true
}

// applyAskUnlockDoor takes the half or the door pick and continues (the tail
// of applyAskBreach).
//
//go:noinline
func (s *State) applyAskUnlockDoor(k *prep, a Action) bool {
	if s.res.ask != raUnlockHalf && s.res.ask != raUnlockDoor {
		return s.applyAskStackRewrite(k, a) // act_stackrewrite.go
	}
	if s.res.ask == raUnlockHalf {
		s.unlockDoorHalf(k, a)
		return true
	}
	s.res.ask = raNone
	sl := Slot(s.res.peek[0]) // code-ok: the slot unlockDoorAsk parked
	lock := s.res.left != 0
	fi := a.Mode()
	if sl != NoSlot && fi < 2 && s.unlockDoorCand(k, sl, lock) && s.unlockDoorFi(roomMask(k, s.perms[sl].card), fi, lock) {
		s.unlockDoorApply(k, sl, lock, fi)
	}
	s.continueRes(k, 0)
	return true
}

// unlockReduced returns c after the StatReduceAbility statics with the
// ReduceAbUnlock bit (Inquisitive Glimmer): every static of the payer's whose
// condition holds and whose ValidCard$ matches the Room at sl subtracts its
// amount from the generic part, floored at zero after each, in battlefield
// entry order (eachStaticTS). Unlock costs pay mana only (gorge
// specialActionScope("unlock")); an ordinary ability is never reduced by
// these (abReduced skips them).
//
//go:noinline
func (s *State) unlockReduced(k *prep, p uint8, sl Slot, c kop.ManaCost) kop.ManaCost {
	if DebugUnlockDoorOff {
		return c // the mutation drops the reduction
	}
	if k.gen == nil || !k.gen.anyAbReduce || !k.anyRoom {
		return c
	}
	g := int(c.Generic)
	s.eachStaticTS(k, sclsReduce, func(st statSrc) bool {
		sa := &st.a.Static
		if st.a.Kind != kop.AbStatic || sa.Kind != kop.StatReduceAbility || st.ctrl != p {
			return true
		}
		if sa.B&kop.ReduceAbUnlock == 0 {
			return true
		}
		if !s.statCond(k, st) {
			return true
		}
		if sa.Filter != 0 && !s.matchPermGen(k, sa.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		n := int(sa.A)
		if sa.StFlags&kop.StDynamic != 0 {
			n = s.evalV(k, sa.VA, s.ctxStatic(st))
		}
		if n <= 0 {
			return true
		}
		g = max(g-n, 0)
		return true
	})
	c.Generic = uint8(g)
	return c
}
