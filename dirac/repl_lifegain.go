package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// replLife is the life player p gains instead of n after the life-gain
// replacements p's permanents carry (gorge applyLifeReplacements: each
// applies once; doublers commute among themselves, plus-N ones too; a
// table holding both stalls at its start, StallLifeRepl).
//
//go:noinline
func (s *State) replLife(k *prep, p uint8, n int) int {
	g := k.gen
	for m := s.statM.and(s.ctrlM[p]); m.any(); m = m.rest() {
		sl := m.first()
		b := g.replBits[s.cardAt(sl)]
		if b&(rbDblLife|rbPlusLife) == 0 || s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement {
				continue
			}
			switch a.Repl.Kind {
			case kop.ReplLifeDouble:
				if n <= 0x3fff {
					n *= 2
				}
			case kop.ReplLifePlus:
				n += int(a.Repl.A)
			}
		}
	}
	return n
}
