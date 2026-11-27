package kgen

import "strings"

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

// definedCompound splits a compound Defined$ "A & B" value into its members'
// refs, in script order. gorge's " & " joins independent selectors, not their
// intersection (knownDefinedTargets, context.go; DefinedRef's RefCompound
// arm): each member resolves on its own over the same ability and the effect
// acts once per member. Only an effect subject whose op can run once per
// object/player uses this; every other caller keeps refusing the compound
// through defined.
func (lc *lowerCtx) definedCompound(v string) ([]lRef, error) {
	parts := strings.Split(v, " & ")
	if len(parts) < 2 {
		return nil, refuse("Defined$ %q: not a compound", v)
	}
	out := make([]lRef, 0, len(parts))
	for _, p := range parts {
		r, err := lc.defined(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// objectSubject reports whether r names an object a one-object op can act on.
// A compound member that names a player (or a set the kernel cannot run
// per-object) keeps the whole compound refused.
func objectSubject(r lRef) bool {
	switch r.Kind {
	case rSelf, rTriggered, rTriggeredNew, rTriggeredSource, rRemembered,
		rDelayedRemembered, rReplaced, rChosen, rEach, rSacrificed, rHost,
		rEquipped, rTarget:
		return true
	}
	return false
}

// playerSubject reports whether r names a player (or a player set's single
// read) that a player effect can act on. Object refs are refused: gorge would
// map an object to its controller, which the op's own subject path does not
// promise here.
func playerSubject(r lRef) bool {
	switch r.Kind {
	case rYou, rEachOpp, rTriggeredPlayer, rTriggeredTgtCtrl,
		rDefending, rTargetController, rAttackedBySource, rRememberedOwner,
		rPromised, rChosenPlayer, rResChosenPlayer, rExiledWithOwner,
		rEnchantedPlayer:
		return true
	}
	return false
}
