package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ExtCounterPlusEOT: until end of turn, if one or more counters would be put
// on a permanent a player controls, that many plus one of each of those kinds
// are put on it instead (Brad Boimler, Eager Ensign). gorge lowers the
// tap-triggered Effect's AddCounter replacement with no ValidCounterType$;
// the effect's own ActiveZones$ gate is not consulted after it resolves, so
// the flag lives to cleanup regardless of the source leaving.

// DebugCtrPlusEOTOff is the mutation switch: the flag is set but ignored, so
// a lockstep that puts counters after the tap diverges.
var DebugCtrPlusEOTOff bool

// CtrPlusEOTStats counts the lockstep test's coverage: placements the flag
// made one larger.
var CtrPlusEOTStats struct{ Applied atomic.Int64 }

// prepCtrPlusEOT sets genPrep.anyRepl's rbPlusCtrEOT when some instruction
// is ExtCounterPlusEOT.
func prepCtrPlusEOT(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.A.K == kop.VConst && kop.ExtOp(in.A.N) == kop.ExtCounterPlusEOT {
					g.anyRepl |= rbPlusCtrEOT
				}
			}
		}
	}
}

// ctrPlusEOT is ExtCounterPlusEOT: mark the resolving controller's permanents
// for the rest of the turn. Each activation is its own replacement effect, so
// the per-player count stacks (Twiddle can tap Boimler again); the seats stay
// apart.
func (s *State) ctrPlusEOT(k *prep) {
	p := s.res.e.ctrl
	if s.g.plusCtr[p] < 255 {
		if s.g.plusCtr[p] == 0 {
			s.g.plusSrc[p] = s.res.e.obj
		} else if s.g.plusSrc[p] != s.res.e.obj {
			s.g.plusSrc[p] = NoObj // two Boimlers: the order ask stalls
		}
		s.g.plusCtr[p]++
	}
}
