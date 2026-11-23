package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwBaseToughnessCmp: FwBaseToughnessCmp, gorge numericPred's baseToughness
// field (engine/effects/filter.go:1747, the values at filter_value.go:292):
// the base toughness through layer 7b (CR 613.4). A permanent reads
// basePTGen (the layer walk included); a card off the battlefield its
// printed, CDA-folded toughness, the off-battlefield boost check included
// (cardPT). The P/T-less non-creature reads no toughness (gorge lacksPT,
// filter_value.go:679), as fwBasePowerCmp does.
//
//kmanifest:fword FwBaseToughnessCmp
func (s *State) fwBaseToughnessCmp(c *fctx, l *kop.FLit) bool {
	switch {
	case c.perm():
		if c.k.t.Cards[s.cardAt(c.sl)].NoPT && s.typesGen(c.k, c.sl)&kop.TCreature == 0 {
			return false // gorge lacksPT: a P/T-less non-creature reads no toughness
		}
		_, bt := s.basePTGen(c.k, c.sl)
		return s.cmpV(c, l, bt)
	case c.card():
		if c.k.t.Cards[c.c].NoPT {
			return false // off the battlefield the printed type is the read
		}
		_, t := s.cardPT(c.k, c.c, c.owner, c.o, c.fd)
		return s.cmpV(c, l, t)
	}
	return false
}
