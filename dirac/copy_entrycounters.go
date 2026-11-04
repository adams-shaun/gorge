package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// copyEnteredCounters is OpBecomeCopy's BecomeCopyCtr rider (gorge's
// RememberCloneOrigin$ True + PutCounter EachFromSource$ Remembered:
// Dominion Saboteur). It puts on the entering permanent at sl one counter of
// each kind the chosen copy origin at src holds, read once before any is
// placed (CR 707.9b/614.12: the copy enters with them, so its enters triggers
// and the state-based actions see them). A counter kind above the inline
// slots comes from the overflow table. CtrLevel is a Class level, not a
// counter (the same exclusion OpPutCountersFrom keeps). The placements go
// through placeCounters, so the AddCounter replacements apply exactly as
// they do to the rider's PutCounter in gorge.
func (s *State) copyEnteredCounters(k *prep, sl, src Slot, ctrl uint8) {
	p := &s.perms[src]
	p1 := int(p.p1)
	ck, cn := p.ck, p.cn
	var over [maxCtrOver]ctrOverEntry
	nover := 0
	if p.ck[ctrSlots-1] != 0 {
		nover = copy(over[:], s.ctrOverOf(p.obj, 0))
	}
	if p1 > 0 {
		s.placeCounters(k, sl, int(kop.CtrP1P1), p1, ctrl, ctrPutEffect)
	}
	for i := range ck {
		if cn[i] > 0 && kop.CounterKind(ck[i]) != kop.CtrLevel {
			s.placeCounters(k, sl, int(ck[i]), int(cn[i]), ctrl, ctrPutEffect)
		}
	}
	for i := 0; i < nover; i++ {
		if over[i].n > 0 && kop.CounterKind(over[i].kind) != kop.CtrLevel {
			s.placeCounters(k, sl, int(over[i].kind), int(over[i].n), ctrl, ctrPutEffect)
		}
	}
}
