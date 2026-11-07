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
	p := int(s.nextActive())
	n := 0
	for i := 0; i < int(s.g.nemb[p]); i++ {
		if k.t.Emblems[s.g.emb[p][i]-1].UntilNextTurn {
			continue
		}
		s.g.emb[p][n], s.g.embC[p][n], s.g.embO[p][n], s.g.embN[p][n] = s.g.emb[p][i], s.g.embC[p][i], s.g.embO[p][i], s.g.embN[p][i]
		s.g.embT[p][n] = s.g.embT[p][i]
		n++
	}
	for i := n; i < int(s.g.nemb[p]); i++ {
		s.g.emb[p][i], s.g.embC[p][i], s.g.embO[p][i], s.g.embN[p][i] = 0, 0, 0, 0
		s.g.embT[p][i] = 0
	}
	s.g.nemb[p] = uint8(n)
}
