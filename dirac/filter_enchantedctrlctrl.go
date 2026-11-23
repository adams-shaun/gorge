package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwEnchantedControllerCtrl: FwEnchantedControllerCtrl, the battlefield
// permanent is controlled by the controller of the permanent the reading
// source is attached to (gorge wordEnchantedControllerCtrl,
// engine/effects/filter_word.go:1149, through playerEnchantedController,
// player_filter.go:1037). A source attached to nothing, or to no permanent
// still there, fails closed.
//
//kmanifest:fword FwEnchantedControllerCtrl
func (s *State) fwEnchantedControllerCtrl(c *fctx, l *kop.FLit) bool {
	if !c.perm() || c.src == NoSlot || !s.occ.has(c.src) {
		return false
	}
	h := s.perms[c.src].attached
	return h != NoSlot && s.occ.has(h) && s.perms[c.sl].ctrl == s.perms[h].ctrl
}
