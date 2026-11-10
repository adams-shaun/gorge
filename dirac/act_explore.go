package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// explore is OpExplore (gorge exploreOnce): the explorer's controller
// reveals their library's top card; a land goes to their hand, a nonland
// card poses the graveyard-or-top ask (false). Nothing when the explorer
// left the battlefield or the library is empty.
func (s *State) explore(k *prep, in *kop.Instr) bool {
	if s.objRefObj(in.Obj) == NoObj {
		return true // up to one target, and none chosen
	}
	p := s.lastKnownController(k, in.Obj)
	pl := &s.pl[p]
	if pl.libLo == pl.libHi {
		if k.gen != nil && k.gen.anyExplEvTrig {
			s.fireExplores(k, in, p, NoObj) // CR 701.44b: it explored all the same
		}
		return true
	}
	top := pl.lib[pl.libLo]
	if k.cards[s.objs[top].card].land {
		pl.libLo++
		s.touch(top)
		s.objs[top].zone = kop.ZoneNone
		s.placeFrom(top, kop.ZoneHand, false, kop.ZoneLibrary, false)
		if k.gen != nil && k.gen.anyExplEvTrig {
			s.fireExplores(k, in, p, top) // trig_exploreevidence.go
		}
		return true
	}
	s.res.ask, s.res.askP = raGenExplore, p
	return false
}

// exploreNonland applies a nonland explore's answer (gorge
// applyNonlandExplore): a +1/+1 counter on the explorer first, then the
// revealed card into its owner's graveyard when grave.
func (s *State) exploreNonland(k *prep, in *kop.Instr, grave bool) {
	p := s.res.askP
	pl := &s.pl[p]
	top := pl.lib[pl.libLo]
	if sl := s.objRef(k, in.Obj); sl != NoSlot {
		n := s.placeCounters(k, sl, int(kop.CtrP1P1), 1, s.perms[sl].ctrl, ctrPutEffect)
		if n > 0 && k.anyCtrAdded {
			s.ctrNote(k, sl, int(kop.CtrP1P1))
		}
		if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
			s.g.lastCK = kop.CtrP1P1 // CounterAddedOnce reads the kind (a stale LORE fired a Saga's chapter)
			s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(n))
		}
	}
	if grave && pl.libLo < pl.libHi && pl.lib[pl.libLo] == top && k.gen != nil && s.park(k, top, parkMill, 0) {
		// two exile-instead effects compete for the revealed card's move
		// (two Rest in Peaces): gorge asks their CR 616.1 order, and the card
		// waits on top of the library for the answer, as a milled card does
		grave = false
	}
	if grave && pl.libLo < pl.libHi && pl.lib[pl.libLo] == top {
		pl.libLo++
		s.touch(top)
		s.objs[top].zone = kop.ZoneNone
		s.placeFrom(top, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
	}
	if k.gen != nil && k.gen.anyExplEvTrig {
		s.fireExplores(k, in, p, top) // trig_exploreevidence.go
	}
}
