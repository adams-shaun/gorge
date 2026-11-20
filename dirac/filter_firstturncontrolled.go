package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwFirstTurnControlled: FwFirstTurnControlled, the battlefield permanent
// came under its controller's control since their most recent turn began
// (gorge engine/effects/filter.go predicates "firstTurnControlled":
// Zone == ZBattlefield && SummonSick). pfSick is that flag for every
// permanent, not only creatures. An object elsewhere fails closed, so the
// literal's Neg ("!firstTurnControlled") holds there, as gorge's does.
//
//kmanifest:fword FwFirstTurnControlled
func (s *State) fwFirstTurnControlled(c *fctx, l *kop.FLit) bool {
	return c.perm() && s.perms[c.sl].flags&pfSick != 0
}
