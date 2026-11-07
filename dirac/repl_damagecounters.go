package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugDmgPlusCtrOff mutates a ReplDmgCtrHost replacement to add 0 instead of
// the host's counters (TestFiredmgMutation).
var DebugDmgPlusCtrOff bool

// dmgPlusAmt: the amount a ReplDamagePlus replacement hosted at slot b adds:
// Repl.A, or with ReplDmgCtrHost the host's Repl.A-kind counters now
// (gorge rewriteReplaceCountOperand, effects/replacement.go:54-71), or with
// ReplDmgPowHost the host's current power. gorge adds a negative power as
// is and floors the total at 0 (CR 107.1b), which does not commute with the
// other additions' order: a negative-power host stalls loud (StallReplPlus)
// and adds 0.
func (s *State) dmgPlusAmt(k *prep, r *kop.Repl, b Slot) int {
	if r.C&kop.ReplDmgPowHost != 0 {
		pw, _ := s.ptGen(k, b)
		if pw < 0 {
			s.capFull(StallReplPlus)
			return 0
		}
		return pw
	}
	if r.C&kop.ReplDmgCtrHost != 0 {
		if DebugDmgPlusCtrOff {
			return 0
		}
		return s.perms[b].ctrN(s, int(r.A))
	}
	return int(r.A)
}
