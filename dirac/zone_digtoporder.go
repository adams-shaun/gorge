package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The rest of a dig put back on top in any order (kop.DigRestTopAsk; gorge
// effDig's digTopOrderAsk, CR 701.29a): once the taken cards have moved,
// the two or more cards left sit on top of the library in window order, and
// the resolving controller -- gorge's chooser, whoever's library it is
// (Spin into Myth fateseals an opponent) -- orders them with the
// raGenDigArrange picks, the first pick on top. gorge applies the answered
// KArrange as one LibraryOrder (pile A, then the untouched remainder:
// rules arrangeAnswerRecord's "top"); each kernel pick puts its card at
// the top of the cards still unpicked, which reaches the same order.

// digArrangeTop reports whether the pending dig arrange is the on-top one.
func (s *State) digArrangeTop(k *prep) bool {
	if s.res.ask != raGenDigArrange {
		return false
	}
	return (s.prog(k)[s.res.pc].C>>3)&kop.DigRestTopAsk == kop.DigRestTopAsk
}

// digArrangeAsker is who orders the dig's rest: the controller for the
// on-top order (gorge digTopOrderAsk's chooser), the library's player for
// the bottom order.
func (s *State) digArrangeAsker(k *prep) uint8 {
	if (s.prog(k)[s.res.pc].C>>3)&kop.DigRestTopAsk == kop.DigRestTopAsk {
		return s.res.e.ctrl
	}
	return s.g.pickP
}

// digArrangeTopPick moves picked rest card o to just above the topmost
// rest card not yet picked: the picks so far stay on top in pick order.
func (s *State) digArrangeTopPick(o Obj) {
	pl := &s.pl[s.g.pickP]
	at := uint8(255)
	for i := 0; i < int(s.g.nlook); i++ {
		c := s.resArm().look[i]
		if s.g.lookSel&(1<<i) != 0 || s.objs[c].zone != kop.ZoneLibrary {
			continue
		}
		if j := pl.libIndex(c); j < at {
			at = j
		}
	}
	if at == 255 || pl.libIndex(o) == at {
		return
	}
	s.touch(o)
	pl.libRemove(o)
	pl.libInsert(o, at)
}

// FeatArrangeDigTop reports whether the ChArrange decision is a dig's rest
// ordered back on top (gorge's "top" options).
func (s *State) FeatArrangeDigTop() bool {
	return s.ctl == ctlResolve && s.digArrangeTop(s.k())
}
