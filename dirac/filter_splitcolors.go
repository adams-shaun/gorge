package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// splitCols is the colour set of object o showing card c to a card filter.
// Off the stack and the battlefield a split card (a Room too) has the
// characteristics of both halves (CR 709.4), so its colours are the halves'
// union (gorge effects.ColorMaskOf over state.Object.SplitHalves): Reduce //
// Rubble in a hand is blue and red, and Addle naming red may take it. On the
// stack (the half cast) and the battlefield (a Room's unlocked doors) the
// card's own colours stand. The mana-value twin is splitMV (face_castface.go).
func (s *State) splitCols(k *prep, c CardID, o Obj, cols kop.ColorMask) kop.ColorMask {
	cd := &k.t.Cards[c]
	if k.gen == nil || !k.gen.anySplit || o == NoObj || cd.FaceMode != kop.FaceSplit && cd.FaceMode != kop.FaceRoom {
		return cols
	}
	if cd.FaceMode == kop.FaceSplit && (cd.Face != 0 || len(cd.Faces) != 2) || cd.FaceMode == kop.FaceRoom && len(cd.Faces) != 8 {
		return cols
	}
	if z := s.objs[o].zone; z == kop.ZoneStack || z == kop.ZoneBattlefield {
		return cols
	}
	return k.t.Cards[cd.Faces[0]].Colors | k.t.Cards[cd.Faces[1]].Colors
}
