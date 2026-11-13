package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// rearrange is OpRearrange (gorge effRearrangeTopOfLibrary): the top A
// cards of player Obj's library (at most what it holds) are looked at by
// the controller, who orders all of them (raGenRearrange); none: nothing.
// false: the ask is posed.
func (s *State) rearrange(k *prep, in *kop.Instr) bool {
	p := s.playerRef(k, in.Obj)
	if p >= MaxSeats {
		return true
	}
	n := s.value(k, in.A)
	pl := &s.pl[p]
	if n > pl.libLen() {
		n = pl.libLen()
	}
	if n <= 0 {
		return true
	}
	r := &s.res
	if n > len(r.peek) {
		s.capFull(StallTables)
		return true
	}
	for i := 0; i < n; i++ {
		r.peek[i] = pl.lib[int(pl.libLo)+i]
	}
	r.peekN, r.peekI, r.peekSel = uint8(n), 0, 0
	r.ask, r.askP = raGenRearrange, s.res.e.ctrl
	return false
}

// rearrangeDone puts the looked-at cards back on top of their owner's
// library in the picked order (the first pick on top); with C&1
// (MayShuffle$) it then asks whether to shuffle that library (false).
func (s *State) rearrangeDone(k *prep) bool {
	r := &s.res
	r.ask = raNone
	in := &s.prog(k)[r.pc]
	p := s.playerRef(k, in.Obj)
	pl := &s.pl[p]
	n := int(r.peekN)
	for i := 0; i < n; i++ {
		s.touch(r.peek[i])
		s.objs[r.peek[i]].zone = kop.ZoneNone
	}
	pl.libLo += uint8(n)
	for i := n - 1; i >= 0; i-- {
		o := r.arr[i]
		s.objs[o].zone = kop.ZoneLibrary
		pl.libTop(o)
	}
	r.peekN, r.peekI, r.peekSel = 0, 0, 0
	if in.C&1 != 0 { // seat-ok: an instruction flag bit
		r.ask, r.askP = raGenRearrShuffle, s.res.e.ctrl
		return false
	}
	return true
}
