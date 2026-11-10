package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CR 731 day/night and CR 702.145 daybound/nightbound (gorge
// rules/daynight.go, cr-daybound-nightbound 2e8eb2156 and -transform
// 0c28afe60). The designation and the last turn's active player's spell
// count share genState.dayNight; every hook is behind prep.anyDayNight.
//   - beginTurn: the outgoing active player's spells this turn, capped at
//     2, are kept (gorge foldTurnChange's PrevTurnSpells).
//   - the untap step, before the untap action (CR 731.2, 502.2): day with
//     no spells becomes night, night with two or more becomes day.
//   - the SBA pass, after the others (gorge checkStateBased's last check):
//     with neither, a daybound permanent makes it day, else a nightbound
//     one night (CR 702.145d/g); with a designation the faces are re-synced.
//   - a designation change turns every face-up transforming permanent
//     whose face disagrees (CR 702.145c/f), player 0's battlefield list
//     first: a transform (FlipFace), so "transforms" triggers see it.
//   - a daybound front face entering at night enters back face up, no
//     transform (CR 702.145b; gorge entersTransformedAtNight).
//   - a daybound/nightbound permanent can't be transformed otherwise
//     (CR 702.145b/e; gorge effSetState's dayNightBound skip).
// Keywords are read off the face (no corpus line grants them) unless its
// abilities are lost (gorge reads the derived keyword).

const (
	dnDay   uint8 = 1
	dnNight uint8 = 2
)

// DayNight is the CR 731 designation: 0 neither, dnDay 1, dnNight 2.
func (s *State) DayNight() uint8 { return s.g.dayNight & 3 }

// DebugNoNight (TestDaynightLockstepMutation): day never becomes night.
var DebugNoNight bool

// dnTurn keeps the outgoing active player's spell count for CR 731.2.
func (s *State) dnTurn() {
	n := min(s.g.hist[SeatIx(s.active)][histSpells], 2)
	s.g.dayNight = s.g.dayNight&3 | n<<2
}

// dnUntap is CR 731.2's untap-step check.
func (s *State) dnUntap(k *prep) {
	n := s.g.dayNight >> 2
	switch s.g.dayNight & 3 {
	case dnDay:
		if n == 0 && !DebugNoNight {
			s.dnSet(k, dnNight)
		}
	case dnNight:
		if n >= 2 {
			s.dnSet(k, dnDay)
		}
	}
}

// dnSet changes the designation and turns the faces.
func (s *State) dnSet(k *prep, d uint8) {
	if s.g.dayNight&3 == d {
		return
	}
	s.g.dayNight = s.g.dayNight&^3 | d
	s.dnSync(k)
}

// dnKw is the daybound / nightbound keyword of the permanent at sl (its
// own face, abilities not lost).
func (s *State) dnKw(k *prep, sl Slot) (day, night bool) {
	cd := &k.t.Cards[s.objs[s.perms[sl].obj].card]
	if !cd.Daybound && !cd.Nightbound || k.gen != nil && s.genLost(k, sl) {
		return false, false
	}
	return cd.Daybound, cd.Nightbound
}

// dnBound: o is a daybound or nightbound permanent.
func (s *State) dnBound(k *prep, o Obj) bool {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return false
	}
	d, n := s.dnKw(k, ob.slot)
	return d || n
}

// dnSync turns every face-up transforming permanent whose face the
// designation contradicts; true when one turned.
func (s *State) dnSync(k *prep) bool {
	dn := s.g.dayNight & 3
	if dn == 0 {
		return false
	}
	changed := false
	for p := uint8(0); p < MaxSeats; p++ {
		var ord [MaxPerm]Slot
		n := s.battlefieldOrder(p, s.ctrlM[p], ord[:], 0)
		for _, sl := range ord[:n] {
			if !s.occ.has(sl) || s.faceDown(k, sl) {
				continue
			}
			o := s.perms[sl].obj
			cd := &k.t.Cards[s.objs[o].card]
			if cd.FaceMode != kop.FaceTransform || len(cd.Faces) < 2 {
				continue
			}
			day, night := s.dnKw(k, sl)
			if dn == dnNight && cd.Face == 0 && day || dn == dnDay && cd.Face != 0 && night {
				s.transformObj(k, o)
				changed = true
			}
		}
	}
	return changed
}

// dnSBA is CR 702.145c/d/f/g's state check; true when it changed anything.
func (s *State) dnSBA(k *prep) bool {
	if s.g.dayNight&3 != 0 {
		return s.dnSync(k)
	}
	day, night := false, false
	for m := s.occ; m.any(); m = m.rest() {
		d, n := s.dnKw(k, m.first())
		day, night = day || d, night || n
	}
	switch {
	case day:
		s.dnSet(k, dnDay)
	case night:
		s.dnSet(k, dnNight)
	default:
		return false
	}
	return true
}

// dnEnter turns a daybound front face entering at night to its back.
func (s *State) dnEnter(k *prep, o Obj) {
	cd := &k.t.Cards[s.objs[o].card]
	if cd.Daybound && cd.FaceMode == kop.FaceTransform && cd.Face == 0 && len(cd.Faces) >= 2 {
		s.turnFace(k, o, 1)
	}
}
