package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// storeCensusGap reports whether p holds an activatable storage battery
// (Icatian Store's "{T}, Remove any number of storage counters: Add {W} for
// each"), an idle one holding no counters included: gorge's window census
// admits its ability (X is announced, so any count pays the cost) but prices no
// SubCounter<X> cost shape (castWindowSourceExcluded's paidCostShape), so the
// census is incomplete and a pool-short target menu keeps its candidates
// (affordableTargetCandidates' fallback, CR 601.2c, 601.2h). A battery the
// build committed (withheld from s.occ) is no census source.
func (s *State) storeCensusGap(k *prep, p uint8) bool {
	if !k.ctrMana {
		return false
	}
	for b := s.scriptCtrIdle(k, p, true).and(s.occ); b.any(); b = b.rest() {
		if kop.StoreManaAb(k.ctrAbOf(s.cardAt(b.first()))) {
			return true
		}
	}
	return false
}
