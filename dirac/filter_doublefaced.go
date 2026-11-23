package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwDoubleFaced: FwDoubleFaced, gorge wordDoubleFaced
// (engine/effects/filter_word.go:949): o.Card.BackFaceIsBattlefieldOnly
// (engine/cards/layout.go:17) -- a two-faced card of either transforming kind
// (CR 712, CR 712.4d). A printed read of the card's layout: the face it shows
// now (face down, transformed, the back face) changes nothing, as in gorge.
//
//kmanifest:fword FwDoubleFaced
func (s *State) fwDoubleFaced(c *fctx, l *kop.FLit) bool {
	switch {
	case c.perm():
		return doubleFacedDef(&c.k.t.Cards[s.cardAt(c.sl)])
	case c.card():
		return doubleFacedDef(&c.k.t.Cards[c.c])
	}
	return false
}

// doubleFacedDef is kop.CardDef's record of BackFaceIsBattlefieldOnly.
func doubleFacedDef(cd *kop.CardDef) bool {
	return len(cd.Faces) == 2 && (cd.FaceMode == kop.FaceTransform || cd.FaceMode == kop.FaceModal)
}
