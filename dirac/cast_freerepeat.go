package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Play with AllowRepeats$ True and Amount$ X (OpCastFree C bit 7, A = X):
// "copy that card X times ... you may cast the copies" (Infinite Diversity,
// CR 707.12). Gorge (effects/play.go) offers the candidate list X times over
// as one multi-pick ask, a KModes decision with Max X, where each entry taken
// is one copy cast; the answer may stop at any count (Optional$).
//
// The kernel mirrors that list in the arrays it already has: free and freeAll
// hold the card once per copy, so the picks, the Done and the queue (freeQ,
// one entry per copy; each drains through beginCopyCast, which mints a fresh
// copy) need no new state and no State byte. A count over the arrays' eight
// places stalls loud (StallFree). A count below one offers nothing: gorge
// casts no copy for X = 0 (CR 707.12; the Amount$ read of effects/play.go).
//
// The offer lists the card once, at its first place not yet picked, and the
// lockstep's gorge side collapses the identical entries the same way (the
// copies are interchangeable); CastFreeIndex gives that place.

// DebugRepeatNoOptional makes the kernel cast every copy without the
// per-copy decision (mutation test only): X picks queued at once.
var DebugRepeatNoOptional bool

// castFreeRepeat repeats the accepted candidates X times, block by block as
// gorge lists them (once, once, ...).
func (s *State) castFreeRepeat(k *prep, in *kop.Instr) {
	n := s.value(k, in.A)
	once := int(s.g.nfree)
	if n < 1 {
		s.g.nfree = 0
		return
	}
	if n == 1 || once == 0 {
		return
	}
	var blk [8]Obj
	copy(blk[:], s.g.free[:once])
	for r := 1; r < n; r++ {
		for _, o := range blk[:once] {
			if int(s.g.nfree) >= len(s.g.free) {
				s.capFull(StallFree)
				return
			}
			s.g.free[s.g.nfree] = o
			s.g.nfree++
		}
	}
}

// freeRepeatForce queues every copy still on offer (mutation only: no
// per-copy decision).
func (s *State) freeRepeatForce() {
	for i := 0; i < int(s.g.nfree) && int(s.g.nfq) < len(s.g.freeQ); i++ {
		s.g.freeQ[s.g.nfq] = s.g.free[i]
		s.g.nfq++
	}
	s.g.nfree = 0
}

// freeRepeated reports whether free[i] already stands earlier in the list: a
// repeated entry is not offered again.
func (s *State) freeRepeated(i int) bool {
	for j := 0; j < i; j++ {
		if s.g.free[j] == s.g.free[i] {
			return true
		}
	}
	return false
}

// freeRepeatIndex is the place in gorge's list of the next copy of o to be
// picked, when the frozen list holds o more than once: the (picked+1)-th
// place of o, picked being the copies already taken. ok is false for a card
// that stands once (every other Play).
func (s *State) freeRepeatIndex(o Obj) (uint8, bool) {
	total, left := 0, 0
	for i := 0; i < int(s.g.nfreeAll); i++ {
		if s.g.freeAll[i] == o {
			total++
		}
	}
	if total < 2 {
		return 0, false
	}
	for i := 0; i < int(s.g.nfree); i++ {
		if s.g.free[i] == o {
			left++
		}
	}
	skip := total - left // places already taken
	for i := 0; i < int(s.g.nfreeAll); i++ {
		if s.g.freeAll[i] == o {
			if skip == 0 {
				return uint8(i), true
			}
			skip--
		}
	}
	return uint8(s.g.nfreeAll) - 1, true
}
