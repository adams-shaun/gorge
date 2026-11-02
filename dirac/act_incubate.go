package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugIncubateNoTransform makes a transforming DFC token stay on its front
// face. The lockstep mutation test flips it to prove the transform matters.
var DebugIncubateNoTransform bool

// incubate is OpIncubate (gorge effIncubate, CR 701.53a-b): B times (at
// least once) create the Incubator token C under p's control -- token
// replacements may make several per mint -- and put A +1/+1 counters on
// each mint through the counter replacements (CounterAddedOnce). A < 0
// creates nothing; A = 0 still mints, with no counter event. The counters
// are part of the entry (CR 701.53a "enters with"; gorge
// entryWithCounters). A resolution window opened by TokenCreated is not
// suspended for the remaining mints (amass does not either).
//
//go:noinline
func (s *State) incubate(k *prep, in *kop.Instr) {
	n := s.value(k, in.A)
	if n < 0 {
		return
	}
	times := s.value(k, in.B)
	if times < 1 {
		times = 1
	}
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		p = s.res.e.ctrl
	}
	for ; times > 0; times-- {
		t := s.replTokens(k, p, CardID(in.C), 1)
		for i := 0; i < t; i++ {
			if s.nearCapacity() {
				s.stall(StallBoard)
				return
			}
			o := s.newToken(CardID(in.C), p)
			if n > 0 {
				// the Incubator enters with its counters (gorge
				// entryWithCounters, CR 122.6, 614.1c): the staged entry
				// counters are placed through the counter replacements in
				// enterGen, before the enters triggers see the token
				s.g.enterCtr.n, s.g.enterCtr.kind = uint8(n), kop.CtrP1P1
			}
			sl := s.enter(k, o, p, false, kop.ZoneLibrary)
			s.g.enterCtr.n = 0
			if k.gen.anyTokCr {
				s.fireTokenCreated(k, sl)
			}
			s.res.lastTok = o
		}
	}
}
