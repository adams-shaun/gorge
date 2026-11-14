package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kxpaid: CR 107.3m binds the X of a triggered ability that paid no {X}
// of its own to the X chosen for the spell its trigger object was (an
// enters trigger of a permanent cast with X; a cast trigger reads the
// spell cast). gorge captures it when the trigger matches
// (rules/trigger_referents.go: TriggerContext.TriggerPaidX = card.X of
// the causing event's card) and resolution reads it when the ability
// object's own X is 0 (rules/stack.go resolveTop, triggerPaidX). kgen marks
// the triggers whose body reads it (Tf2XBound, dirac/kgen/lower_xbound.go:
// self enters, and SpellCast of a source with no {X}); the pending entry
// carries the value (pendingTrig.x) into the stack entry's x.
//
// The trigger object's X as it triggers: the resolving spell's when the
// object is the permanent it is becoming (the enters check runs inside
// the resolution, the entry already popped), a spell entry's on the
// stack (a cast trigger), else 0 (gorge's card.X off the stack and the
// battlefield; an object put onto the battlefield other than by its
// spell's resolution was never cast with X).

// bindTrigX sets the X of the pending trigger just queued (the last one)
// from its trigger object tobj.
//
//go:noinline
func (s *State) bindTrigX(tobj Obj, spent bool) {
	if s.npend == 0 || s.pend[s.npend-1].trigObj != tobj || tobj == NoObj {
		return
	}
	if spent {
		s.pend[s.npend-1].x = objWord(s.spellSpent(tobj)) // Tf2SpentBound (CR 601.2h)
		return
	}
	s.pend[s.npend-1].x = objWord(s.spellX(tobj))
}

// spellSpent is the mana spent to cast the spell object o is or is
// becoming (recordSpent's entry amount), 0 if it was never cast.
func (s *State) spellSpent(o Obj) uint8 {
	n := -1
	if s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o {
		n = int(s.res.e.trigAmt)
	}
	for i := int(s.sp) - 1; n < 0 && i >= 0; i-- {
		if e := &s.stack[i]; e.kind == seSpell && e.obj == o {
			n = int(e.trigAmt)
		}
	}
	if n < 0 {
		return 0
	}
	if n > 255 {
		n = 255
	}
	return uint8(n)
}

// spellX is the paid X of the spell object o is or is becoming, 0 if none.
func (s *State) spellX(o Obj) uint8 {
	if s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o {
		return uint8(s.res.e.x)
	}
	for i := int(s.sp) - 1; i >= 0; i-- {
		if e := &s.stack[i]; e.kind == seSpell && e.obj == o {
			return uint8(e.x)
		}
	}
	return 0
}

// projTrigX is the X the projections show for trigger entry e (e.x set):
// gorge's stack object's own X (Object.X), which a trigger's bound X is
// not (it rides the trigger context), so a trigger without an {X} cost
// shows 0.
//
//go:noinline
func projTrigX(k *prep, e *stackEntry) uint8 {
	cd := &k.t.Cards[e.card]
	if int(e.ab) < len(cd.Abilities) && cd.Abilities[e.ab].Kind == kop.AbTriggered && cd.Abilities[e.ab].Cost.Mana.X != 0 {
		return uint8(e.x)
	}
	return 0
}

// The spell hold (kverify-c4 open item 3). gorge makes a resolving spell's
// free casts mid-resolution (effects/discover.go effPlay, then the
// discover tail), the spell still on the stack under the cast while its
// choices are asked, and the spell leaves the stack only when its
// resolution completes after them. The kernel makes the free casts after
// the resolution's program (genState.freeQ), holding an ability's entry
// on the stack under them (sfHold, turn_gen.go); a spell is held the same
// way: finishResolve leaves its object on the stack, stepGen pushes its
// entry back (holdable), and dropHold takes it off and moves the card to
// its graveyard (or exile) once the last cast is made. A spell entry's
// sfHold bit is sfPromised, so the held spell is named by the resolution
// context instead (spellHeld: res.e stays the resolved entry until the
// next resolution, and nothing resolves before the casts are made).

// holdable: the resolved entry can be held: an ability, or a spell whose
// card is still on the stack (finishResolve kept it there).
func (s *State) holdable() bool {
	e := &s.res.e
	return e.kind != seSpell || s.objs[e.obj].zone == kop.ZoneStack
}

// spellHeld: stack entry e is the held resolved spell.
func (s *State) spellHeld(e *stackEntry) bool {
	return e.kind == seSpell && e.obj == s.res.e.obj && e.srcVer == s.res.e.srcVer
}

// spellHoldLeaves moves the held spell's card off the stack, as
// finishResolve would have: to its owner's graveyard, exile when flashed
// back or a copy (spellRestExiles), and a face's own exit
// (face_castface.go).
//
//go:noinline
func (s *State) spellHoldLeaves(k *prep) {
	e := &s.res.e
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneStack || ob.ver != e.srcVer {
		return
	}
	for i := 0; i < int(s.sp); i++ {
		if s.stack[i].kind == seSpell && s.stack[i].obj == e.obj {
			return // still on the stack (not held)
		}
	}
	s.touch(e.obj)
	ob.zone = kop.ZoneNone
	z := kop.ZoneGraveyard
	// spellRestZone: the flashback family and the Play rider's
	// ReplaceGraveyard$ both exile a held spell's card, and a copy of a
	// spell is exiled as it leaves the stack (CR 707.10a).
	if s.spellRestExiles(e) || e.markExiles(false) {
		z = kop.ZoneExile
	}
	rebound := s.reboundRests(k, e, false) // kw_rebound.go
	if rebound {
		z = kop.ZoneExile
	}
	if k.gen != nil && k.gen.anyFaces {
		z = s.stackExitFace(k, e.obj, z, true)
	}
	if k.gen != nil && k.gen.anyParadigm {
		z = s.paradigmRest(k, e.obj, z) // kw_paradigm.go
	}
	z = buybackRest(k, e, z, false) // kw_buyback.go
	if z == kop.ZoneLibrary {
		s.omenToLibrary(e.obj) // face_omen.go
	} else {
		s.placeFrom(e.obj, z, false, kop.ZoneStack, false)
		s.markedPlot(k, e) // repl_stackexile.go
		if rebound {
			s.reboundRegister(k, e)
		}
	}
}

// A mandatory dig (kxpaid: Geometer's Arthropod's "put one of them into
// your hand", carried by the bound X). gorge effDig asks only when there
// is a choice (dig.go: takeChoice, optionalChoice) and its ask's Min is
// ChangeNum unless the dig is optional or takes any number: a mandatory
// dig with no more matching cards than it takes moves them all unasked
// (the greedy take), and its ask has no stop.

// digMayStop: the resolving OpDigEx may take fewer than its count
// (Optional$, or ChangeNum$ Any).
func (s *State) digMayStop(k *prep) bool {
	in := &s.prog(k)[s.res.pc]
	return (in.C>>3)&kop.DigOptional != 0 || in.B.K == kop.VConst && in.B.N == -2
}

// digForced takes every matching looked-at card of a mandatory dig that
// has no more of them than it takes (s.g.pickLeft), reporting whether it
// did; the caller finishes the dig. Under a pick-set bound
// (kop.Filter.PickSumMax / PickDistinctPower) the walk is gorge's greedy
// forced take: a card joins only while the running sum and distinct powers
// still admit it, so a non-fitting card is skipped rather than taken.
//
//go:noinline
func (s *State) digForced(k *prep) bool {
	if s.digMayStop(k) {
		return false
	}
	f := s.prog(k)[s.res.pc].F
	b := k.t.Filters[f].PickSumMax
	hasBudget := b.K == kop.VConst && b.N > 0
	save := s.g.lookSel
	n, eligible := 0, 0
	for i := 0; i < int(s.g.nlook); i++ {
		o := s.resArm().look[i]
		if save&(1<<i) != 0 || !s.matchCardGen(k, f, s.objs[o].card, s.objs[o].owner(), s.res.e.ctrl, o) {
			continue
		}
		// gorge's budgetEligible test is the card's own mana value against
		// the budget, independent of the running sum
		if hasBudget && int(k.t.Cards[s.objs[o].card].Cost.MV()) > int(b.N) {
			continue
		}
		eligible++
		// gorge's greedy forced take: the accepted simulated picks are the
		// bits set so far, so the candidate is packed only once it fits
		if s.pickFits(k, f, o, true) {
			s.g.lookSel |= 1 << i
			n++
		}
	}
	// gorge asks when the eligible pool is larger than the take (takeChoice)
	// or, under a budget, when the greedy take skips an affordable card
	// (askBudget). Without a budget a same-power skip is the forced take.
	if eligible > int(s.g.pickLeft) || hasBudget && n != eligible {
		s.g.lookSel = save
		return false
	}
	s.g.pickLeft -= uint8(n)
	return true
}
