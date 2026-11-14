package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Multi-face cards, cold: reached only when
// the table has a card with faces (genPrep.anyFaces). An object's CardID
// is its current face's def (kop.CardDef.Faces), so every characteristic
// read sees the face without a face index in State; turning a face up
// rewrites the object's card and gorge's FaceIdx resets turn it back to
// the front (Faces[0]).

// offerFaceLand offers playing hand card o as its land back face: gorge's
// "modal_land" (rules legal.go modalLandBack: a Modal DFC in hand whose
// back face is a land, after its front's own land play, CR 712.8),
// AkPlayLand with mode = the face index. The caller checked the land play
// is allowed now.
func (s *State) offerFaceLand(k *prep, d *Decision, o Obj) {
	cd := &k.t.Cards[s.objs[o].card]
	if cd.FaceMode != kop.FaceModal || cd.Face != 0 || len(cd.Faces) != 2 {
		return
	}
	if k.cards[cd.Faces[1]].land {
		d.add(MakeAction(AkPlayLand, TCode(o), 0, 1))
	}
}

// turnFace turns object o's face i up (gorge's FlipFace before a
// modal_land play).
func (s *State) turnFace(k *prep, o Obj, i uint8) {
	ob := &s.objs[o]
	cd := &k.t.Cards[ob.card]
	if int(i) >= len(cd.Faces) {
		panic("kernel: no such face")
	}
	s.touch(o)
	ob.card = cd.Faces[i]
}

// faceReset turns a permanent that left the battlefield front face up
// again (gorge events apply_zone.go: a Modal DFC leaving the battlefield
// is a new object with its front face's characteristics, CR 712.4d).
func (s *State) faceReset(k *prep, o Obj) {
	ob := &s.objs[o]
	if k.anyRoom && s.roomLeave(k, o) { // face_rooms.go
		return
	}
	if cd := &k.t.Cards[ob.card]; cd.Face != 0 && len(cd.Faces) > 0 {
		s.touch(o)
		ob.card = cd.Faces[0]
	}
}
