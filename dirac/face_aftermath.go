package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Aftermath (CR 702.127a), cold: reached only when the table holds a split
// card whose right half carries aftermath (genPrep.anyAftermath; the face-1
// def's CardDef.Aftermath). The keyword's three static abilities:
//
//   - the right half may be cast from its owner's graveyard: gorge's
//     "aftermath" option (rules legal_walk_alt.go's graveyard loop over
//     aftermathAlternateFace), here AkCast with castFace and target 1 from
//     the graveyard (offerAftermathCasts), for the half's own mana cost,
//     with the half's own timing, prohibitions and targets (faceCastable);
//   - it can't be cast from any other zone: the hand walk offers no
//     split_alt for it (offerFaceCasts; gorge's splitAlternateCastFace
//     excludes an Aftermath half);
//   - cast from a graveyard, it is exiled whenever it would leave the
//     stack: beginCast stamps sfFlashback (gorge's FlagAftermath is one of
//     the flashback family state.ExilesLeavingStack names), so a resolution,
//     a fizzle, a counter and a library or hand destination exile it. Off
//     the stack it is its front again (stackExitFace; gorge apply_zone.go
//     resets FaceIdx, CR 709.4).

// DebugAftermathGraveyard leaves the exile-on-leave mark off an aftermath
// cast, so a resolved one rests in its owner's graveyard (mutation testing
// only: the lockstep must catch it).
var DebugAftermathGraveyard bool

// DebugAftermathFromHand offers an aftermath half from the hand as a plain
// split half (mutation testing only).
var DebugAftermathFromHand bool

// aftermathHalf reports whether card c is the front of a split card whose
// right half carries aftermath.
func aftermathHalf(k *prep, c CardID) bool {
	cd := &k.t.Cards[c]
	return cd.FaceMode == kop.FaceSplit && cd.Face == 0 && len(cd.Faces) == 2 && k.t.Cards[cd.Faces[1]].Aftermath
}

// offerAftermathCasts offers casting the aftermath half of each split card in
// p's graveyard (gorge legal_walk_alt.go's aftermath loop, in graveyard
// order). The card leaves the graveyard as it is cast, so it is no candidate
// for the half's own targets (faceCastable's spellTargetsFeasibleFrom; gorge
// castTargetsAvailable names the cast card).
func (s *State) offerAftermathCasts(k *prep, d *Decision, p uint8, sorc bool) {
	pl := &s.pl[p]
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		c := s.objs[o].card
		if !aftermathHalf(k, c) {
			continue
		}
		if s.faceCastable(k, p, o, k.t.Cards[c].Faces[1], sorc) {
			d.add(MakeAction(AkCast, TCode(o), 1, castFace))
		}
	}
}

// aftermathCastFlags is the stack-entry flag an aftermath cast of object o,
// showing card c as it is cast, carries: sfFlashback, the exile-on-leave and
// cast-from-graveyard mark, when c is an aftermath half cast from a
// graveyard (CR 702.127a); none otherwise.
func (s *State) aftermathCastFlags(k *prep, o Obj, c CardID, mode uint8) uint8 {
	if mode&castFace == 0 || !k.t.Cards[c].Aftermath || s.objs[o].zone != kop.ZoneGraveyard || DebugAftermathGraveyard {
		return 0
	}
	return sfFlashback
}
