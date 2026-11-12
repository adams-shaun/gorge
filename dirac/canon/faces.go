package canon

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
	FaceCodeModal       = 1
	FaceCodeAdventure   = 2
	FaceCodeRecast      = 3
	FaceCodeSplit       = 4
	FaceCodeAftermath   = 6 // a split card's aftermath half cast from the graveyard (gorge "aftermath", rules legal_walk_alt.go; dirac/face_aftermath.go)
	FaceCodeMorphed     = 8
	FaceCodeMegamorphed = 9
	FaceCodeDisguised   = 10
	FaceCodePrepared    = 12 // a prepared permanent's exile copy (face_prepare.go; gorge "prepared_copy")
	FaceCodeRoomAlt     = 13 // a Room's other door (gorge "room_alt", rules legal_walk_hand.go; the face nibble is 0: the route names it)
	FaceCodeSneak       = 14 // a hand cast for its sneak cost (gorge "sneak", rules kw_sneak.go; kernel AkSneak, kw_sneak.go): castModes is full
	FaceCodeOmen        = 15 // an Omen card's face cast from the hand (gorge "omen_alt", rules legal_walk_hand.go; dirac/face_omen.go)
)

// FollowedFaceRoute reports whether the kernel follows the planner for a
// face route's potential play (prices every face route the kernel
// casts; the face-down routes are not face casts of this table).
func FollowedFaceRoute(fc uint8) bool {
	switch fc & 15 {
	case FaceCodeModal, FaceCodeAdventure, FaceCodeRecast, FaceCodeSplit,
		FaceCodeOmen, FaceCodePrepared, FaceCodeSneak, FaceCodeRoomAlt, FaceCodeAftermath:
		return true
	}
	return false
}

// FaceCastCode is the ActCastFace mode of a face route's Option.Mode; ok
// is false for every other mode.
func FaceCastCode(mode string) (uint8, bool) {
	for i := 1; i < len(faceCastModes); i++ {
		if faceCastModes[i] == mode {
			return faceCastFaces[i]<<4 | uint8(i), true
		}
	}
	return 0, false
}

// CastOptCanon is a "cast" option's canonical action: ActCastFace for a
// face route, else ActCast with the cast mode and the AltCostIndex.
func CastOptCanon(src Ref, mode string, alt int) Canon {
	if fc, ok := FaceCastCode(mode); ok {
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
	mc, _ := CastModeCode(mode)
	return Pack(ActCast, src, RefNone, mc|uint8(alt)<<4)
}

// faceCastCanon is the ActCastFace mode of the kernel's face cast of
// object src as face i (dirac.FeatCastFace): the route follows from the
// card's layout and the face.
func FaceDownCastCanon(fam uint8) uint8 {
	code := uint8(FaceCodeMorphed)
	switch fam {
	case 2:
		code = FaceCodeMegamorphed
	case 3:
		code = FaceCodeDisguised
	}
	return faceCastFaces[code]<<4 | code
}

func (r *Translator) faceCastCanon(src, i uint8) uint8 {
	code := uint8(0)
	switch cd := &r.Tbl.Cards[r.S.ObjCard(dirac.Obj(src))]; {
	case cd.FaceMode == kop.FaceModal && i == 1:
		code = FaceCodeModal
	case cd.FaceMode == kop.FaceAdventure && i == 1:
		code = FaceCodeAdventure
	case cd.FaceMode == kop.FaceAdventure:
		code = FaceCodeRecast
	case cd.FaceMode == kop.FaceSplit && i == 1 && len(cd.Faces) == 2 && r.Tbl.Cards[cd.Faces[1]].Aftermath:
		code = FaceCodeAftermath
	case cd.FaceMode == kop.FaceSplit && i == 1:
		code = FaceCodeSplit
	case cd.FaceMode == kop.FacePrepare && i == 1:
		code = FaceCodePrepared
	case cd.FaceMode == kop.FaceRoom && i != cd.Face:
		code = FaceCodeRoomAlt
	case cd.FaceMode == kop.FaceOmen && i == 1:
		code = FaceCodeOmen
	}
	return faceCastFaces[code]<<4 | code
}

// CastModeOf is the Option.Mode of a cast's canonical action.
func CastModeOf(c Canon) string {
	switch c.Kind() {
	case ActCastFace:
		return faceCastModes[c.Mode()&15]
	case ActCastCasualty:
		return "casualty"
	case ActCastImpended:
		return "impended"
	case ActCastAirbend:
		return "airbend_cast"
	}
	return CastModeName(c.Mode() & 15)
}

// BeamAlt is the alt-cost nibble of a gorge cast option: its AltCostIndex, but 1
// for the second beam me up permission of a card (rules beamMeUpInstances: the
// printed one first, then a grant at the mana cost), which gorge tells apart by
// Option.Cost alone.
func BeamAlt(mode string, alt int, cost string) int {
	if mode == "beam_me_up" && cost != "" {
		return 1
	}
	return alt
}

// CastAlt is the Option.AltCostIndex of a cast's canonical action.
func CastAlt(c Canon) int {
	switch c.Kind() {
	case ActCast, ActCastCasualty, ActCastImpended:
		return int(c.Mode() >> 4)
	}
	return 0
}
