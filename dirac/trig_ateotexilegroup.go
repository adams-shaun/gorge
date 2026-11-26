package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// AtEOT$ Exile over several objects of one resolution (Feral Lightning's
// "exile them at the beginning of the next end step", a CopyPermanent token
// rider over several copies) is one delayed triggered ability naming them all
// (gorge scheduleAtEOTGroup, CR 603.7a/b): the first object's tracked exile
// record is the head, put on the stack at the end step; every later object
// joins it as a dlyGrpFollow record (act OpExile) that is never put on the
// stack itself. The head's resolution exiles every member still on the
// battlefield as the incarnation registered, at once.

// DebugExileGroupOff makes every exile-rider token register its own trigger
// (the pre-group shape); the lockstep mutation test flips it and requires a
// divergence.
var DebugExileGroupOff bool

// exileAtEndJoin registers token t, made after another exile-rider token by the
// same resolution, as a follower of the head's exile, and reports whether it
// did.
//
//go:noinline
func (s *State) exileAtEndJoin(t Obj) bool {
	if s.ndly == 0 || s.ndly >= MaxDelayed || DebugExileGroupOff {
		return false
	}
	prev := &s.dly[s.ndly-1]
	if prev.act != uint8(kop.OpExile) {
		return false
	}
	head := prev.obj
	switch {
	case prev.when == dlyGrpFollow:
		head = prev.creator
	case prev.when != 0 || prev.creator != s.res.e.obj || prev.obj == t:
		return false
	}
	s.dly[s.ndly] = delayed{act: uint8(kop.OpExile), ctrl: s.res.e.ctrl, obj: t, ver: objWord(s.objs[t].ver), when: dlyGrpFollow, creator: head}
	s.ndly++
	return true
}

// resolveExileAtEnd resolves a next-end-step exile record: the head (when its
// registered incarnation is still on the battlefield) and its followers,
// consumed here, leave together. A record without followers is the plain
// one-object exile.
//
//go:noinline
func (s *State) resolveExileAtEnd(k *prep, e *stackEntry) {
	head := dlyRef(e.obj, e.trigObj)
	var hit slotMask
	if ob := &s.objs[head]; ob.ver == e.srcVer && ob.zone == kop.ZoneBattlefield {
		hit = hit.with(ob.slot)
	}
	w := uint8(0)
	for i := uint8(0); i < s.ndly; i++ {
		d := s.dly[i]
		if d.when == dlyGrpFollow && d.creator == head && d.act == uint8(kop.OpExile) {
			if ob := &s.objs[d.obj]; objWord(ob.ver) == d.ver && ob.zone == kop.ZoneBattlefield {
				hit = hit.with(ob.slot)
			}
			continue
		}
		s.dly[w] = d
		w++
	}
	for i := w; i < s.ndly; i++ {
		s.dly[i] = delayed{}
	}
	s.ndly = w
	switch hit.count() {
	case 0:
	case 1:
		s.leave(k, hit.first(), kop.ZoneExile, false)
	default:
		s.leaveAll(k, hit, kop.ZoneExile)
	}
}
