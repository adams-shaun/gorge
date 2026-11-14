package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// pilesToGrave moves the revealed cards ord (pilesGraveOrder) to the graveyard
// one at a time. Gorge asks the CR 616.1 order before each move whose
// exile-instead replacements compete (Fact or Fiction's unchosen pile beside
// two Rest in Peace), so the first such card and every later one wait in the
// queue, competing or not, still on the library list; the answers move them in
// turn and the last one continues the resolution (continueRes). The earlier
// cards move at once.
func (s *State) pilesToGrave(k *prep, ord []Obj) {
	first := -1
	if k.gen != nil {
		for i, o := range ord {
			if s.graveMoveCompetes(k, o) {
				first = i
				break
			}
		}
	}
	if first < 0 {
		first = len(ord)
	}
	for _, o := range ord[:first] {
		s.pileMove(o, kop.ZoneGraveyard)
	}
	for _, o := range ord[first:] {
		if s.park(k, o, parkPile, 0) {
			continue
		}
		if !s.parkPileQuiet(o) {
			s.capFull(StallReplCands)
			return
		}
	}
}

// parkPileQuiet queues a pile card with one exile-instead effect or none, whose
// turn comes in the queue all the same; false when the queue is full.
func (s *State) parkPileQuiet(o Obj) bool {
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkNone {
			*r = parkedMove{obj: o, kind: parkPile}
			return true
		}
	}
	return false
}
