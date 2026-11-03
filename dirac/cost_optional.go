package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// S:Mode$ OptionalCost (CR 601.2b, "as an additional cost to
// cast this spell, you may blight 1 / behold a Dragon"), copied from gorge
// (rules statics_costmods.go optionalCostViews, legal_walk_hand.go's hand
// walk, cast_begin.go's "optionalcost" charge, cast_targets.go's
// FlagOptionalCostPaid):
//
//   - the optional cast is its own priority option (gorge Mode
//     "optionalcost", AltCostIndex 1), offered beside the plain cast when
//     the part is payable and the spell has its targets. There is no
//     in-cast election. A card never has Kicker, Bargain, Offspring or
//     another optional cost with it (kgen refuses it), so the cast is the
//     kicked one (AkCast mode 1) and CardDef.OptCost/OptBehold pick the row.
//   - the part settles through the ordinary cost stages: a behold (the
//     card's BeholdN/BeholdF, ChCostBehold) or a Blight<N> (the pkBlight
//     pick, CardDef.OptCost), behold before blight as gorge's
//     costObjectAsks orders them.
//   - the paid flag rides the kicked provenance: the cast sets sfKicked and
//     ExKicked (Count$OptionalGenericCostPaid.<paid>.<unpaid>) and CondKicked
//     (Condition$ OptionalCost) read it. gorge keeps FlagOptionalCostPaid on
//     a copy (it is not in state.CastProvenanceFlags; CR 707.10 copies the
//     additional-cost decision), so copySpell keeps sfKicked.
//   - gorge's planner prices no "optionalcost" mode, so under either oracle
//     surface the cast is offered only when the floating pool alone pays
//     it, as an Offspring cast (gorgeOffersKicked).
//
// Every path is behind genPrep.anyOptCost. Instants and sorceries, plus a
// permanent with a CollectEvidence part, whose self-ETB trigger reads the
// permanent's kicked provenance (CastSA>Collected; kgen lower_collectedevidence.go).

// DebugOptCostNeverPaid (mutation test only): ExKicked and CondKicked read
// an OptionalCost spell as unpaid.
var DebugOptCostNeverPaid bool

// optCostCard: card c has an OptionalCost part.
func optCostCard(k *prep, c CardID) bool {
	cd := &k.t.Cards[c]
	return cd.OptCost.Flags != 0 || cd.OptBehold || cd.OptEvidence != 0
}

// optCostOffer: p may cast o (card c) from hand paying its optional part.
// The plain cast's mana, targets and pick costs were gated by the caller;
// the part adds no mana.
func (s *State) optCostOffer(k *prep, p uint8, c CardID, o Obj) bool {
	if !optCostCard(k, c) {
		return false
	}
	cd := &k.t.Cards[c]
	if cd.OptBehold && s.beholdCands(k, p, c, o, nil) < int(cd.BeholdN) {
		return false
	}
	if cd.OptEvidence != 0 && s.graveTotalMV(k, p) < int(cd.OptEvidence) {
		return false // CollectEvidence<N> needs a graveyard worth N (gorge pay.EvidencePayable)
	}
	if cd.OptCost.Flags != 0 && !s.costObjPayable(k, p, &cd.OptCost, NoObj, o) {
		return false
	}
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) == 0 || s.g.availX&availWalk != 0 {
		return true
	}
	return s.FeatPoolPays(p, o, 1)
}

// optCostPick is the cost whose pick parts a paid OptionalCost spell owes
// (its Blight<N> part), or nil.
func optCostPick(k *prep, e *stackEntry) *kop.Cost {
	if e.flags&sfKicked == 0 {
		return nil
	}
	if cd := &k.t.Cards[e.card]; cd.OptCost.Flags != 0 {
		return &cd.OptCost
	}
	return nil
}

// optCostBehold: the spell being built owes its OptionalCost behold.
func optCostBehold(k *prep, e *stackEntry) bool {
	return e.flags&sfKicked != 0 && k.t.Cards[e.card].OptBehold
}

// optCostUnpaid: the mutation hook reads e's paid OptionalCost as unpaid.
func optCostUnpaid(k *prep, e *stackEntry) bool {
	return DebugOptCostNeverPaid && e.kind == seSpell && optCostCard(k, e.card)
}

// FeatOptCostCard reports whether o's card has an OptionalCost part (the
// oracle's cast mode choice; cost_optional.go).
func (s *State) FeatOptCostCard(o Obj) bool {
	return optCostCard(s.k(), s.objs[o].card)
}
