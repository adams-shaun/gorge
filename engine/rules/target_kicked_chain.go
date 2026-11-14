package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// kickedChainTargetsAvailable is the kicked cast's half of the offer census
// (CR 601.2b, 601.2c): chainTargetsAvailable leaves a Condition$ Kicked link
// unjudged because the plain cast owes it nothing, but the kicked cast
// announces it, and a TargetUnique$ link among them ("if kicked, return
// another target permanent card from your graveyard", Urborg Repossession)
// must be answerable together with the root's target by pairwise distinct
// targets. Offering the kicked cast on a graveyard with one permanent card
// could only end in the CR 733.1 reversal ("cast aborted: no legal target":
// the root ask withholds the lone card because it leaves the link nothing).
//
// Only a chain with a Kicked-conditioned unique link is judged here; every
// other chain is the plain census's alone. The slots are the root and every
// unique link the kicked cast announces, each at its kicked minimum
// (kickedTargetMin), judged by the same joint assignment
// (distinctTargetsFeasible) the plain census runs.
func (e *Engine) kickedChainTargetsAvailable(p state.PlayerID, id state.ObjID, root *cards.SA) bool {
	if root == nil || root.Sub == nil {
		return true
	}
	var svars map[string]string
	if o := e.G.Obj(id); o != nil && o.Face() != nil {
		svars = o.Face().SVars
	}
	var uniq []*cards.SA
	kickedLink := false
	for _, sub := range e.collectSubTargetPreAsks(root) {
		cond := strings.TrimSpace(sub.ParamStr(cards.PKCondition))
		if strings.EqualFold(cond, "OptionalCost") || !effects.TargetUniqueRequested(sub) || subTargetingReadsRootTarget(sub, svars) {
			continue
		}
		kickedLink = kickedLink || strings.EqualFold(cond, "Kicked")
		uniq = append(uniq, sub)
	}
	if !kickedLink {
		return true
	}
	slots := make([]uniqueTargetSlot, 0, len(uniq)+1)
	if effects.TargetsOf(root).Targeted() && !announceShapesTargets(e, id, root) {
		if need, dynamic := targetOfferMin(e, p, id, root, 0, true, nil); !dynamic && need > 0 {
			slots = append(slots, uniqueTargetSlot{cands: e.legalTargetCandidates(p, id, id, root), need: need})
		}
	}
	for _, sub := range uniq {
		need := e.kickedTargetMin(p, id, sub)
		if need <= 0 {
			continue
		}
		subExclude := id
		slots = append(slots, uniqueTargetSlot{cands: e.legalTargetCandidates(p, id, subExclude, sub), need: need})
	}
	return distinctTargetsFeasible(slots)
}

// kickedTargetMin is sa's mandatory target minimum as the KICKED cast
// announces it: a bound reading Count$Kicked (TargetMin$ Y over
// SVar:Y:Count$Kicked.1.0) resolves with the cast's kicked bit set, the
// pre-payment binding the cast's own announcement ask gives it
// (targetBoundCtx, Kicker.PendingKicked).
func (e *Engine) kickedTargetMin(p state.PlayerID, id state.ObjID, sa *cards.SA) int {
	min, _ := targetBounds(sa)
	tp := effects.TargetsOf(sa)
	if !targetBoundsDynamic(sa) || !tp.Min.Present || isLiteralBound(tp.Min.Text) {
		return min
	}
	ctx, ok := e.targetBoundCtx(p, id)
	if !ok {
		return min
	}
	ctx.Kicker.PendingKicked = true
	if n, resolved := effects.NumTextResolvedStrict(e, ctx, tp.Min, 1); resolved {
		min = int(n)
	}
	return min
}
