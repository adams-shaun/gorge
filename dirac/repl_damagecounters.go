package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugDmgPlusCtrOff mutates a ReplDmgCtrHost replacement to add 0 instead of
// the host's counters (TestFiredmgMutation).
var DebugDmgPlusCtrOff bool

// dmgPlusAmt: the amount a ReplDamagePlus replacement hosted at slot b adds:
// Repl.A, or with ReplDmgCtrHost the host's Repl.A-kind counters now
// (gorge rewriteReplaceCountOperand, effects/replacement.go:54-71).
func (s *State) dmgPlusAmt(r *kop.Repl, b Slot) int {
	if r.C&kop.ReplDmgCtrHost != 0 {
		if DebugDmgPlusCtrOff {
			return 0
		}
		return s.perms[b].ctrN(s, int(r.A))
	}
	return int(r.A)
}
