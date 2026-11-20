package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwCountersCmp: FwCountersCmp, the number of counters of kind Arg on the
// battlefield permanent compared to the literal's V by Op (PredCountersGE and
// PredHasCounter are its CmpGE forms). An object off the battlefield fails
// closed: its last-known counters are a card matcher's own read.
//
//kmanifest:fword FwCountersCmp
func (s *State) fwCountersCmp(c *fctx, l *kop.FLit) bool {
	if !c.perm() || l.Arg < 0 {
		return false
	}
	return s.cmpV(c, l, s.perms[c.sl].ctrN(s, int(l.Arg)))
}
