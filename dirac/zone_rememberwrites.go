package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// RememberDrawn$ on Draw, RememberMoved$ and RememberKept$ on Surveil,
// RememberSacrificed$ on SacrificeAll. Each is the instruction's
// kop.MvRemember bit; every path here is behind that one guard.

// DebugNoRemWrites3 (TestRememberWritesMutation) skips the three remember writes.
var DebugNoRemWrites3 bool

// drawRemember draws one card for p and remembers it when it reached the
// hand (gorge effDraw: the top card, unless the library was empty). all is
// RememberDrawn$ AllReplaced: every card the draw and its replacement body
// moved into p's hand, in library order (gorge draw.go recordDrawn), not only
// the pre-draw top card.
//
//go:noinline
func (s *State) drawRemember(p uint8, all bool) {
	pl := &s.pl[p]
	if pl.libLo == pl.libHi {
		s.draw(p)
		return
	}
	lo, hi := pl.libLo, pl.libHi
	s.draw(p)
	if DebugNoRemWrites3 {
		return
	}
	if !all {
		if o := pl.lib[lo]; s.objs[o].zone == kop.ZoneHand {
			s.remember(o)
		}
		return
	}
	for i := lo; i < hi; i++ {
		if o := pl.lib[i]; s.objs[o].zone == kop.ZoneHand {
			s.remember(o)
		}
	}
}

// surveilRemember remembers o when the surveil's graveyard move landed it
// there (gorge arrange.go: RememberMoved$ after the moves, in pile-B order).
//
//go:noinline
func (s *State) surveilRemember(o Obj) {
	if s.objs[o].zone == kop.ZoneGraveyard && !DebugNoRemWrites3 {
		s.remember(o)
	}
}

// surveilRememberKept remembers the top cards the surveil left on top of the
// library, in the order they were put there (gorge arrange.go: RememberKept$
// records pile A).
//
//go:noinline
func (s *State) surveilRememberKept(top int) {
	if DebugNoRemWrites3 {
		return
	}
	for i := 0; i < top; i++ {
		s.remember(s.res.arr[i])
	}
}

// sacAllRemember is leaveAll for a SacrificeAll with RememberSacrificed$:
// each sacrificed permanent joins the remembered set in gorge's victim
// order (seat 0's battlefield list, then seat 1's).
//
//go:noinline
func (s *State) sacAllRemember(k *prep, hit slotMask) {
	if DebugNoRemWrites3 {
		s.leaveAll(k, hit, kop.ZoneGraveyard)
		return
	}
	s.rememberLeaveAll(k, hit, kop.ZoneGraveyard)
}

// an ImmediateTrigger's RememberObjects$ Targeted
// (gorge effImmediateTrigger replaces the instance's remembered set with
// the parent's target; Novel Nunchaku, Curse of the Werefox) and the body's
// Fight Defined$ DelayTriggerRemembered. A gorge id changes on a zone move
// (CR 400.7) while the kernel keeps Obj and bumps ver, so the pending entry
// carries the object in trigObj and its ver in trigAmt; the fight skips a
// captured creature whose ver has moved since.

// DebugNoReflexCapture (test-only mutation) captures nothing.
var DebugNoReflexCapture bool

// reflexCapture is the object and ver a ReflexiveTrigger's Obj target ref
// names, or (NoObj, 0) when the slot is empty, illegal, a player, or the
// object left the battlefield since it was chosen.
//
//go:noinline
func (s *State) reflexCapture(r kop.Ref) (Obj, int16) {
	sl, _ := tgtSlot(r)
	c := s.tgtCode(sl)
	if int(c) >= MaxObj || DebugNoReflexCapture {
		return NoObj, 0
	}
	ob := &s.objs[c]
	if ob.ver != s.res.e.tver[sl] || ob.zone != kop.ZoneBattlefield {
		return NoObj, 0
	}
	return Obj(c), int16(ob.ver)
}

// A Bite from the capture (Thorin, Mountain-king: "that creature deals
// damage equal to its power") uses the creature's last known power once it
// has left (CR 608.2h; gorge rules rememberedPTLKI): OpReflexiveTrigger
// snapshots the captured creature into the entry's LKI record (li) as the
// trigger is put on the stack, the same instant gorge captures it, and the
// stale Bite deals that power from the gone creature. A pump between the
// queueing and the departure is seen by neither side.

// DebugNoReflexBiteLKI (test-only mutation) skips the gone capture's bite,
// as before the last known power was kept.
var DebugNoReflexBiteLKI bool

// reflexBites reports whether latent ability ab of card c bites from its
// captured object (an OpBite of RefDelayedRemembered): only that body takes
// the snapshot, so no other reflexive entry gains an LKI record.
func (s *State) reflexBites(k *prep, c CardID, ab uint8) bool {
	abs := k.t.Cards[c].Abilities
	if int(ab) >= len(abs) {
		return false
	}
	for i := range abs[ab].Prog {
		if in := &abs[ab].Prog[i]; in.Op == kop.OpBite && in.Obj == kop.RefDelayedRemembered {
			return true
		}
	}
	return false
}

// reflexBiteGone is OpBite from a reflexive capture that left the
// battlefield: the creature, by its snapshot, deals its last known power to
// the creature or planeswalker in Ref C (dmgSrcObject's departed-source
// path: its last known keywords and controller). It reports false (the bite
// is skipped as before) without a snapshot of that capture.
//
//go:noinline
func (s *State) reflexBiteGone(k *prep, in *kop.Instr) bool {
	e := &s.res.e
	if e.li == 0 || e.remSet || e.trigObj == NoObj || DebugNoReflexBiteLKI {
		return false
	}
	l := s.lk(e.li)
	if l.obj != e.trigObj || l.ver != uint8(e.trigAmt) {
		return false
	}
	b := s.objRef(k, kop.Ref(in.C))
	if b == NoSlot || !s.isCreature(k, b) && s.typesAt(k, b)&kop.TPlaneswalker == 0 {
		return true
	}
	n := int(l.power)
	if DebugDmgAmountPlusOne {
		n++
	}
	if n <= 0 {
		return true
	}
	o := e.trigObj
	s.dmgSrcObject(k, s.perms[b].obj, o, NoSlot, l.kw, l.ctrl, k.t.Cards[s.objs[o].card].Infect, n)
	return true
}

// reflexStale reports the resolving reflexive entry's captured object gone:
// none was captured, or it changed zones since (its ver moved).
func (s *State) reflexStale() bool {
	o := s.res.e.trigObj
	return o == NoObj || s.objs[o].ver != uint8(s.res.e.trigAmt)
}
