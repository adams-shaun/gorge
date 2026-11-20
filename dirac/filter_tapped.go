package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwTapped: FwTapped, the battlefield permanent is tapped (FTapped). Tapped
// is a battlefield status, so an object elsewhere fails closed.
//
//kmanifest:fword FwTapped
func (s *State) fwTapped(c *fctx, l *kop.FLit) bool {
	return c.perm() && s.perms[c.sl].flags&pfTapped != 0
}
