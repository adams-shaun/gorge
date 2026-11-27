package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

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

// thisTargetedPlayer lowers Defined$ ThisTargetedPlayer: the player target
// the node declaring it chose (gorge's getDefinedPlayers "ThisTargetedPlayer"
// reads the resolving link's own target). A sub-link with no target of its
// own (a charm mode's body) reads the one player target the enclosing charm
// mode declared, the same mode-scoped slot walk the hand-move and
// Play-TargetedPlayer lowerings use.
func (lc *lowerCtx) thisTargetedPlayer() (lRef, error) {
	if t := lc.ownTarget(); t >= 0 && playerTargetOnly(lc.ab.Targets[t]) {
		return lRef{Kind: rTarget, N: t}, nil
	}
	slot, n := -1, 0
	for i, t := range lc.ab.Targets {
		if lc.mode >= 0 && t.Mode != lc.mode {
			continue
		}
		n++
		if playerTargetOnly(t) {
			if slot >= 0 {
				return lRef{}, refuse("Defined$ ThisTargetedPlayer over two player targets")
			}
			slot = i
		}
	}
	if slot < 0 || n != 1 {
		return lRef{}, refuse("Defined$ ThisTargetedPlayer without one player target")
	}
	return lRef{Kind: rTarget, N: slot}, nil
}

// playerTargetOnly reports whether a target declaration names players only
// (no permanent alternative), which a player-writing instruction can read.
func playerTargetOnly(t lTarget) bool {
	return t.F != nil && t.F.Players != 0 && len(t.F.Clauses) == 0
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

// definedParentTargetedController lowers Defined$ ParentTargetedController:
// Forge's getDefinedPlayers answer, the controllers of the NEAREST earlier
// link's targets (effects context.go controllersOf(parentLinkTargets)), read
// live: an object that left the battlefield answers its owner, not the
// controller it had when targeted (Unlicensed Disintegration deals its damage
// after the parent destroyed the creature; Season of the Burrow draws after
// the parent exiled it).
func (lc *lowerCtx) definedParentTargetedController(v string) (lRef, error) {
	t := lc.parentLastTarget()
	if t < 0 {
		return lRef{}, noDefinedTargetSlot(v)
	}
	if t != 0 {
		return lRef{}, refuse("Defined$ %s over target slot %d", v, t)
	}
	// A stack-object target that a parent counter removes has no recoverable
	// live owner in the kernel (the stack entry is gone), so this ref is exact
	// only over permanent/card targets.
	if pt := lc.ab.Targets[t]; pt.Spell || pt.StackAb || pt.Zone == zStack {
		return lRef{}, refuse("Defined$ %s over a stack-object target", v)
	}
	return lRef{Kind: rTargetCtrlLive}, nil
}

// definedSourceController lowers Defined$ SourceController: the resolving
// source's controller. A spell's source controller is its caster, and a
// delayed trigger's is the caster that registered it, which is the entry's
// controller (rYou); a permanent's ability reads the source's LIVE controller,
// which a control change between activation and resolution can move, so those
// stay refused rather than guessing the activator.
func (lc *lowerCtx) definedSourceController() (lRef, error) {
	if lc.ab == nil || (lc.ab.Kind != akSpell && lc.ab.Kind != akLatent) {
		kind := "no ability"
		if lc.ab != nil {
			kind = abilityKindNames[lc.ab.Kind]
		}
		return lRef{}, refuse("Defined$ SourceController on a %s ability", kind)
	}
	return lRef{Kind: rYou}, nil
}

// tokenOwnerTargetRiders checks the target riders on a Token SA that declares
// its own player ValidTgts$ for TokenOwner$. TargetUnique$ True is not a
// no-op: it is the "another target player" restriction (Cybernetica
// Datasmith), so it rides the slot as Unique (gorge's target_unique_chain
// excludes every earlier pick); any other rider would make the owner
// ambiguous.
func tokenOwnerTargetRiders(ps *params, o string) (unique bool, err error) {
	for _, k := range []string{"TargetMin", "TargetMax", "TargetUnique", "TgtZone"} {
		if v, ok := ps.get(k); ok {
			if k == "TargetUnique" && strings.EqualFold(strings.TrimSpace(v), "True") {
				unique = true
				continue
			}
			return false, refuse("Token TokenOwner$ %q with %s$", o, k)
		}
	}
	if _, ok := ps.get("TargetUnique"); ok {
		ps.consume("TargetUnique")
	}
	return unique, nil
}
