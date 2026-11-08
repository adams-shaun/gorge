package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CR 903.9a: a commander put into a graveyard or exile since the last
// state-based check may be put into the command zone by its owner. It is a
// state-based action with a choice, posed at the start of a pass (before the
// lose checks) and only while no other ask of the pass is outstanding. The
// owner's answer is DkChoose/ChCommanderZone: mode 0 moves it to the command
// zone, mode 1 leaves it where it is.

// cmdZoneDue reports whether seat p's commander sits in a graveyard or exile
// and has arrived there since the last ask (CR 903.9a's "put into that zone
// since the last time state-based actions were checked"). cmdAsk holds the
// object's ver at the last ask; a zone change bumps ver (CR 400.7), so a
// declined commander is not asked again until it moves.
func (s *State) cmdZoneDue(p uint8) bool {
	pl := &s.pl[p]
	o := pl.cmd
	if o == NoObj {
		return false
	}
	z := s.objs[o].zone
	return (z == kop.ZoneGraveyard || z == kop.ZoneExile) && s.objs[o].ver != pl.cmdAsk
}

// cmdZoneFirstDue returns the due seat whose commander arrived first, as
// gorge asks in arrival order. The per-object stamp is the low 8 bits of the
// touch count at its last write: among a burst of moves (well under 256
// object writes) the larger distance from the current count is the earlier
// arrival. A tie keeps seat order.
func (s *State) cmdZoneFirstDue() (uint8, bool) {
	d0, d1 := s.cmdZoneDue(0), s.cmdZoneDue(1)
	if !d0 && !d1 {
		return 0, false
	}
	if d0 && d1 {
		a := uint8(s.touchN) - s.stamp[s.pl[0].cmd]
		b := uint8(s.touchN) - s.stamp[s.pl[1].cmd]
		if b > a {
			return 1, true
		}
		return 0, true
	}
	if d0 {
		return 0, true
	}
	return 1, true
}

// cmdZoneAsk poses the pass-start CR 903.9a ask when it is owed and no other
// ask of the pass (a parked move or a queued as-enters ask) is outstanding.
// A legend ask already posed is ctlLegend, which the control-state switch
// below rejects; a legend duplicate merely owed this pass does not defer the
// commander ask (gorge's guard is legendBatch, parked only later in the
// pass). It reports whether it posed one; the sba loop then returns and Step
// builds the decision from the ctlCmdZone* state.
func (s *State) cmdZoneAsk() bool {
	if s.cfg&cfgCommander == 0 || s.over {
		return false
	}
	switch s.ctl {
	case ctlPriority, ctlStepBegin, ctlDivide:
		// the ordinary pass, and the combat damage pass (combat.go defers
		// its priority round and re-poses this ask after opening it)
	default:
		return false
	}
	if s.parkedWaiting() || s.g.nask != 0 {
		return false
	}
	p, ok := s.cmdZoneFirstDue()
	if !ok {
		return false
	}
	s.ctl = ctlCmdZone0 + p
	return true
}

// askCmdZone builds the owner's two-option decision and records the asked
// object's version, so a decline is not re-asked until the card moves again.
func (s *State) askCmdZone(_ *prep, d *Decision, p uint8) {
	pl := &s.pl[p]
	o := pl.cmd
	pl.cmdAsk = s.objs[o].ver
	d.Kind, d.Sub, d.Player = DkChoose, ChCommanderZone, p
	d.add(MakeAction(AkChoose, uint8(o), 0, 0)) // put it into the command zone
	d.add(MakeAction(AkChoose, uint8(o), 0, 1)) // leave it
}

// applyCmdZone answers the ask: mode 0 moves the commander from its graveyard
// or exile to the command zone (a zone change, so ver bumps and it leaves the
// list); mode 1 leaves it. The SBA loop then runs again.
func (s *State) applyCmdZone(k *prep, p, mode uint8) {
	s.ctl = ctlPriority
	if mode != 0 {
		return
	}
	pl := &s.pl[p]
	o := pl.cmd
	if o == NoObj {
		return
	}
	z := s.objs[o].zone
	if (z != kop.ZoneGraveyard && z != kop.ZoneExile) || s.objs[o].ver != pl.cmdAsk {
		return
	}
	from := s.detach(o)
	s.placeFromK(k, o, kop.ZoneCommand, false, from, false)
}
