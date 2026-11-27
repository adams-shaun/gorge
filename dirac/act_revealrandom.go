package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// revealRandomExt is ExtRevealRandom (Urza's Bauble's Reveal Random$ True over
// a target player's hand). gorge effReveal narrows the hand pool to one card
// with a single h.Rand(len(pool)) draw when the pool is non-empty, then only
// emits a Note: nothing moves and nothing is asked, so the draw is the whole
// state change. An empty hand draws nothing; Obj resolving to no player (the
// target gone) does nothing.
//
//kmanifest:ext ExtRevealRandom
func (s *State) revealRandomExt(k *prep, in *kop.Instr) (ok, done bool) {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		return true, true
	}
	if m := int(s.pl[p].handN); m > 0 {
		s.rng.IntN(m)
	}
	return true, true
}
