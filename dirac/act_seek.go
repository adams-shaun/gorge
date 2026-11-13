package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// seek is OpSeek (gorge effSeek, one Type$): the eligible cards of player
// Obj's library in library order from the top (gorge's zone slice), A of
// them drawn without replacement by one rng IntN(remaining) each, then
// each moved to its owner's hand in the order drawn. C&1 (RememberFound$):
// the found cards become the resolution's remembered set.
//
//go:noinline
func (s *State) seek(k *prep, in *kop.Instr) {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		return
	}
	pl := &s.pl[p]
	var elig [MaxObj]Obj
	ne := 0
	for _, o := range pl.lib[pl.libLo:pl.libHi] {
		if s.matchCardGen(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o) {
			elig[ne] = o
			ne++
		}
	}
	n := s.value(k, in.A)
	if n > ne {
		n = ne
	}
	var sel [MaxObj]Obj
	for i := 0; i < n; i++ {
		j := s.rng.IntN(ne)
		sel[i] = elig[j]
		copy(elig[j:ne-1], elig[j+1:ne])
		ne--
	}
	if in.C&1 != 0 { // seat-ok: an instruction flag bit
		// RememberFound$: the found cards REPLACE the remembered set,
		// emptied even when nothing is found
		s.g.nrem, s.g.remCnt = 0, 0
	}
	for _, o := range sel[:max(n, 0)] {
		from := s.detach(o)
		s.placeFrom(o, kop.ZoneHand, false, from, false)
		if in.C&1 != 0 { // seat-ok: an instruction flag bit
			s.remember(o)
		}
	}
}
