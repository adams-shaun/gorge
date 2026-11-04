package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Brazen Bluff's optional exile of a targeted spell: "that player may exile
// it" is a ChangeZone Origin$ Stack Optional$ True whose Chooser$ is the
// spell's controller (TargetedController). gorge asks that player a yes/no
// (changeZoneOptionalConfirm) for each object the move would move; a "no"
// drops the move only and the chain goes on. The kernel poses it as an
// OpMayAsk (MayGuardStackSpell) over an OpCounter with CtrDestExile and
// CtrRemember, the asked player being the OpMayAsk's Obj (the target's
// controller as it last existed). The remembered set is per resolution, so
// the follow-up free cast's "nothing was remembered" gate (ConditionDefined$
// Remembered, ConditionCompare$ EQ0) needs no State byte.

// DebugBluffNoRemember is a mutation switch for the lockstep tests: the
// exiled spell is not remembered, so the free cast is allowed after an
// exile as well.
var DebugBluffNoRemember bool

// stackSpellAsks is MayGuardStackSpell: the next OpCounter's Obj is a spell
// still on the stack. Otherwise gorge has no object to offer, nothing is
// asked and the move is skipped (A instructions).
//
//go:noinline
func (s *State) stackSpellAsks(k *prep, in, next *kop.Instr) bool {
	if o := s.objRefObj(next.Obj); o != NoObj {
		if i := s.stackIndexOf(o); i >= 0 && s.stack[i].kind == seSpell {
			return true
		}
	}
	s.res.pc += uint8(s.value(k, in.A))
	return false
}
