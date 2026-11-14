package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// maxMayCold is how many play-from-zone permissions past the first maxMay a
// table that grants them over a remembered set keeps in the cold buffer (an
// Expedited Inheritance chain peaks at 40 records).
const maxMayCold = 32

// coldMay is the records past maxMay, for a table with a remembered-set grant.
var coldMay = newCold[mayPlay]("may", func(g *genPrep) int {
	if g.anyMayCold {
		return maxMayCold
	}
	return 0
})

// tableMayPlayRemembered reports whether some program of t grants a play
// permission to every card of a remembered, chosen or imprinted set
// (genPrep.anyMayCold).
func tableMayPlayRemembered(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpMayPlay && (in.Obj == kop.RefRemembered || in.Obj == kop.RefChosen || in.Obj == kop.RefImprinted) {
					return true
				}
			}
		}
	}
	return false
}

// mayCap is how many permission records s can hold: maxMay in the State and,
// for a table that sized it, the cold buffer's records after them.
func (s *State) mayCap() int {
	return maxMay + coldMay.count(s.cold.layout)
}

// mayFull reports whether no record can be added.
func (s *State) mayFull() bool { return int(s.g.nmay) >= s.mayCap() }

// mayAt is record i: the first maxMay live in genState, the rest in the cold
// buffer.
func (s *State) mayAt(i int) *mayPlay {
	if i < maxMay {
		return &s.g.may[i]
	}
	return &s.mayCold()[i-maxMay]
}

func (s *State) mayCold() []mayPlay { return coldMay.of(s) }

// mayShiftDown moves every record after i one place down, over i; the caller
// drops nmay.
func (s *State) mayShiftDown(i int) {
	for j := i; j < int(s.g.nmay)-1; j++ {
		*s.mayAt(j) = *s.mayAt(j + 1)
	}
}

// mayColdLive is the records past maxMay, in order.
func (s *State) mayColdLive() []mayPlay {
	if int(s.g.nmay) <= maxMay {
		return nil
	}
	return s.mayCold()[:int(s.g.nmay)-maxMay]
}
