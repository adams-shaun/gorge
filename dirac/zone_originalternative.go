package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ChangeZone's OriginAlternative$ (a library
// search that may also look in the graveyard or the hand) and AttachedTo$ (a
// card returned from a graveyard enters attached). Cold and out of line;
// each hot site tests one flag bit and calls in here.

// DebugCzAttachOff is the mutation switch: the zone election is skipped (the
// library alone is searched) and a returned card enters unattached.
// TestCzattachMutation requires the lockstep to diverge.
var DebugCzAttachOff bool

// raGenKczZones is a OriginAlternative$ search's zone election (gorge
// originAlternativeZones: one KChoose "Choose the zones to search", Min 1,
// Max 2, an option per zone, library first); ChGenKczZone its pick, mode 0
// the library and 1 the alternative zone (the option's Index). Picks are
// made one zone at a time and AkDone closes them once one is made.
const (
	raGenKczZones = raGenManifestDread + 71
)

// ChGenKczZone is the election's ChoiceKind: mode = the zone picked.
const ChGenKczZone ChoiceKind = 251

// The election lives in genState.lookSel (dead on a search op; startSearch
// clears it for an alt-origin search, scrResEnter at each resolution): bit 0
// the library was elected, bit 1 the first alternative zone, bit 2 the
// second (the Graveyard,Hand pair's hand), bit 3 the election is over.
const (
	altLib     uint32 = 1
	altOther   uint32 = 2
	altOther2  uint32 = 4
	altElected uint32 = 8
)

// altOp reports an OpSearchEx with SrAltOrigin (bit 4 of an OpChooseMove's B
// is its entry counter kind, so the op is part of the test).
func altOp(in *kop.Instr) bool {
	return in.Op == kop.OpSearchEx && in.B.N&kop.SrAltOrigin != 0
}

// altOriginOp is altOp of the search op in progress.
func (s *State) altOriginOp(k *prep) bool { return altOp(&s.prog(k)[s.res.pc]) }

// altBegin starts an alt-origin search afresh (startSearch).
//
//go:noinline
func (s *State) altBegin() { s.g.lookSel = 0 }

// altZoneOf is the alternative zone the instruction names. The pair's
// SrAltGraveHand code reads as kop.ZoneLibrary, which no single alternative
// can name.
func altZoneOf(in *kop.Instr) kop.Zone {
	return kop.Zone(in.B.N>>kop.SrAltZoneShift) & 7
}

// altPair reports the two-zone alternative (Graveyard then Hand).
func altPair(in *kop.Instr) bool { return altZoneOf(in) == kop.ZoneLibrary }

// altZoneCount is how many options the zone election offers: the library and
// one alternative, or the library, graveyard and hand for the pair.
func altZoneCount(in *kop.Instr) uint8 {
	if altPair(in) {
		return 3
	}
	return 2
}

// altZoneLists is the search's possible pool per election bit, in gorge's
// origin order: library, first alternative, second alternative (nil for a
// single alternative). The caller masks by the elected bits.
func (s *State) altZoneLists(in *kop.Instr) [3][]Obj {
	pl := &s.pl[s.g.pickP]
	lists := [3][]Obj{pl.lib[pl.libLo:pl.libHi]}
	switch altZoneOf(in) {
	case kop.ZoneLibrary: // the pair
		lists[1] = pl.grave[:pl.graveN]
		lists[2] = pl.hand[:pl.handN]
	case kop.ZoneHand:
		lists[1] = pl.hand[:pl.handN]
	default:
		lists[1] = pl.grave[:pl.graveN]
	}
	return lists
}

// altSearchZone is searchZone of an alt-origin search once elected: the
// library when it was among the chosen zones; otherwise the one elected
// alternative, or ZoneNone when the pair's two alternatives were both
// elected (the pool spans zones, so no single-zone consumer may run).
func (s *State) altSearchZone(in *kop.Instr) kop.Zone {
	if s.g.lookSel&altLib != 0 {
		return kop.ZoneLibrary
	}
	if altPair(in) {
		switch {
		case s.g.lookSel&(altOther|altOther2) == altOther:
			return kop.ZoneGraveyard
		case s.g.lookSel&(altOther|altOther2) == altOther2:
			return kop.ZoneHand
		default:
			return kop.ZoneNone
		}
	}
	return altZoneOf(in)
}

// altPending poses the zone election the first time searchPick runs for an
// alt-origin search; true: the election is asked (or, under the mutation,
// settled at once as the library alone, which poses nothing).
//
//go:noinline
func (s *State) altPending(k *prep) bool {
	in := &s.prog(k)[s.res.pc]
	if !altOp(in) || s.g.lookSel&altElected != 0 {
		return false
	}
	if DebugCzAttachOff {
		s.g.lookSel = altLib | altElected
		return false
	}
	s.g.lookSel = 0
	s.res.ask, s.res.askP = raGenKczZones, s.chooserOf(s.g.pickP)
	return true
}

// altOriginCands is searchCands of an alt-origin search: the elected zones'
// matching cards, the library first in library order, then the alternative
// zone in its order (gorge effSearchLibrary's candidate walk).
//
//go:noinline
func (s *State) altOriginCands(k *prep, in *kop.Instr, d *Decision) {
	p := s.g.pickP
	lists := s.altZoneLists(in)
	dn := s.g.pickF&kop.SrDifferentNames != 0
	for zi := uint8(0); zi < altZoneCount(in); zi++ {
		if s.g.lookSel&(1<<zi) == 0 {
			continue
		}
		list := lists[zi]
		if dn {
			s.differentNameCands(k, in, d, list) // the one name rule, shared with a plain library search
			continue
		}
		for _, o := range list {
			if s.picksHas(o) {
				continue
			}
			if s.matchCardGen(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o) {
				d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			}
		}
	}
}

// askOptionsCzAttach builds this file's resolution asks' options; false:
// not one of them.
//
//go:noinline
func (s *State) askOptionsCzAttach(k *prep, d *Decision) bool {
	switch s.res.ask {
	case raGenKczZones:
		d.Sub = ChGenKczZone
		in := &s.prog(k)[s.res.pc]
		n := altZoneCount(in)
		all := (uint32(1) << n) - 1
		for zi := uint8(0); zi < n; zi++ {
			if s.g.lookSel&(1<<zi) == 0 {
				d.add(MakeAction(AkChoose, 0, 0, zi))
			}
		}
		if s.g.lookSel&all != 0 {
			d.add(MakeAction(AkDone, 0, 0, 0))
		}
	default:
		return false
	}
	return true
}

// applyAskCzAttach applies an answer to this file's resolution asks; false:
// not one of them.
//
//go:noinline
func (s *State) applyAskCzAttach(k *prep, a Action) bool {
	switch s.res.ask {
	case raGenKczZones:
		in := &s.prog(k)[s.res.pc]
		n := altZoneCount(in)
		all := (uint32(1) << n) - 1
		if a.Kind() == AkChoose {
			if m := a.Mode(); m >= n {
				return true
			} else {
				s.g.lookSel |= 1 << m
			}
			if s.g.lookSel&all != all {
				return true // another zone may still be added, or Done
			}
		}
		s.res.ask = raNone
		s.g.lookSel |= altElected
		if s.searchPick(k) && !s.moveAsked() {
			s.continueRes(k, 0)
		}
	default:
		return false
	}
	return true
}

// czAttachHost is OpReturnEx's RetAttachTo for a battlefield entry: the slot
// the returned card o enters attached to, and stay when it enters nowhere.
// An Aura enters attached to its named bearer when it may legally enchant
// it, else it stays in its zone (CR 303.4g; gorge markChangeZoneAttach and
// the entry gate, which keep the named bearers among the Aura's own
// candidates). Anything else (an Equipment) enters, then attaches to the
// bearer when that is a creature on the battlefield (gorge
// changeZoneAttachedTo, whose attach the CR 704.5n/704.5q state-based
// actions would undo for any other bearer).
//
//go:noinline
func (s *State) czAttachHost(k *prep, in *kop.Instr, o Obj, ctrl uint8) (host Slot, stay bool) {
	if DebugCzAttachOff {
		return NoSlot, false
	}
	if in.F != 0 {
		return s.czBearerHost(k, in, o) // a filter bearer: the controller chooses (tgt_attachedto.go)
	}
	b := s.objRefObj(kop.Ref(in.B.N >> kop.RetAttachShift & 7))
	c := s.objs[o].card
	if k.cards[c].types&kop.TAura != 0 {
		if b == NoObj || k.t.Cards[c].Enchant.Kind != kop.TgtObject {
			return NoSlot, true
		}
		var buf [MaxOptions]uint8
		for i, n := 0, s.enchantCands(k, c, ctrl, &buf); i < n; i++ {
			if Obj(buf[i]) == b {
				return s.objs[b].slot, false
			}
		}
		return NoSlot, true
	}
	if b == NoObj || s.objs[b].zone != kop.ZoneBattlefield {
		return NoSlot, false
	}
	sl := s.objs[b].slot
	if s.typesGen(k, sl)&kop.TCreature == 0 {
		return NoSlot, false
	}
	return sl, false
}
