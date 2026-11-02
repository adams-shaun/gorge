package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// gorge reads a resolving SA's own targets for
// Defined$ ThisTargetedCard and for a value's Targeted$ (c.Targets), never an
// earlier node's slot, so lastTarget() alone cannot stand in for the node's
// own slot.
type ownTgt struct {
	node *cards.SA
	ab   *lAbility
	slot int
}

// noteOwnTarget is called by addTarget for every slot it declares.
func (lc *lowerCtx) noteOwnTarget(slot int) {
	lc.own = ownTgt{node: lc.curNode, ab: lc.ab, slot: slot}
}

// ownTarget returns the slot the current node declared, or -1.
func (lc *lowerCtx) ownTarget() int {
	if lc.curNode == nil || lc.own.node != lc.curNode || lc.own.ab != lc.ab {
		return -1
	}
	return lc.own.slot
}

// thisTargetedCard lowers Defined$ ThisTargetedCard: the node's own slot.
func (lc *lowerCtx) thisTargetedCard() (lRef, error) {
	t := lc.ownTarget()
	if t < 0 {
		return lRef{}, refuse("Defined$ ThisTargetedCard without its own target slot")
	}
	return lRef{Kind: rTarget, N: t}, nil
}

// ownTargetFirst reports whether the node declares its own ValidTgts$, so a
// dependent read (NumDmg$, DamageSource$) must come after objOrTarget.
func ownTargetFirst(ps *params) bool {
	vt, ok := ps.get("ValidTgts")
	return ok && vt != ""
}

// parentChain is the chain's target slots without the current node's own
// slot: ParentTarget(ed) names an earlier link's targets (gorge
// parentLinkTargets), whichever order the node reads its params in.
func (lc *lowerCtx) parentChain() []int {
	c := lc.chainT
	if o := lc.ownTarget(); o >= 0 && len(c) > 0 && c[len(c)-1] == o {
		c = c[:len(c)-1]
	}
	return c
}

// parentLastTarget is lastTarget() over parentChain (-1: none).
func (lc *lowerCtx) parentLastTarget() int {
	c := lc.parentChain()
	if len(c) == 0 {
		return -1
	}
	return c[len(c)-1]
}

// parentFirstTarget is firstTarget() over parentChain (-1: none).
func (lc *lowerCtx) parentFirstTarget() int {
	c := lc.parentChain()
	if len(c) == 0 {
		return -1
	}
	return c[0]
}
