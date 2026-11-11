package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An emblem's remembered objects (an Effect's RememberObjects$ over a set the
// emblem's trigger reads as IsRemembered). Four slots hold up to four objects,
// each object + 1 (0 = empty): slot 0 is the frozen number g.embN (a capturing
// emblem has no frozen number), slot 1 g.embR, slots 2 and 3 g.embX; an emblem
// past maxEmb keeps slots 1..3 in embRec.rem. The set a trigger matches is the
// object compare, not a version (gorge matches the object).
//
//	kop.Emblem.Capture     slot 0 = the first target slot's object
//	kop.Emblem.Capture2    slot 1 = the second target slot's object (T'Pring)
//	kop.Emblem.CaptureRem  slots 0..3 = the resolution's remembered set (Mysterio)

// StallEmbRem is a remembered set of more than four objects handed to an
// emblem (Mysterio, Master of Illusion's tokens past the fourth): the slots
// hold four, so the game stalls rather than forget the rest (a declared
// capacity limit).
const StallEmbRem = 161

func init() { stallNames[StallEmbRem] = "embrem" }

// embRemGet is slot j (0..3) of player p's emblem i: object + 1, 0 = none.
func (s *State) embRemGet(p uint8, i, j int) uint8 {
	if j == 0 {
		return s.embNum(p, i)
	}
	if j >= embRemSlots {
		return 0
	}
	if i >= maxEmb {
		return s.embRecAt(p, i).rem[j-1]
	}
	if j == 1 {
		return uint8(s.g.embR[p][i])
	}
	return s.g.embX[p][i][j-2]
}

// embRemSet sets slot j of player p's emblem i.
func (s *State) embRemSet(p uint8, i, j int, v uint8) {
	switch {
	case j >= embRemSlots:
	case j == 0:
		s.embSetNum(p, i, v)
	case i >= maxEmb:
		s.embRecAt(p, i).rem[j-1] = v
	case j == 1:
		s.g.embR[p][i] = Obj(v)
	default:
		s.g.embX[p][i][j-2] = v
	}
}

// embRemHolds reports whether player p's emblem i remembers object o.
func (s *State) embRemHolds(p uint8, i int, o Obj) bool {
	for j := 0; j < embRemSlots; j++ {
		if s.embRemGet(p, i, j) == uint8(o)+1 {
			return true
		}
	}
	return false
}

// embRemDrop forgets object o in player p's emblem i.
func (s *State) embRemDrop(p uint8, i int, o Obj) {
	for j := 0; j < embRemSlots; j++ {
		if s.embRemGet(p, i, j) == uint8(o)+1 {
			s.embRemSet(p, i, j, 0)
		}
	}
}

// embRemFill stores the objects the resolving OpCreateEmblem's emblem e
// remembers into r. It reports false when the resolution's set holds more
// than the emblem's slots (a declared capacity stall, StallEmbRem).
func (s *State) embRemFill(e *kop.Emblem, r *embRec) bool {
	if e.Capture2 != kop.RefNone {
		if o := s.objRefObj(e.Capture2); o != NoObj {
			r.rem[0] = uint8(o) + 1
		}
	}
	if !e.CaptureRem {
		return true
	}
	if int(s.g.nrem) > embRemSlots {
		return false
	}
	r.num = 0
	for j := 0; j < int(s.g.nrem); j++ {
		v := uint8(s.g.rem[j]) + 1
		if j == 0 {
			r.num = v
		} else {
			r.rem[j-1] = v
		}
	}
	return true
}

// hostNow reports whether e's source is on the battlefield now. gorge binds a
// permanent Effect to the source's current incarnation when it is registered
// (SourceBattlefield at registration), not to the one that put the trigger on
// the stack: a source blinked or reanimated while the trigger waited still has
// the registration end with it. A token cannot return, so its id on the
// battlefield with another version is a reused id, not the source.
func (s *State) hostNow(e *stackEntry) bool {
	if e.obj == NoObj || int(e.obj) >= MaxObj {
		return false
	}
	ob := &s.objs[e.obj]
	return ob.zone == kop.ZoneBattlefield && (ob.of&ofToken == 0 || ob.ver == e.srcVer)
}
