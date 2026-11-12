package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The unconditional DestinationAlternative$ asks (kgen lower_destalt.go;
// gorge engine/effects zone_positionchoice.go and zone_search.go).

// raGenDestAlt is the hand-or-graveyard ask of a library search with
// kop.SrDestAltGrave (ChGenDestAlt). It sits past the ask families that
// offset from raGenManifestDread (raGenTokCopy is +79).
const raGenDestAlt = raGenManifestDread + 80

// ChGenDestAlt is gorge's changezone_dest_alt KChoose: mode 0 = the
// search's destination (gorge's option 0, "primary"), 1 = the graveyard
// ("alternate"); the options name the searcher, no object.
const ChGenDestAlt ChoiceKind = 213

// destAltPark poses the ask once the pick is final and a card was found
// (gorge askUnconditionalDestination after the pick): the found cards still
// sit in the library.
func (s *State) destAltPark(k *prep) bool {
	in := &s.prog(k)[s.res.pc]
	if in.Op != kop.OpSearchEx || in.B.N&kop.SrDestAltGrave == 0 || in.B.N&kop.SrAltOrigin != 0 || s.g.nlook == 0 {
		return false
	}
	s.res.ask, s.res.askP = raGenDestAlt, s.res.e.ctrl
	return true
}

func (s *State) destAltOptions(d *Decision) {
	d.Sub = ChGenDestAlt
	d.add(MakeAction(AkChoose, uint8(NoObj), 0, 0))
	d.add(MakeAction(AkChoose, uint8(NoObj), 0, 1))
}

// destAltAnswer moves the found cards to the answered zone and finishes the
// search (its shuffle and the rest of the resolution).
func (s *State) destAltAnswer(k *prep, a Action) {
	s.res.ask = raNone
	if a.Mode() == 1 {
		s.g.pickZ = uint8(kop.ZoneGraveyard)
	}
	s.finishSearchPicked(k)
}

// handPosPark poses the controller's top-or-bottom ask of a hand put-back
// with kop.SrHandPosAsk once the n picked cards are on top of the library
// (gorge handLibraryTail asks after the move, only when a card moved). The
// count waits in pickLeft, which the finished pick no longer reads.
func (s *State) handPosPark(k *prep, n uint8) {
	prog := s.prog(k)
	if n == 0 || int(s.res.pc) >= len(prog) || s.res.ask != raNone {
		return
	}
	in := &prog[s.res.pc]
	if in.Op != kop.OpChooseMove || in.B.N&kop.SrHandPosAsk == 0 || kop.Zone(s.g.pickZ) != kop.ZoneLibrary {
		return
	}
	s.g.pickLeft = n
	s.res.ask, s.res.askP = raGenTopBottom, s.res.e.ctrl
}

// handPosAnswer is the put-back's answer: bottom moves the set from the top
// to the bottom keeping its order, the first picked above the second (gorge
// libraryOrderPlacement bottom=true).
func (s *State) handPosAnswer(bottom bool) {
	if bottom {
		pl := &s.pl[s.g.pickP]
		for i := 0; i < int(s.g.pickLeft) && pl.libHi > pl.libLo; i++ {
			o := pl.lib[pl.libLo]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.g.pickLeft = 0
}
