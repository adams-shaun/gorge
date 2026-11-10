package dirac

// Effect-delivered Continuous statics.

// dropNextTurnEmblems removes the emblems whose Duration$ UntilYourNextTurn
// ends at the cleanup before their controller's next turn begins. gorge
// computes the boundary as nextTurnFor - 1 (layers_effect.go), so the drop
// happens when the controller is the next active player -- the same test
// dropNextTurnOv uses for OpOverride records. Emblems are not objects, so
// there is no slot to touch; the per-player arrays are compacted in place.
//
//go:noinline
func (s *State) dropNextTurnEmblems(k *prep) {
	p := s.nextActive()
	var drop uint8
	for i := 0; i < int(s.g.nemb[p]); i++ {
		if k.t.Emblems[s.embIdx(p, i)-1].UntilNextTurn {
			drop |= 1 << uint(i)
		}
	}
	if drop != 0 {
		s.embCompact(uint8(p), drop)
	}
}
