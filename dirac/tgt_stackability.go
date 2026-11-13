package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Stack-ability targets (Ertai Resurrected's "counter target spell,
// activated ability, or triggered ability"; gorge TargetType$
// Spell,Activated,Triggered: rules target_legal.go's stack census).
//
// A TgtStackObj slot takes a spell matching its filter by the spell's
// object, as TgtSpell does, or an activated or triggered ability as the
// target byte StackTarget0 + its stack index from the bottom: the oracle's
// canonical RefStack + i, which gorge's identity also maps a stack object
// to, so both sides name the option alike. The index is only the name: the
// slot's tver holds the entry's stack identity (stackEntry.sid, given at
// its push while genState.anyStackTgt), and the legality recheck and the
// counter find the entry by it, so the target survives an entry below it
// leaving the stack (a countered spell beneath). Everything here is reached
// only from a TgtStackObj slot or an anyStackTgt table.

// maxStackTgt is the number of stack indexes the code space (and gorge's
// RefStack range) names.
const maxStackTgt = int(PlayerTarget0 - StackTarget0) // seat-ok: the stack-code span, not a seat

// isStackTarget reports whether target byte c names a stack ability.
func isStackTarget(c TCode) bool { return c >= StackTarget0 && c < PlayerTarget0 } // seat-ok: the stack-code span, not a seat

// StackTargetIndex is isStackTarget for the kernel's clients (the search
// bots): the stack index, from the bottom, that target byte c names. A byte
// that names a stack ability is not an Obj, so a client that indexes an
// object table with it reads past MaxObj.
func StackTargetIndex(c TCode) (int, bool) {
	if !isStackTarget(c) {
		return 0, false
	}
	return int(c - StackTarget0), true
}

// DebugStackTgtByIndex finds a stack-ability target by its index rather
// than its identity (the kertai mutation test).
var DebugStackTgtByIndex bool

// stackSid gives the entry being pushed its stack identity (never 0).
func (s *State) stackSid(e *stackEntry) {
	s.g.sidN++
	if s.g.sidN == 0 {
		s.g.sidN = 1
	}
	e.sid = s.g.sidN
}

// stackTgtIdx is the stack index of the ability entry target byte c with
// identity sid names, or -1 when it is no longer on the stack.
func (s *State) stackTgtIdx(c TCode, sid uint8) int {
	if DebugStackTgtByIndex {
		if i := int(c - StackTarget0); i < int(s.sp) && s.stack[i].kind != seSpell {
			return i
		}
		return -1
	}
	for i := 0; i < int(s.sp); i++ {
		if e := &s.stack[i]; e.sid == sid && e.kind != seSpell {
			return i
		}
	}
	return -1
}

// stackTgtSelf is the stack index of the entry being built or resolving
// (CR 115.5: it is not a legal target for itself), -1 for none. A spell the
// resolving entry offers is not that entry: while the offer probes its targets
// the resolving ability is still on the stack (CR 608.2m) and it can be chosen.
func (s *State) stackTgtSelf() int {
	if s.g.probe.kind == probeSpell {
		return -1
	}
	if s.ctl == ctlBuild && s.bld().bfl&bflPushed != 0 || s.res.active {
		return int(s.sp) - 1
	}
	return -1
}

// stackTgtOK is targetOK for a TgtStackObj slot.
//
//go:noinline
func (s *State) stackTgtOK(k *prep, t *kop.TargetSpec, p uint8, self Obj, code TCode) bool {
	if t.Kind == kop.TgtSpellTgting {
		return s.spellTgtingOK(k, t, p, self, code) // tgt_countershapes.go
	}
	if t.Kind == kop.TgtStackTgting {
		return s.stackTgtingOK(k, t, p, self, code) // tgt_countershapes.go
	}
	if isStackTarget(code) {
		i := int(code - StackTarget0)
		if i >= int(s.sp) || i == s.stackTgtSelf() {
			return false
		}
		if copyAbKind(t.Kind) {
			return s.copyAbOK(k, t, p, i) // copy_ability.go
		}
		if t.Kind == kop.TgtStackObjTrig {
			return trigOnlyOK(s.stack[i].kind) // tgt_countershapes.go
		}
		if e := &s.stack[i]; t.TFlags&kop.TgfActOnly != 0 && e.kind != seAbility {
			return false
		}
		return s.stack[i].kind != seSpell && (t.TFlags&kop.TgfSingleTgt == 0 || s.ksrSingle(&s.stack[i]))
	}
	if !isObjTarget(code) || t.Kind == kop.TgtStackAb || copyAbKind(t.Kind) {
		return false
	}
	o := Obj(code)
	ob := &s.objs[o]
	if ob.zone != kop.ZoneStack || o == self {
		return false
	}
	return s.matchCardAt(k, t.Filter, ob.card, s.stackCtrl(o), p, o) && (t.TFlags&kop.TgfSingleTgt == 0 || s.ksrSingleSpell(o))
}

// stackTgtCands writes a TgtStackObj slot's candidates, the stack top-down
// as targetCands lists spells.
//
//go:noinline
func (s *State) stackTgtCands(k *prep, t *kop.TargetSpec, p uint8, self Obj, out *[MaxOptions]TCode, n int) int {
	for i := int(s.sp) - 1; i >= 0 && n < MaxOptions; i-- {
		e := &s.stack[i]
		code := TCode(e.obj)
		if e.kind != seSpell {
			if i >= maxStackTgt {
				s.stall(StallTables) // beyond gorge's canonical stack range
				continue
			}
			code = StackTarget0 + TCode(i)
		}
		if s.stackTgtOK(k, t, p, self, code) {
			out[n] = code
			n++
		}
	}
	return n
}

// stackTgtPicked records the identity of a picked stack-ability target in
// its slot's tver.
func (s *State) stackTgtPicked(e *stackEntry, ti uint8, code TCode) {
	if i := int(code - StackTarget0); i < int(s.sp) {
		e.tver[ti] = s.stack[i].sid
	}
}

// stackTgtSid is the stack identity of the ability entry stack-ability target
// byte c names (0 when it is not on the stack).
func (s *State) stackTgtSid(c TCode) uint8 {
	if i := int(c - StackTarget0); i < int(s.sp) {
		return s.stack[i].sid
	}
	return 0
}

// stackTgtStillLegal is targetStillLegal for a stack-ability target: the
// entry is still on the stack (CR 608.2b).
func (s *State) stackTgtStillLegal(e *stackEntry, i int) bool {
	return s.stackTgtIdx(e.tgt[i], e.tver[i]) >= 0
}

// counterStackTgt is OpCounter of a stack-ability target: the entry leaves
// the stack (CR 701.5a: an ability is removed, its source untouched), its
// controller kept for the chain's "its controller" (RefTargetController).
// It reports whether in named such a target.
//
//go:noinline
func (s *State) counterStackTgt(k *prep, in *kop.Instr) bool {
	slot, ok := tgtSlot(in.Obj)
	if !ok {
		return false
	}
	c := s.tgtCode(slot)
	if !isStackTarget(c) {
		return false
	}
	if i := s.stackTgtIdx(c, s.res.e.tver[slot]); i >= 0 {
		s.g.stkCt = s.stack[i].ctrl + 1
		s.counterAt(k, i)
	}
	return true
}

// stackTgtCtrl is the controller of stack-ability target slot i: the live
// entry's, else the one a counter removed.
func (s *State) stackTgtCtrl(i int) uint8 {
	e := &s.res.e
	if j := s.stackTgtIdx(e.tgt[i], e.tver[i]); j >= 0 {
		return s.stack[j].ctrl
	}
	if s.g.stkCt != 0 {
		return s.g.stkCt - 1
	}
	return 255
}

// StackTargetGone is a projected stack-ability target that has left the
// stack: still a target, naming nothing (gorge's RefNone for the vanished
// stack object; the oracle maps it so).
const StackTargetGone TCode = 0xFE

// stackTgtProj is a stack-ability target byte as the projection names it:
// StackTarget0 + the entry's current index (gorge's live RefStack), or
// StackTargetGone.
func (s *State) stackTgtProj(c TCode, sid uint8) TCode {
	if i := s.stackTgtIdx(c, sid); i >= 0 && i < maxStackTgt {
		return StackTarget0 + TCode(i)
	}
	return StackTargetGone
}

// tgtCtrlSlot is the controller of target slot i (RefTargetController for
// slot 0, RefTgtCtrl1/2): a player target is that player; a stack-ability
// target or a spell still on the stack its controller; a spell this resolution countered the controller it
// had; another object that left since it was targeted (once) its last known
// controller (lkiCtrl).
func (s *State) tgtCtrlSlot(k *prep, i int) uint8 {
	e := &s.res.e
	t := s.tgtCode(i)
	if t == NoTarget {
		return 255
	}
	if isPlayerOrNone(t) {
		return CodeSeat(t)
	}
	if isStackTarget(t) {
		return s.stackTgtCtrl(i)
	}
	if ob := &s.objs[t]; ob.zone == kop.ZoneStack {
		if j := s.stackIndexOf(Obj(t)); j >= 0 {
			return s.stack[j].ctrl // a spell is controlled by whoever cast it, not its owner (CR 108.4, 110.2)
		}
	}
	if ob := &s.objs[t]; ob.zone != kop.ZoneBattlefield && ob.ver == e.tver[i]+1 {
		if s.g.stkCt != 0 {
			return s.g.stkCt - 1 // the spell this resolution countered, cast by its controller, not its owner
		}
		return s.lkiCtrlSlot(k, Obj(t), i)
	}
	return s.ctrlOfObj(Obj(t))
}
