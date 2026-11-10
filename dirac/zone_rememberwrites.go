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

// reflexStale reports the resolving reflexive entry's captured object gone:
// none was captured, or it changed zones since (its ver moved).
func (s *State) reflexStale() bool {
	o := s.res.e.trigObj
	return o == NoObj || s.objs[o].ver != uint8(s.res.e.trigAmt)
}
