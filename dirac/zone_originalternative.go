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
// clears it for an alt-origin search, scrResEnter at each resolution):
// bit 0 the library was elected, bit 1 the alternative zone, bit 2 the
// election is over.
const (
	altLib     uint32 = 1
	altOther   uint32 = 2
	altElected uint32 = 4
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

// altZoneOf is the alternative zone the instruction names.
func altZoneOf(in *kop.Instr) kop.Zone {
	return kop.Zone(in.B.N>>kop.SrAltZoneShift) & 7
}

// altSearchZone is searchZone of an alt-origin search once elected: the
// library when it was among the chosen zones, else the alternative zone.
func (s *State) altSearchZone(in *kop.Instr) kop.Zone {
	if s.g.lookSel&altLib != 0 {
		return kop.ZoneLibrary
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
	s.res.ask, s.res.askP = raGenKczZones, s.chooserOf(k, s.g.pickP)
	return true
}

// altOriginCands is searchCands of an alt-origin search: the elected zones'
// matching cards, the library first in library order, then the alternative
// zone in its order (gorge effSearchLibrary's candidate walk).
//
//go:noinline
func (s *State) altOriginCands(k *prep, in *kop.Instr, d *Decision) {
	p := s.g.pickP
	pl := &s.pl[p]
	for zi := 0; zi < 2; zi++ {
		var list []Obj
		switch {
		case zi == 0 && s.g.lookSel&altLib != 0:
			list = pl.lib[pl.libLo:pl.libHi]
		case zi == 1 && s.g.lookSel&altOther != 0:
			if altZoneOf(in) == kop.ZoneHand {
				list = pl.hand[:pl.handN]
			} else {
				list = pl.grave[:pl.graveN]
			}
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
		for zi := uint8(0); zi < 2; zi++ {
			if s.g.lookSel&(1<<zi) == 0 {
				d.add(MakeAction(AkChoose, 0, 0, zi))
			}
		}
		if s.g.lookSel&(altLib|altOther) != 0 {
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
		if a.Kind() == AkChoose {
			s.g.lookSel |= 1 << (a.Mode() & 1)
			if s.g.lookSel&(altLib|altOther) != altLib|altOther {
				return true // the other zone may still be added, or Done
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
