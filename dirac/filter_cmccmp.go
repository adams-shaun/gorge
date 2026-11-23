package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwCMCCmp: FwCMCCmp, the object's mana value compared to the literal's V by
// Op (PredCMCLE, PredCMCGE, PredCMCEQ). A permanent reads cmcPerm (a face-down
// one has none, CR 708.2), a card off the battlefield cardMVIn (a spell on the
// stack counts its X, CR 202.3e).
//
//kmanifest:fword FwCMCCmp
func (s *State) fwCMCCmp(c *fctx, l *kop.FLit) bool {
	switch {
	case c.walk:
		return false
	case c.perm():
		return s.cmpV(c, l, s.cmcPerm(c.k, c.sl))
	case c.card():
		var cx *evalCtx // a copy, so the filter's own context stays out of reach
		if c.hasCtx {
			v := c.ctx
			cx = &v
		}
		return s.cmpV(c, l, s.cardMVIn(c.k, c.c, c.o, c.fd, cx))
	}
	return false
}
