package kgen

// definedPlayer lowers a ChangeZone DefinedPlayer$ selector: the player whose
// library the search fetches from. gorge resolves DefinedPlayer$ through the
// shared Defined grammar (params.DefinedPlayerRef -> effects definedPlayerIDs):
// an object target contributes its controller, a player target contributes
// itself, and the trigger/controller spellings name the event's player. Only
// the spellings whose player the kernel already has as a Ref lower; the rest
// refuse.
func (lc *lowerCtx) definedPlayer(v string) (lRef, error) {
	switch v {
	case "You":
		return lRef{Kind: rYou}, nil
	case "Targeted", "ParentTarget", "ParentTargeted":
		t := lc.lastTarget()
		if t < 0 {
			return lRef{}, refuse("ChangeZone DefinedPlayer$ %s with no target slot", v)
		}
		if v == "Targeted" && len(lc.chainT) > 1 {
			return lRef{}, refuse("ChangeZone DefinedPlayer$ Targeted over %d target slots", len(lc.chainT))
		}
		if f := lc.ab.Targets[t].F; f != nil && f.Players != 0 && len(f.Clauses) == 0 {
			return lRef{Kind: rTarget, N: t}, nil
		}
		return lRef{Kind: rTargetController, N: t}, nil
	case "TargetedPlayer":
		t := lc.lastTarget()
		if t < 0 || lc.ab.Targets[t].F == nil || lc.ab.Targets[t].F.Players == 0 || len(lc.ab.Targets[t].F.Clauses) != 0 {
			return lRef{}, refuse("ChangeZone DefinedPlayer$ %s without a player target slot", v)
		}
		return lRef{Kind: rTarget, N: t}, nil
	case "TargetedController":
		t := lc.lastTarget()
		if t < 0 {
			return lRef{}, refuse("ChangeZone DefinedPlayer$ %s with no target slot", v)
		}
		return lRef{Kind: rTargetController, N: t}, nil
	case "TriggeredCardController", "TriggeredActivator", "TriggeredPlayer", "TriggeredTarget", "TriggeredSourceController", "TriggeredAttackingPlayer":
		// The trigger/controller spellings share the one Defined grammar and
		// its guards (definedSpecTriggeredPlayer's event gate,
		// definedSpecTriggeredTarget's damage-to-player guard): never a
		// second copy that can drift from the Defined$ reader.
		r, err := lc.defined(v)
		if err != nil {
			return lRef{}, refuse("ChangeZone DefinedPlayer$ %s: %v", v, err)
		}
		return r, nil
	case "Player.HasCardsInExile_Card.ExiledWithSource_GE1":
		// gorge's shared player-filter grammar (MatchesPlayerSpecFrom, the
		// Player.HasCardsIn<zone>_<objspec>[_<cmp><n>] property): the seat
		// whose exile holds a card exiled with this source (gorge
		// exiledBySource, the forward ExiledCards link plus the reverse
		// ExiledWith scalar), the corpus carrier being Unidentified
		// Hovership's leave trigger. The kernel's RefExiledWithOwner reads
		// the same link (dirac zone_exiledwith.go); gorge acts on EVERY
		// matching seat, the carrier's ETB exiles at most one card, so at
		// most one seat can match. Any other HasCardsIn spelling refuses:
		// the kernel's ref is the exile zone only.
		//
		// The card's own text names the link, so gorge's
		// exileProvenanceNeeded SVar scan stamps the ETB exile with both the
		// forward ExiledCards list and the reverse ExiledWith scalar
		// (context.go moveZoneEvent). Both reader bits are needed for the
		// kernel to record it (genPrep.exwRec: read at exwExile, rev for the
		// scalar); without them the exile records nothing and the leave
		// trigger reads 255.
		lc.out.ExiledWith, lc.out.ExwWord = true, true
		return lRef{Kind: rExiledWithOwner}, nil
	case "RememberedOwner":
		// gorge definedSpecRememberedOwner: the remembered object's owner
		// (CR 108.3). Only the remembered-persist shape lowers it — the
		// enters trigger's set is kept for the leave trigger's resolution
		// (lower_rempersist.go) — anything else refuses.
		if !lc.remPersistShape() {
			return lRef{}, refuse("ChangeZone DefinedPlayer$ %s outside the remembered-persist shape", v)
		}
		return lRef{Kind: rRememberedOwner}, nil
	}
	return lRef{}, refuse("ChangeZone DefinedPlayer$ %s", v)
}
