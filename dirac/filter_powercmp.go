package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwPowerCmp: FwPowerCmp, the object's current power compared to the
// literal's V by Op. A permanent reads its layered power, a card off the
// battlefield the printed, CDA, face-down or last-known one (cardPT), as
// PredPowerLE and PredPowerGE do. A layer-walk view has no power to read
// without re-entering the walk, so it fails closed.
//
//kmanifest:fword FwPowerCmp
func (s *State) fwPowerCmp(c *fctx, l *kop.FLit) bool {
	switch {
	case c.walk:
		return false
	case c.perm():
		pw, _ := s.ptGen(c.k, c.sl)
		return s.cmpV(c, l, pw)
	case c.card():
		pw, _ := s.cardPT(c.k, c.c, c.owner, c.o, c.fd)
		return s.cmpV(c, l, pw)
	}
	return false
}
