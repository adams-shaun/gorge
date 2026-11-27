package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// lower_targetmv.go is the pending own-target slot for a Targeted$CardManaCost
// read that happens before the effect declares its one target slot. Behemoth of
// Vault 0's ImmediateTrigger asks its UnlessCost$ PayEnergy<X> before the body
// declares ValidTgts$ Permanent.nonLand, and Kozilek, the Great Distortion's
// AB$ Counter lowers Cost$ Discard<1/Card.cmcEQX> before the Counter declares
// ValidTgts$ Card. gorge reads the ability's chosen targets (c.Targets), so the
// value is known at resolution; kgen points the read at the slot the effect is
// about to append and checks that slot after the effect (targetMVReadable).

// pendTargetMV is the slot a before-the-effect read named, and the target count
// it must grow by exactly one for the read to be the effect's own single target.
type pendTargetMV struct {
	ab   *lAbility
	node *cards.SA
	slot int
	base int
}

// pendingTargetSlot returns the slot the current node's single announced target
// will occupy, when the value read is inside a supported cost part: an
// activated ability's discard filter or an unless amount. The caller arms
// lc.mvNode around that part. A node with TargetMin$/TargetMax$ is not the
// one-target shape this read is proven for.
func (lc *lowerCtx) pendingTargetSlot() (int, bool) {
	n := lc.mvNode
	if n == nil || lc.ab == nil || strings.TrimSpace(n.Params["ValidTgts"]) == "" {
		return 0, false
	}
	if _, ok := n.Params["TargetMin"]; ok {
		return 0, false
	}
	if _, ok := n.Params["TargetMax"]; ok {
		return 0, false
	}
	return len(lc.ab.Targets), true
}

// notePendingTargetMV records a pending read. Its slot is the next slot the
// effect will append; checkPendingTargetMV proves it declared exactly that.
func (lc *lowerCtx) notePendingTargetMV(slot int) {
	if lc.pendMV != nil && lc.pendMV.ab == lc.ab && lc.pendMV.node == lc.mvNode && lc.pendMV.slot == slot {
		return
	}
	lc.pendMV = &pendTargetMV{ab: lc.ab, node: lc.mvNode, slot: slot, base: slot}
}

// bindPendingTargetMV gives a Targeted$CardManaCost read that has no target
// slot yet the slot the current node's effect will declare: an unless amount or
// an activated ability's discard filter reads the effect's own single target
// before the effect declares it.
func (lc *lowerCtx) bindPendingTargetMV(ref lRef, head, rest string) lRef {
	if ref.Kind == rTarget && ref.N < 0 && head == "Targeted" && rest == "CardManaCost" {
		if slot, ok := lc.pendingTargetSlot(); ok {
			ref.N = slot
			lc.notePendingTargetMV(slot)
		}
	}
	return ref
}

// targetMVReadableOrPending extends targetMVReadable with the pending slot:
// the effect has not declared it yet; checkPendingTargetMV proves it declares
// exactly that slot.
func (lc *lowerCtx) targetMVReadableOrPending(ref lRef) bool {
	return lc.pendingMVRef(ref) || lc.targetMVReadable(ref)
}

// costHandMV lowers an activated ability's hand cost. A filtered Discard part
// may read the effect's own single target (Kozilek, the Great Distortion's
// cmcEQX), so the node is armed for it around the cost.
func (lc *lowerCtx) costHandMV(sa *cards.SA, c string) (lCost, error) {
	saved := lc.mvNode
	defer func() { lc.mvNode = saved }()
	if strings.Contains(c, "Discard<") {
		lc.mvNode = sa
	}
	return lc.costHand(c)
}

// pendingMVRef reports whether ref is the pending read, still before the
// effect declared its slot. Once Targets grew, an ordinary read owns the slot
// and the normal targetMVReadable check applies.
func (lc *lowerCtx) pendingMVRef(ref lRef) bool {
	p := lc.pendMV
	return p != nil && p.ab == lc.ab && ref.Kind == rTarget && ref.N == p.slot && len(lc.ab.Targets) == p.base
}

// checkPendingTargetMV validates a pending read after node's effect: the effect
// appended exactly one slot (the pending one) and the kernel's targetMV reads
// that kind of slot. A pending read with no such slot refuses the card.
func (lc *lowerCtx) checkPendingTargetMV(node *cards.SA) error {
	p := lc.pendMV
	if p == nil || p.ab != lc.ab || p.node != node {
		return nil
	}
	lc.pendMV = nil
	if p.slot != p.base || len(lc.ab.Targets) != p.base+1 {
		return refuse("Targeted$CardManaCost before an effect that did not declare its single target")
	}
	if lc.ab.Targets[p.slot].AtRes {
		return refuse("Targeted$CardManaCost of a target asked mid-resolution")
	}
	if !lc.targetMVReadable(lRef{Kind: rTarget, N: p.slot}) {
		return refuse("Targeted$CardManaCost of a target slot the kernel's targetMV does not read")
	}
	return nil
}
