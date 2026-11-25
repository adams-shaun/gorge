package kgen

// The Defined$ values the 11-25 census lowered (TestDefinedCensus1125). Each
// maps to a gorge definedSpec reading the kernel already names; every other
// shape of the value stays refused by name.

// triggeredDefenderDefined is the bare Defined$ TriggeredDefender: gorge's
// context.go TriggeredDefender arm (definedSpec, the unqualified base)
// returns c.DefendingPlayer, the seat a Mode$ Attacks trigger binds for its
// attacker, which is a player even when the attacker attacks a planeswalker
// or battle (DefenderObj rides beside it). That is the same seat
// definedSpecTriggeredDefendingPlayer reads on that trigger, so the bare
// value lowers as TriggeredDefendingPlayer does, on an Attacks trigger of the
// controller's own attacker only (ownAttackTrigger; Unquenchable Fury's
// granted "whenever this creature attacks"). Another attacker's Attacks
// trigger (Calculating Lich) and every other mode stay refused.
func (lc *lowerCtx) triggeredDefenderDefined(v string) (lRef, error) {
	if tr := lc.ab.Trigger; tr == nil || tr.Event != "Attacks" || !lc.ownAttackTrigger() {
		return lRef{}, refuse("Defined$ %q outside an Attacks trigger of the controller's own attacker", v)
	}
	return lRef{Kind: rDefending}, nil
}

// targetedCardDefined is Defined$ TargetedCard: gorge definedSpecTargetedCard
// (context.go), the object targets of refTargetUnion, the same chain union
// Defined$ Targeted reads with its player targets dropped. Over a chain of one
// target slot that admits no player it is that slot exactly, the reading
// Targeted lowers to (Will of the Sultai's "it gains trample" after the
// PutCounter link's creature target). A chain of several slots, or a slot a
// player can answer, stays refused.
func (lc *lowerCtx) targetedCardDefined(v string) (lRef, error) {
	if len(lc.chainT) != 1 {
		return lRef{}, refuse("Defined$ %q over %d target slots", v, len(lc.chainT))
	}
	t := lc.chainT[0]
	if f := lc.ab.Targets[t].F; f == nil || f.Players != 0 {
		return lRef{}, refuse("Defined$ %q of a target slot a player can answer", v)
	}
	return lRef{Kind: rTarget, N: t}, nil
}
