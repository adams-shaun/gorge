package dirac

// a permanent spell's CollectEvidence<N>
// OptionalCost part (Crimestopper Sprite, Vitu-Ghazi Inspector). The paid flag
// rides the cast as sfKicked and moves to the permanent's kicked mark on entry
// (enterGen, mechKicked), which the card's own self-ETB trigger reads as
// Card.Self+kicked (F2Kicked). gorge reads the same fact as
// "CastSA>Collected" off the permanent's own cast window; it is empty once
// the permanent has left the battlefield (WasCast ends on any later zone
// change), and F2Kicked on a gone Self reads false the same way. No State
// byte: mechKicked is the existing mark.

// DebugCastCollectedNever (mutation test only): an evidence OptionalCost
// permanent's paid flag is not copied to its kicked mark on entry, so
// CastSA>Collected reads as not collected.
var DebugCastCollectedNever bool

// castCollectedNever: the mutation hook for e's entry (cold; behind
// genPrep.anyOptCost at the call).
func castCollectedNever(k *prep, e *stackEntry) bool {
	return DebugCastCollectedNever && k.t.Cards[e.card].OptEvidence != 0
}

// CollectEvidence<N> as a spell's OptionalCost part
// (CR 701.59a, 601.2b/h; Analyze the Pollen, Extract a Confession, Behind the
// Mask). The optional cast is the kicked cast (cost_optional.go), so the
// paid flag is the stack entry's sfKicked and the spell-side reads of
// "Collected" lower to the kicked condition / value (gorge reads the group
// through CostMovesInWindow over the spell's own push: non-empty exactly when
// the part was paid, since CollectEvidence<N>, N >= 1, exiles a card).
//
// The offer needs a graveyard worth N (optCostOffer; gorge pay.EvidencePayable,
// CR 601.2b): the seSpell program carries bsEvidence after the targets and the
// division (gorge's postTargetAsks), the pick is the activation's
// (cost_collectevidence.go), and finishBuild exiles the picked cards
// with the other cost objects (payOptEvidence). Cold and out of line.

// optEvidenceN is the evidence threshold the spell being built owes, 0 for
// none (not an OptionalCost cast, or a part other than evidence).
func (s *State) optEvidenceN(k *prep) uint8 {
	e := &s.bld().e
	if e.flags&sfKicked == 0 {
		return 0
	}
	return k.t.Cards[e.card].OptEvidence
}

// payOptEvidence exiles the spell's picked graveyard cards (gorge
// cast_commit.go: EvidenceCost per card, after the exile parts).
//
//go:noinline
func (s *State) payOptEvidence(k *prep) {
	if k.t.Cards[s.bld().e.card].OptEvidence == 0 {
		return
	}
	if s.exileEvidence() > 0 && k.gen.anyExplEvTrig {
		s.fireEvidence(k, s.bld().e.ctrl) // trig_exploreevidence.go
	}
}
