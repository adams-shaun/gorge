package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// PlayerCountOpponents$HighestCounters.Poison
// (gorge effects/count_player.go playerCountExtreme's Counters.Poison arm,
// the ONE "Corrupted" statics and gates). Cold and out of line: a table
// without the expression never reads a poison extreme
// (genPrep.anyPoisonHigh).

// DebugPoisonHighSelf makes the kernel read the controller's own poison
// count where the opponents' highest belongs. The lockstep mutation test
// flips it and requires a divergence.
var DebugPoisonHighSelf bool

// prepPoisonHigh sets anyPoisonHigh: some expression is the opponents'
// highest poison count. The read then stays off every table that has no
// such expression.
func prepPoisonHigh(t *kop.Table, g *genPrep) {
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExHighestOppPoison {
			g.anyPoisonHigh = true
			return
		}
	}
}

// highestOppPoison is PlayerCountOpponents$HighestCounters.Poison: the
// greatest poison count among the living players other than ctrl, walked in
// seat order (gorge playerCountExtreme walks its group slice, never a map).
// With two players the group is the one living opponent; an empty group
// reads 0 -- gorge reports it unresolvable (0, false), and the only way the
// group is empty is the game's end, where no later read decides anything.
//
//go:noinline
func (s *State) highestOppPoison(ctrl uint8) int {
	if DebugPoisonHighSelf {
		return int(s.g.poison[SeatIx(ctrl)])
	}
	best := 0
	for p := uint8(0); p < MaxSeats; p++ {
		if p == ctrl || s.pl[p].lost {
			continue
		}
		if v := int(s.g.poison[p]); v > best {
			best = v
		}
	}
	return best
}
