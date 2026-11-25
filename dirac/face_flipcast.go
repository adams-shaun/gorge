package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Back-face casts of a transforming double-faced card, cold: reached only
// when the table has a card with faces (genPrep.anyFaces) and its front
// carries a back-face cast keyword (CardDef.FlipCast):
//
//   - More Than Meets the Eye (CR 702.162a, kop.FlipConverted): the card is
//     cast converted from the hand, back face up, for the keyword's cost:
//     gorge's "converted" (rules legal_walk_hand.go appendConvertedCast);
//   - disturb (CR 702.146a, kop.FlipDisturb): the card is cast transformed
//     from its owner's graveyard for the keyword's cost: gorge's "disturb"
//     (rules legal_walk_alt.go disturbOffers).
//
// Either is a face cast, AkCast with castFace and target 1: applyPriority
// turns the back face up before beginCast, as gorge's flipTransformedCast
// flips the proposal before the ordinary cast transaction, so timing,
// prohibitions and targets are judged on the back face (CR 712.11c), and
// faceCastable prices the cast through the potential planner as gorge's
// potentialModeBaseCostConverted / Disturb do. The cost is the front's
// FlipCost (flipCastCost): prep gives the back face's pcard.cost that cost,
// so the offer's census and the pool tests read it, and buildCost charges
// it. The spell is the back face; resolved, the permanent enters showing it
// (CR 712.11b), and any other exit from the stack turns it front face up
// again (stackExitFace; gorge events apply_zone.go IsTransformingDFC). A
// disturbed back face's "exile it instead" is its own printed replacement.

// flipCastFront is the front def of back face c when its front carries a
// back-face cast keyword; nil otherwise.
func flipCastFront(t *kop.Table, c CardID) *kop.CardDef {
	cd := &t.Cards[c]
	if cd.FaceMode != kop.FaceTransform || cd.Face != 1 || len(cd.Faces) != 2 {
		return nil
	}
	if f := &t.Cards[cd.Faces[0]]; f.FlipCast != kop.FlipNone {
		return f
	}
	return nil
}

// flipCastCost is the cost a cast of back face c pays in place of a mana
// cost: the front's FlipCost (CR 702.162a, 702.146a). ok is false for every
// other card.
func flipCastCost(t *kop.Table, c CardID) (kop.ManaCost, bool) {
	if f := flipCastFront(t, c); f != nil {
		return f.FlipCost, true
	}
	return kop.ManaCost{}, false
}

// flipCastable reports whether front card c (object o, front face up) is a
// FaceTransform card whose back face p may cast now by keyword kind.
func (s *State) flipCastable(k *prep, p uint8, o Obj, c CardID, kind uint8, sorc bool) bool {
	cd := &k.t.Cards[c]
	if cd.FaceMode != kop.FaceTransform || cd.Face != 0 || len(cd.Faces) != 2 || cd.FlipCast != kind {
		return false
	}
	return s.faceCastable(k, p, o, cd.Faces[1], sorc)
}

// offerDisturbCasts offers casting each disturb card in p's graveyard
// transformed (gorge legal_walk_alt.go disturbOffers, in graveyard order,
// after the aftermath halves). The card leaves the graveyard as it is cast,
// so it is no candidate for the back face's own targets (faceCastable).
func (s *State) offerDisturbCasts(k *prep, d *Decision, p uint8, sorc bool) {
	pl := &s.pl[p]
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		if s.flipCastable(k, p, o, s.objs[o].card, kop.FlipDisturb, sorc) {
			d.add(MakeAction(AkCast, TCode(o), 1, castFace))
		}
	}
}
