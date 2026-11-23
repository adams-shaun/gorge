package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwBasePowerCmp: FwBasePowerCmp, gorge numericPred's basePower field
// (engine/effects/filter.go:1747, the values at filter_value.go:292): the
// base power through layer 7b (CR 613.4, printed or
// characteristic-defining, then a 7b set, before 7c modifies and 7d
// counters). A permanent reads basePTGen (the layer walk included); a card
// off the battlefield its printed, CDA-folded power, the off-battlefield
// boost check included (cardPT). An object that prints no power and is not
// a creature has none to read: gorge's lacksPT (filter_value.go:679) binds
// the whole compare to false there, so the words fail closed on it too.
//
//kmanifest:fword FwBasePowerCmp
func (s *State) fwBasePowerCmp(c *fctx, l *kop.FLit) bool {
	switch {
	case c.perm():
		if c.k.t.Cards[s.cardAt(c.sl)].NoPT && s.typesGen(c.k, c.sl)&kop.TCreature == 0 {
			return false // gorge lacksPT: a P/T-less non-creature reads no power
		}
		bp, _ := s.basePTGen(c.k, c.sl)
		return s.cmpV(c, l, bp)
	case c.card():
		if c.k.t.Cards[c.c].NoPT {
			return false // off the battlefield the printed type is the read
		}
		pw, _ := s.cardPT(c.k, c.c, c.owner, c.o, c.fd)
		return s.cmpV(c, l, pw)
	}
	return false
}
