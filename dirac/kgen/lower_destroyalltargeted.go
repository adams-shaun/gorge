package kgen

// destroyAllFilter reads a DestroyAll's ValidCards$. Two Targeted spellings
// are lowered whole; anything else goes through the ordinary filter parse.
//
//   - Maelstrom Pulse's "the target and every other permanent with its name"
//     is one PredSameNameT0 clause (sameNameT0Filter).
//   - destroyAllAttachedSpec (Turn to Slag): every Equipment attached to the
//     chain's target. gorge destroyAllTargetedMatcher anchors the spec on the
//     target and reads Attached as attached TO that anchor, which is the
//     kernel's F2AttachedToT0 word ("Equipment.AttachedTo Targeted", the same
//     referent as ParentTarget). The DestroyAll declares no target slot of
//     its own and the chain has one, so target 0 is that slot's object; the
//     spell resolves only while it is a legal target (CR 608.2b), so the word
//     is never read against a vanished anchor.
func (lc *lowerCtx) destroyAllFilter(ps *params) (*lFilter, error) {
	vc := ps.str("ValidCards")
	switch vc {
	case "TargetedCard.Self,Permanent.NotDefinedTargeted+sharesNameWith Targeted":
		return lc.sameNameT0Filter() // lower_filterreferents.go
	}
	if rest, ok := sameNameSweepRest(vc); ok {
		// Echoing Calm, Echoing Ruin, Wake of Destruction: the same sweep
		// over one type (lower_filterreferents.go)
		return lc.sameNameT0BattleFilter(rest)
	}
	switch vc {
	case destroyAllAttachedSpec:
		if ps.peek("ValidTgts") != "" || lc.ab == nil || len(lc.ab.Targets) != 1 ||
			lc.ab.Targets[0].F == nil || lc.ab.Targets[0].F.Players != 0 {
			return nil, refuse("DestroyAll %q without one object target slot upstream", vc)
		}
		lc.attachedTgtOK = true
		defer func() { lc.attachedTgtOK = false }()
		return lc.filter("Equipment.AttachedTo Targeted")
	}
	return lc.filter(vc)
}

// destroyAllAttachedSpec is Turn to Slag's ValidCards$: the target's Equipment.
const destroyAllAttachedSpec = "Targeted.Equipment+Attached"
