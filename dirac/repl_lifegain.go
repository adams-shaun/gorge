package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// replLife is the life player p gains instead of n after the life-gain
// replacements p's permanents carry (gorge applyLifeReplacements: each
// applies once; doublers commute among themselves, plus-N ones too; a
// gain both kinds compete for is gorge's CR 616.1 order ask, posed by the
// resolving GainLife op (repl_lifeask.go) and a stall at any other site).
//
//go:noinline
func (s *State) replLife(k *prep, p uint8, n int) int {
	g := k.gen
	for m := s.statM.and(s.ctrlM[p]); m.any(); m = m.rest() {
		sl := m.first()
		if s.genLost(k, sl) {
			continue
		}
		// the top card, then a pile's under cards (CR 702.140d:
		// eachPileCard; kw_mutateunion.go)
		s.eachPileCard(k, sl, func(c CardID) bool {
			if g.replBits[c]&(rbDblLife|rbPlusLife) == 0 {
				return true
			}
			cd := &k.t.Cards[c]
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
			return true
		})
	}
	return n
}
