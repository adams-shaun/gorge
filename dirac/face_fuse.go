package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Fuse (CR 702.102), cold: reached only when the table holds a split card
// printed with fuse (genPrep.anyFuse; CardDef.FuseDef). Cast from the hand,
// such a card may be cast as both halves at once, one fused split spell
// (gorge's "fuse" option, rules legal_walk_hand.go). kgen lowers that spell
// as its own def (dirac/kgen lower_fuse.go): the halves' record combined
// (CR 702.102b-c, 709.4d: the costs summed, the colours and types joined)
// and one spell ability, the left half's program then the right half's
// (CR 702.102d), the target slots in gorge's stage order.
//
// The cast is a face cast, AkCast with castFace and target FeatFuseFace: the
// hand card turns to the fused def for it (turnFaceCast), shows its front
// throughout (the def's Face is 0, as gorge's fused cast flips nothing),
// and is its front again as it leaves the stack (stackExitFace) or its cast
// is undone (unturnFace). The offer reads gorge's gates: the fused record's
// speed (gorge fusedTimingOK: both halves instants, or sorcery timing; kgen
// refuses a mixed pair), each half's prohibitions (castForbiddenFace), the
// targets of both halves and the summed cost. gorge's potential planner
// prices no "fuse" mode, so on the lockstep surfaces the fused cast is
// offered only when the floating pool pays it (its legacy option).

// FeatFuseFace is the AkCast target of a fused cast (mode FeatCastFace).
const FeatFuseFace uint8 = 2

// DebugFuseAnyPool offers the fused cast from the mana census on the lockstep
// surfaces too (mutation testing only: gorge offers it only when the
// floating pool pays).
var DebugFuseAnyPool bool

// DebugFuseNoOffer withholds the fused cast (mutation testing only).
var DebugFuseNoOffer bool

// fusedDef is card c's fused split spell def, ok false for a card without
// fuse.
func fusedDef(k *prep, c CardID) (CardID, bool) {
	if fd := k.t.Cards[c].FuseDef; fd != 0 {
		return fd - 1, true
	}
	return 0, false
}

// offerFuseCast offers hand card o's fused cast (gorge's "fuse" option).
func (s *State) offerFuseCast(k *prep, d *Decision, p uint8, o Obj, sorc bool) {
	fd, ok := fusedDef(k, s.objs[o].card)
	if !ok || DebugFuseNoOffer {
		return
	}
	if s.faceCastable(k, p, o, fd, sorc) {
		d.add(MakeAction(AkCast, uint8(o), FeatFuseFace, castFace))
	}
}

// fusePoolGate reports whether the fused spell c that object o shows for the
// probe may be offered on this surface: off the lockstep surfaces always,
// on them only when the floating pool pays (gorge's planner prices no fused
// cast). faceCastable calls it with the probe in place.
func (s *State) fusePoolGate(k *prep, p uint8, o Obj, c CardID) bool {
	return !k.t.Cards[c].FusedSpell || s.cfg&cfgGorgeUnpriced == 0 || DebugFuseAnyPool || s.FeatPoolPays(p, o, 0)
}

// castForbiddenFace is castForbidden for a face cast of o as c. A fused
// split spell is judged half by half, as gorge's fuse offer judges it (the
// card-level castRestricted of the front face and castRestrictedAsFace of
// the right half), never as its combined record. The caller's probe shows c;
// it is restored before returning.
func (s *State) castForbiddenFace(k *prep, p uint8, c CardID, o Obj) bool {
	cd := &k.t.Cards[c]
	if !cd.FusedSpell || len(cd.Faces) != 2 {
		return s.castForbidden(k, p, c, o)
	}
	ob := &s.objs[o]
	forbidden := false
	for _, h := range cd.Faces {
		ob.card = h
		if s.castForbidden(k, p, h, o) {
			forbidden = true
			break
		}
	}
	ob.card = c
	return forbidden
}

// fuseTurn turns hand card o to its fused spell for the cast (turnFaceCast);
// false when o's card has no fuse.
func (s *State) fuseTurn(k *prep, o Obj) bool {
	fd, ok := fusedDef(k, s.objs[o].card)
	if !ok {
		return false
	}
	s.touch(o)
	s.objs[o].card = fd
	return true
}

// fusedFront is the front of fused spell def c, ok false for any other def:
// the card as it leaves the stack or its cast is undone (CR 709.4).
func fusedFront(k *prep, c CardID) (CardID, bool) {
	if cd := &k.t.Cards[c]; cd.FusedSpell && len(cd.Faces) == 2 {
		return cd.Faces[0], true
	}
	return 0, false
}

// FuseFaceCast reports whether a face cast of a card showing cd as face i
// is its fused cast (dirac/canon's FaceCodeFuse).
func FuseFaceCast(cd *kop.CardDef, i uint8) bool {
	return i == FeatFuseFace && cd.FaceMode == kop.FaceSplit && cd.FuseDef != 0
}
