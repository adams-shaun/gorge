package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwToughnessCmp: FwToughnessCmp, fwPowerCmp's read of toughness
// (PredToughnessLE).
//
//kmanifest:fword FwToughnessCmp
func (s *State) fwToughnessCmp(c *fctx, l *kop.FLit) bool {
	switch {
	case c.walk:
		return false
	case c.perm():
		_, t := s.ptGen(c.k, c.sl)
		return s.cmpV(c, l, t)
	case c.card():
		_, t := s.cardPT(c.k, c.c, c.owner, c.o, c.fd)
		return s.cmpV(c, l, t)
	}
	return false
}
