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
// staying where it is); false: the game stalled, or the play is parked on an
// ask of its entry (freeLandAsk) and the drain resumes once it is answered.
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
		return s.freeLandAsk(k, o, p)
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

// freeLandAsk plays queued land o whose entry may ask something, as the hand
// play does (legal.go's AkPlayLand arm: gorge beginPlay's land arm asks the
// as-enters choice before the move, newCast then continueCast): the choice is
// posed with the land still in its zone and its answer plays it
// (preLandAsk, applyGen); a shock land's election and the CR 616.1 entry
// order are the play's own asks (playLandNow). false when an ask is posed:
// gorge's runPlays parks at it, so the rest of the queue waits.
//
//go:noinline
func (s *State) freeLandAsk(k *prep, o Obj, p uint8) bool {
	if st := DebugVentureStats; st != nil {
		st.LandAsks++
	}
	if s.preLandAsk(k, o) {
		return false
	}
	s.playLandNow(k, o, p)
	return s.ctl == ctlPriority && s.g.nask == 0 && s.g.ro.n == 0
}

// preLandSeat is the seat whose land play a pre-ask belongs to: the caster of
// the Play whose queue is being drained (a land picked from another zone is
// played by the Play's controller), otherwise the player holding priority.
func (s *State) preLandSeat() uint8 {
	if s.g.freeP&freeLandsOn != 0 {
		return uint8(s.g.freeP & freeSeatMask)
	}
	return s.prio
}
