package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// splitCardColors is the colours of the card object o read for itself: its
// card's, a split card off the stack and the battlefield showing both halves'
// (CR 709.4b; splitHalvesChars). A shared-colour comparison reads the other
// object through it.
func (s *State) splitCardColors(k *prep, o Obj) kop.ColorMask {
	c := s.objs[o].card
	_, cols := s.splitHalvesChars(k, c, o, false, false, 0, k.t.Cards[c].Colors)
	return cols
}

// splitHalvesChars is the card types and colours card c shows to a filter
// when c is a split card's front and o lies off the stack and the battlefield:
// both halves' combined (CR 709.4, 709.4b/c; gorge state.Object.SplitHalves
// read by effects.ColorMaskOf and hasTypeCtx), so Leave // Chance in a hand is
// a white and a red instant and sorcery card. Anything else, a face-down card
// included, keeps ty and cols; so does a read as the spell cast (spell: the
// cast half alone, CR 601.2f, 709.3a). Cold: behind genPrep.anySplit.
//
//go:noinline
func (s *State) splitHalvesChars(k *prep, c CardID, o Obj, fd, spell bool, ty kop.TypeMask, cols kop.ColorMask) (kop.TypeMask, kop.ColorMask) {
	if fd || spell || k.gen == nil || !k.gen.anySplit {
		return ty, cols
	}
	cd := &k.t.Cards[c]
	if cd.FaceMode != kop.FaceSplit || cd.Face != 0 || o == NoObj || len(cd.Faces) != 2 {
		return ty, cols
	}
	if z := s.objs[o].zone; z == kop.ZoneStack || z == kop.ZoneBattlefield {
		return ty, cols
	}
	h := cd.Faces[1]
	return ty | k.cards[h].types, cols | k.t.Cards[h].Colors
}
