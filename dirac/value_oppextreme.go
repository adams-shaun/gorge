package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// oppExtreme is ExOppExtreme: a figure over the living players other than
// ctrl, walked in seat order as gorge walks opponentGroup's slice (never a
// map). OppExtAmount counts them (gorge playerGroupCount); the rest take the
// highest, or with OppExtLowest the lowest, of one figure per opponent
// (gorge lifeExtreme, playerCountExtreme). An empty group reads 0: gorge
// reports a life extreme unresolvable there, and the group is empty only at
// the game's end, where no later read decides anything.
func (s *State) oppExtreme(k *prep, x *kop.Expr, ctx evalCtx) int {
	what := int(x.N) & 0xff
	lowest := int(x.N)&kop.OppExtLowest != 0
	best, seen := 0, false
	for p := uint8(0); p < s.nSeats(); p++ {
		if p == ctx.ctrl || s.out(p) {
			continue
		}
		v := 1
		pl := &s.pl[p]
		switch what {
		case kop.OppExtLife:
			v = int(pl.life)
		case kop.OppExtHand:
			v = int(pl.handN)
		case kop.OppExtGrave:
			v = 0
			for j := 0; j < int(pl.graveN); j++ {
				if s.objs[pl.grave[j]].of&ofToken == 0 {
					v++ // a token that died is no card (CR 111.7), as ExCountGraveyard
				}
			}
		case kop.OppExtValid:
			at := ctx
			at.ctrl = p
			v = s.countPerm(k, x.F, at)
		}
		if what == kop.OppExtAmount {
			if !DebugOppCountOne || best == 0 {
				best++
			}
			continue
		}
		if !seen || (lowest && v < best) || (!lowest && v > best) {
			best, seen = v, true
		}
	}
	return best
}

// DebugOppCountOne is a test-only mutation: the controller's living
// opponents count at most one, as in a two-player game.
var DebugOppCountOne bool
