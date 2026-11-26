package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// graveFlashLost reports whether graveyard card o, cast through another
// source's permission (a may-play), has no flash: its instant-speed rests on
// the printed Flash keyword alone and the card lost all its abilities
// (Yixlid Jailer; CR 613.1f). gorge derives the keyword (legal_walk_alt.go
// hasKeywordH(kwhFlash)), which the removal strips; an instant's own type
// stays, and a flash another static grants (castFlash) is read apart.
func (s *State) graveFlashLost(k *prep, o Obj, pc *pcard) bool {
	if !graveLossOn(k) || s.objs[o].zone != kop.ZoneGraveyard || pc.types&kop.TInstant != 0 || pc.kw&kop.KwFlash == 0 {
		return false
	}
	return s.gravePrintedHidden(k, o)
}
