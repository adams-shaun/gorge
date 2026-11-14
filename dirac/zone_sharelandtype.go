package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ShareLandType$ True on a library search (SrShareLandType in OpSearchEx's
// B): the found cards must share a land type ("up to two basic land cards
// that share a land type"). gorge poses the whole answer at once and rejects
// one naming cards with no common land type at Submit (rules.validateSearch,
// through effects.SharedLandTypes); the kernel poses one pick at a time, so a
// later pick is offered only the cards that share a subtype with every card
// already found. The legal answers are the same sets. kgen lowers the flag
// only over a basic-land filter, whose printed subtypes are exactly its land
// types; a card in a library has no other characteristics to read.

// shareLandTypeOp reports a search carrying ShareLandType$ True.
func shareLandTypeOp(in *kop.Instr) bool {
	return in.Op == kop.OpSearchEx && in.B.N&kop.SrShareLandType != 0
}

// shareLandTypeCands is searchCands' library walk once a card is found: a
// matching card is a candidate only if some subtype is common to it and
// every found card. A found card with no subtype (Wastes) leaves none.
//
//go:noinline
func (s *State) shareLandTypeCands(k *prep, in *kop.Instr, d *Decision, list []Obj) {
	p := s.g.pickP
	common := k.t.Cards[s.objs[s.resArm().look[0]].card].Subtypes
	for i := 1; i < int(s.g.nlook); i++ {
		common = common.And(k.t.Cards[s.objs[s.resArm().look[i]].card].Subtypes)
	}
	if !common.Any() {
		return
	}
	for _, o := range list {
		if s.picksHas(o) || !common.And(k.t.Cards[s.objs[o].card].Subtypes).Any() {
			continue
		}
		if s.matchCardGen(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o) {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
}
