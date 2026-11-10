package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// replLoss applies ReplLifeLossDouble (Bloodletter of Aclazotz,
// gorge LifeReduced ReplaceCount$Amount/Twice): each permanent with the
// replacement whose controller is p's opponent doubles the life p would
// lose during that controller's turn -- a LoseLife effect, damage's life
// loss (the damage itself is unchanged: cr-lifereduced-twice-not-damage)
// and a life cost alike. The doublers commute (gorge
// lifeReplacementsCommute), so each applies once in any order. Only with
// prep's rbDblLoss.
//
//go:noinline
func (s *State) replLoss(k *prep, p uint8, n int) int {
	q := s.active // the doubler's controller: an opponent of p, on its own turn
	if q == p {
		return n
	}
	g := k.gen
	for m := s.statM.and(s.ctrlM[q]); m.any(); m = m.rest() {
		sl := m.first()
		if g.replBits[s.cardAt(sl)]&rbDblLoss == 0 || s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			if a := &cd.Abilities[i]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplLifeLossDouble && n <= 0x3fff {
				n *= 2
			}
		}
	}
	return n
}
