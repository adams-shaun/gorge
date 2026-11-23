package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwChosenCtrl: FwChosenCtrl, the battlefield permanent is controlled by the
// player the reading source permanent chose as it entered (gorge
// chosenCtrlMatches, engine/effects/filter_type.go:264: the source's Chosen
// player target against the candidate's controller). No source or no chosen
// player fails closed, as gorge's "no chosen player never matches".
//
//kmanifest:fword FwChosenCtrl
func (s *State) fwChosenCtrl(c *fctx, l *kop.FLit) bool {
	if !c.perm() {
		return false
	}
	ch := s.chosenPlayerOf(c.src)
	return ch != 255 && s.perms[c.sl].ctrl == ch
}
