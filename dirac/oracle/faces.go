package oracle

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Casting a face. gorge offers a face cast
// as a "cast" option whose Mode names the face route; the canonical form
// is ActCastFace with mode = face<<4 | the route's code (faceCastModes),
// beside ActCast, whose low nibble (castModes) is full.

// faceCastModes are the face routes' Option.Mode spellings, by code (0 is
// none); faceCastFaces is the face each casts (gorge's FlipFace target:
// rules cast_begin.go; a fused cast and the adventure-zone recast keep the
// front up).
var (
	faceCastModes = []string{"", "modal_spell", "adventure_alt", "adventure_recast", "split_alt", "fuse", "aftermath", "warp_recast", "morphed", "megamorphed", "disguised", "plot_cast", "prepared_copy", "room_alt", "sneak", "omen_alt"}
	faceCastFaces = []uint8{0, 1, 1, 0, 1, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 1}
)

// The face route codes the kernel casts (dirac/face_castface.go).
const (
	faceCodeModal       = 1
	faceCodeAdventure   = 2
	faceCodeRecast      = 3
	faceCodeSplit       = 4
	faceCodeMorphed     = 8
	faceCodeMegamorphed = 9
	faceCodeDisguised   = 10
	faceCodePrepared    = 12 // a prepared permanent's exile copy (face_prepare.go; gorge "prepared_copy")
	faceCodeRoomAlt     = 13 // a Room's other door (gorge "room_alt", rules legal_walk_hand.go; the face nibble is 0: the route names it)
	faceCodeSneak       = 14 // a hand cast for its sneak cost (gorge "sneak", rules kw_sneak.go; kernel AkSneak, kw_sneak.go): castModes is full
	faceCodeOmen        = 15 // an Omen card's face cast from the hand (gorge "omen_alt", rules legal_walk_hand.go; dirac/face_omen.go)
)

// followedFaceRoute reports whether the kernel follows the planner for a
// face route's potential play (prices every face route the kernel
// casts; the face-down routes are not face casts of this table).
func followedFaceRoute(fc uint8) bool {
	switch fc & 15 {
	case faceCodeModal, faceCodeAdventure, faceCodeRecast, faceCodeSplit,
		faceCodeOmen, faceCodePrepared, faceCodeSneak, faceCodeRoomAlt:
		return true
	}
	return false
}

// faceCastCode is the ActCastFace mode of a face route's Option.Mode; ok
// is false for every other mode.
func faceCastCode(mode string) (uint8, bool) {
	for i := 1; i < len(faceCastModes); i++ {
		if faceCastModes[i] == mode {
			return faceCastFaces[i]<<4 | uint8(i), true
		}
	}
	return 0, false
}

// castOptCanon is a "cast" option's canonical action: ActCastFace for a
// face route, else ActCast with the cast mode and the AltCostIndex.
func castOptCanon(src Ref, mode string, alt int) Canon {
	if fc, ok := faceCastCode(mode); ok {
		return Pack(ActCastFace, src, RefNone, fc)
	}
	if mode == "casualty" {
		return Pack(ActCastCasualty, src, RefNone, uint8(alt)<<4)
	}
	if mode == "airbend_cast" {
		return Pack(ActCastAirbend, src, RefNone, 0)
	}
	if mode == "impended" {
		return Pack(ActCastImpended, src, RefNone, uint8(alt)<<4)
	}
	mc, _ := castModeCode(mode)
	return Pack(ActCast, src, RefNone, mc|uint8(alt)<<4)
}

// faceCastCanon is the ActCastFace mode of the kernel's face cast of
// object src as face i (dirac.FeatCastFace): the route follows from the
// card's layout and the face.
func faceDownCastCanon(fam uint8) uint8 {
	code := uint8(faceCodeMorphed)
	switch fam {
	case 2:
		code = faceCodeMegamorphed
	case 3:
		code = faceCodeDisguised
	}
	return faceCastFaces[code]<<4 | code
}

func (r *realSide) faceCastCanon(src, i uint8) uint8 {
	code := uint8(0)
	switch cd := &r.tbl.Cards[r.s.ObjCard(dirac.Obj(src))]; {
	case cd.FaceMode == kop.FaceModal && i == 1:
		code = faceCodeModal
	case cd.FaceMode == kop.FaceAdventure && i == 1:
		code = faceCodeAdventure
	case cd.FaceMode == kop.FaceAdventure:
		code = faceCodeRecast
	case cd.FaceMode == kop.FaceSplit && i == 1:
		code = faceCodeSplit
	case cd.FaceMode == kop.FacePrepare && i == 1:
		code = faceCodePrepared
	case cd.FaceMode == kop.FaceRoom && i != cd.Face:
		code = faceCodeRoomAlt
	case cd.FaceMode == kop.FaceOmen && i == 1:
		code = faceCodeOmen
	}
	return faceCastFaces[code]<<4 | code
}
