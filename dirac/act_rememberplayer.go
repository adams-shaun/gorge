package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// rememberPlayerOp is OpExt ExtRememberPlayer: it records the player a Pump's
// RememberObjects$ TargetedController named as the resolution's player
// (res.chPl, seat + 1, 0 none), which the Draw after the chained GainControl
// reads as RefResChosenPlayer. The Pump resolves before the control change, so
// the Draw pays the permanent's old controller while the permanent is already
// under the new one (CR 608.2c: instructions are followed in order).
//
//kmanifest:ext ExtRememberPlayer
func (s *State) rememberPlayerOp(k *prep, in *kop.Instr) (ok, done bool) {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		s.res.chPl = 0
	} else {
		s.res.chPl = p + 1
	}
	return true, true
}
