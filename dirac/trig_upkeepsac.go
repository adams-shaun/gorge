package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CopyPermanent AtEOT$ SacrificeUpkeep (Firion, Wild Rose Warrior: "Sacrifice
// that token at the beginning of the next upkeep"). gorge's atEOTRiders maps it
// to the same untracked __kwEncoreSacrifice body as AtEOT$ Sacrifice, fired at
// the next upkeep of ANY player (YourSacrificeUpkeep is the gated one). The
// kernel registers it as the untracked one-shot dlyUntracked record under
// another trigger step, dlyUpkeep, which fireDelayed leaves in place and
// fireDlyUpkeep puts on the stack as an upkeep begins; resolution is
// resolveSacUntracked's, so a token that already left does nothing (CR 603.7c).

// dlyUpkeep is delayed.when for a record fired at the next upkeep step.
const dlyUpkeep = 5

// sacAtUpkeepOf reports whether copy flags fl are the SacrificeUpkeep rider:
// both end-step rider bits at once (kop.CpSacAtUpkeep), which neither rider
// emits alone.
func sacAtUpkeepOf(fl int16) bool {
	return fl&kop.CpSacAtUpkeep == kop.CpSacAtUpkeep
}

// sacAtUpkeepUntracked registers token t's untracked next-upkeep sacrifice
// under ctrl.
//
//go:noinline
func (s *State) sacAtUpkeepUntracked(t Obj, ctrl uint8) {
	if s.ndly < MaxDelayed {
		s.dly[s.ndly] = delayed{act: uint8(kop.OpSacrifice), ctrl: ctrl, obj: t, ver: s.objs[t].ver, when: dlyUpkeep, creator: s.res.e.obj}
		s.ndly++
	} else {
		s.capFull(StallDelayed)
	}
}

// delayEmptyUpkeep is delayEmpty's record due at the next upkeep: the rider
// registered over no token (gorge scheduleAtEOTNone), doing nothing there.
//
//go:noinline
func (s *State) delayEmptyUpkeep(e *stackEntry) {
	if e.obj == NoObj {
		return
	}
	if s.ndly >= MaxDelayed {
		s.capFull(StallDelayed)
		return
	}
	s.dly[s.ndly] = delayed{ctrl: e.ctrl, obj: e.obj, ver: e.srcVer, when: dlyUpkeep, creator: NoObj}
	s.ndly++
}

// fireDlyUpkeep enqueues the dlyUpkeep records as the upkeep step begins,
// after the upkeep's delayed programs, and keeps the other records in order
// (the group followers a head's trigger carries included).
//
//go:noinline
func (s *State) fireDlyUpkeep(k *prep) {
	n := s.ndly
	var keep uint8
	for i := uint8(0); i < n; i++ {
		d := s.dly[i]
		if d.when != dlyUpkeep {
			s.dly[keep] = d
			keep++
			continue
		}
		if d.act == uint8(kop.OpNop) {
			s.enqueue(d.obj, d.ver, s.objs[d.obj].card, abDelayed, d.ctrl, NoObj, d.ctrl, int16(d.act))
			continue
		}
		s.enqueue(dlySource(&d), d.ver, s.objs[d.obj].card, abDelayed, d.ctrl, d.obj, d.ctrl, opSacUntracked)
	}
	for i := keep; i < n; i++ {
		s.dly[i] = delayed{}
	}
	s.ndly = keep
}
