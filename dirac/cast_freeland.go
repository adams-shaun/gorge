package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Land plays of a cast-free Play (OpCastFree freeLands; gorge
// effects/play.go with rules beginPlay's land arm): a Play with no ValidSA$
// over remembered cards that may be lands (Dungeon of the Mad Mage's Runestone
// Caverns: "exile the top two cards of your library, you may play them").

// DebugVentureNoLand (TestVentureRunestoneLandMutation): a land a Play picked
// is never played.
var DebugVentureNoLand bool

// freeLandable is gorge effectLandPlayRefusal's turn and count gates as
// CastProhibited reads them when the offer is posed: p is the active player
// and has a land play left. (A CantPlayLand static is not modelled.)
func (s *State) freeLandable(k *prep, p uint8) bool {
	return p == s.active && s.pl[p].landsPlayed < s.landLimit(k, p)
}

// freeLandPlay plays queued land o for p (gorge beginPlay: the gates are read
// again when the play is begun, a refused one is a Note and nothing, the card
// staying where it is); false: the game stalled. A land whose entry asks (an
// as-enters choice, a shock land's election, the CR 616.1 entry order) is a
// stall: the queue parks no land play.
//
//go:noinline
func (s *State) freeLandPlay(k *prep, o Obj, p uint8) bool {
	if DebugVentureNoLand {
		return true
	}
	if s.objs[o].zone != kop.Zone((s.g.freeP>>freeZoneShift)&7) || !s.freeLandable(k, p) {
		return true
	}
	c := s.objs[o].card
	if k.gen.anyEntUntap || k.gen.has(c, gfShockEntry) || s.enterChoiceKind(k, c) != 0 {
		s.capFull(StallLowering)
		return false
	}
	from := s.detach(o)
	if k.gen.anyMayLimit {
		s.countMayPlayLand(k, o, from)
	}
	s.pl[p].landsPlayed++
	if k.gen.anyLandPlayed {
		s.enterLandPlayed(k, o, p, from)
	} else {
		s.enter(k, o, p, false, from)
	}
	s.passes = 0
	return true
}
