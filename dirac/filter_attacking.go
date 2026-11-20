package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwAttacking: FwAttacking, the battlefield permanent is attacking
// (FAttacking). An object elsewhere is attacking nothing, so it fails closed.
//
//kmanifest:fword FwAttacking
func (s *State) fwAttacking(c *fctx, l *kop.FLit) bool {
	return c.perm() && s.perms[c.sl].flags&pfAttacking != 0
}
