package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kop.TgtPermOrSpell, ChangeZone Origin$ Battlefield,Stack
// ("return target spell or creature/permanent to its owner's hand": Fatehold
// Charm, Jeskai Revelation; the owner's top/bottom choice: Swat Away, Sudden
// Setback). gorge effChangeZone's object
// path (engine/effects/zone_change.go) moves the one target from whichever
// of the two zones it is in; a spell leaving the stack that was cast with
// flashback (or a sibling) is exiled instead (rules/stack_keyword_exile.go,
// CR 702.34a), a move and not a counter.

// DebugPermOrSpellSelf (mutation, TestPermorspellMutation): the source
// spell itself is a legal target (CR 115.5 ignored).
var DebugPermOrSpellSelf bool

// DebugPermOrSpellNoExile (mutation, TestPermorspellNoExileMutation): a
// spell target leaves the stack by moveRef, skipping the flashback exile.
var DebugPermOrSpellNoExile bool

// DebugPermOrSpellTopBottomNoExile (mutation,
// TestPermorspellTopBottomNoExileMutation): the owner's top/bottom answer
// moves a spell target by moveRef, skipping the flashback exile.
var DebugPermOrSpellTopBottomNoExile bool

// targetOKPermOrSpell: a permanent matching t.Filter (the TgtObject arm), or
// a spell on the stack other than the source (gorge Card.inZoneStack,
// filter_word.go; CR 115.5). An ability on the stack is not a spell.
//
//go:noinline
func (s *State) targetOKPermOrSpell(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, o Obj) bool {
	ob := &s.objs[o]
	switch ob.zone {
	case kop.ZoneBattlefield:
		return s.matchPerm(k, t.Filter, ob.slot, p, src) && s.objTargetable(k, ob.slot, p)
	case kop.ZoneStack:
		if o == self && !DebugPermOrSpellSelf {
			return false
		}
		i := s.stackIndexOf(o)
		return i >= 0 && s.stack[i].kind == seSpell
	}
	return false
}

// permOrSpellCands lists TgtPermOrSpell's choices: the battlefield in slot
// order, then the stack top-down (targetCands' order; MaxPerm+MaxStack fits
// MaxOptions).
//
//go:noinline
func (s *State) permOrSpellCands(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj, out *[MaxOptions]TCode, n int) int {
	for m := s.occ; m.any(); m = m.rest() {
		o := s.perms[m.first()].obj
		if s.targetOK(k, t, p, src, self, TCode(o)) {
			out[n] = TCode(o)
			n++
		}
	}
	for i := int(s.sp) - 1; i >= 0; i-- {
		e := &s.stack[i]
		if e.kind == seSpell && s.targetOK(k, t, p, src, self, TCode(e.obj)) {
			out[n] = TCode(e.obj)
			n++
		}
	}
	return n
}

// bouncePermOrSpell is OpBounce kop.MvPermOrSpell's target off the
// battlefield: a spell goes to its owner's hand by counterTo (exiled if cast
// by flashback); in any other zone it stays (changeZoneMoveSkip). done
// false: a permanent, bounced by the caller.
//
//go:noinline
func (s *State) bouncePermOrSpell(k *prep, o Obj) (done bool) {
	switch s.objs[o].zone {
	case kop.ZoneBattlefield:
		return false
	case kop.ZoneStack:
		if DebugPermOrSpellNoExile {
			return false
		}
		if i := s.stackIndexOf(o); i >= 0 {
			s.counterTo(k, i, kop.CtrDestHand)
		}
	}
	return true
}

// topBottomStack is OpToLibraryChoice's resume for a target still on the
// stack: the owner's answer through counterTo, so a flashback-family spell
// is exiled (gorge keywordExileDeparture on every stack departure).
// ok false: the object is not a spell on the stack (the caller's moveRef).
//
//go:noinline
func (s *State) topBottomStack(k *prep, r kop.Ref, bottom bool) bool {
	o := s.objRefObj(r)
	if o == NoObj || s.objs[o].zone != kop.ZoneStack {
		return false
	}
	i := s.stackIndexOf(o)
	if i < 0 {
		return false
	}
	if DebugPermOrSpellTopBottomNoExile {
		s.moveRef(k, r, kop.ZoneLibrary, bottom)
		return true
	}
	d := int16(kop.CtrDestLibTop)
	if bottom {
		d = kop.CtrDestLibBottom
	}
	s.counterTo(k, i, d)
	return true
}
