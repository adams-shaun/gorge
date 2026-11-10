package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// attachObj is OpAttachObj (gorge effAttach with an Object$ other than the
// source): the permanent in Ref C becomes attached to the permanent Obj;
// nothing when either is not on the battlefield (attachObjectGone, the
// destination walk's battlefield rule) or they are the same permanent.
// B = Const 1 (RememberAttached$ True) remembers each object attached (gorge
// attachTo's remember half). A Ref C target slot holding several objects
// (Object$ ParentTarget over Thorin, Mountain-king's "any number of target
// Equipment": gorge attachObjectParentTarget) attaches each still-legal one,
// in pick order (attachEachTarget).
func (s *State) attachObj(k *prep, in *kop.Instr) {
	dst := s.objRef(k, in.Obj)
	if kop.Ref(in.C) == kop.RefRemembered && s.g.nrem > 1 {
		s.attachRemembered(k, dst) // tgt_attachremembered.go
		return
	}
	if dst == NoSlot {
		dst = s.attachPickSelf(k, s.res.pc) // tgt_attachpick.go
	}
	if s.attachEachTarget(k, in, dst) {
		return
	}
	s.attachOne(k, in, dst)
}

// attachOne attaches the one object Ref C names to dst (attachObj's
// single-object rule) and remembers it under B = Const 1.
func (s *State) attachOne(k *prep, in *kop.Instr, dst Slot) {
	src := s.objRef(k, kop.Ref(in.C))
	if src == NoSlot || dst == NoSlot || src == dst {
		return
	}
	s.attachTo(k, src, dst)
	s.touchSl(src)
	if in.B == kop.Const(1) {
		s.remember(s.perms[src].obj)
	}
}

// attachEachTarget is attachObj over a Ref C target slot with extra objects:
// the slot's first object, then each still-legal extra in pick order (the
// order gorge's parent link answered them, divTargets'), each by attachOne's
// rule (biteEachTarget's save/restore of the slot). It reports whether in is
// such an attach.
func (s *State) attachEachTarget(k *prep, in *kop.Instr, dst Slot) bool {
	e := &s.res.e
	if k.gen == nil || s.xtOf(e).n == 0 {
		return false
	}
	sl, ok := tgtSlot(kop.Ref(in.C))
	if !ok || !s.slotHasExtra(e, uint8(sl)) {
		return false
	}
	saveT, saveV, saveBad := e.tgt[sl], e.tver[sl], s.res.bad
	s.attachOne(k, in, dst)
	for j := 0; j < int(s.xtOf(e).n); j++ {
		if s.xtOf(e).slot[j] != uint8(sl) || !s.xtLegalRes(k, e, j, uint8(sl)) {
			continue
		}
		e.tgt[sl], e.tver[sl] = s.xtOf(e).code[j], s.xtOf(e).ver[j]
		s.res.bad &^= 1 << sl
		s.attachOne(k, in, dst)
	}
	e.tgt[sl], e.tver[sl], s.res.bad = saveT, saveV, saveBad
	return true
}

// attachRemembered is OpAttachObj with Ref C = RefRemembered over several
// remembered objects (gorge attachObjectRemembered / rememberedAttachObjects:
// EVERY object of the resolution's remembered set is fastened to the
// destination, each by its own legality). The kgen guard admits the shape
// behind a single one-object writer, so a second entry only appears when a
// replacement multiplies the writer (a token doubler: one remembered Sturdy
// Shield per token made). An object no longer on the battlefield, or the
// destination itself, attaches nothing (attachObjectGone, CR 701.3b).
func (s *State) attachRemembered(k *prep, dst Slot) {
	if dst == NoSlot {
		return
	}
	for i := 0; i < int(s.g.nrem); i++ {
		o := s.g.rem[i]
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
			continue
		}
		src := s.objs[o].slot
		if src == dst {
			continue
		}
		s.attachTo(k, src, dst)
		s.touchSl(src)
	}
}
