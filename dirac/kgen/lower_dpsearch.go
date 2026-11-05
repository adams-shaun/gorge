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
	case "TriggeredCardController", "TriggeredActivator", "TriggeredPlayer", "TriggeredTarget":
		// The trigger/controller spellings share the one Defined grammar and
		// its guards (definedSpecTriggeredPlayer's event gate,
		// definedSpecTriggeredTarget's damage-to-player guard): never a
		// second copy that can drift from the Defined$ reader.
		r, err := lc.defined(v)
		if err != nil {
			return lRef{}, refuse("ChangeZone DefinedPlayer$ %s: %v", v, err)
		}
		return r, nil
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
