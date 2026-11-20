package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// lifeLostOf is ExLifeLostThisTurn: the ref-less form is the controller's
// own tally; a player ref (RefTargetController from a targeted player,
// CR 119.3) is that player's tally (gorge's per-player lifeLost).
//
//go:noinline
func (s *State) lifeLostOf(k *prep, r kop.Ref, ctx evalCtx) int {
	if r != kop.RefNone {
		if p := s.refPlayer(k, r, ctx); p != 255 {
			return int(s.pl[p].lifeLost)
		}
		return 0
	}
	return int(s.pl[ctx.ctrl].lifeLost)
}
