package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// tapOrUntap is OpTapOrUntap (gorge effTapOrUntap): the controller is
// asked whether to tap or untap the permanent Obj, while it is on the
// battlefield; nothing otherwise. false: the ask is posed.
func (s *State) tapOrUntap(k *prep, in *kop.Instr) bool {
	if s.objRef(k, in.Obj) == NoSlot {
		return true
	}
	s.res.ask, s.res.askP = raGenTapOrUntap, s.res.e.ctrl
	return false
}
