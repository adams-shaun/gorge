package kgen

import "strings"

// an Effect CantBlockBy carrying no ExileOnMoved$
// is safe to lower only when its remembered object is its own ATTACKING
// target (Gossip's Talent's level-2 trigger). gorge remembers the target's
// object id and matches Card.IsRemembered, so the grant ends at the target's
// next zone change (a new id every move); the target is already attacking,
// and the kernel has no extra combat, so the only window the "can't be
// blocked" grant can matter in is this combat's declare-blockers step.
//
// cantBlockAttackingTarget recognizes exactly that shape. The caller keeps
// the existing ExileOnMoved$ Battlefield accept and refuses everything else
// as "Effect CantBlockBy shape".
func (lc *lowerCtx) cantBlockAttackingTarget(ps *params, obj lRef) bool {
	if obj.Kind != rTarget || ps.str("RememberObjects") != "Targeted" {
		return false
	}
	if _, ok := ps.m["ExileOnMoved"]; ok {
		return false
	}
	if _, ok := ps.m["Duration"]; ok {
		return false
	}
	return requiresAttackingTarget(ps.str("ValidTgts"))
}

// requiresAttackingTarget: every comma alternative of a ValidTgts$ string
// must name the attacking predicate (as a dot or plus atom). Gossip's
// "Creature.attacking+powerLE3" qualifies; a plain "Creature.powerLE3" and a
// player alternative do not. An empty string matches nothing and is false.
func requiresAttackingTarget(validTgts string) bool {
	if strings.TrimSpace(validTgts) == "" {
		return false
	}
	for _, alt := range strings.Split(validTgts, ",") {
		found := false
		for _, part := range strings.Split(alt, "+") {
			for _, atom := range strings.Split(part, ".") {
				if strings.TrimSpace(atom) == "attacking" {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// cantBlockTargetedUntilEOT: the Effect's remembered object is its own target
// (RememberObjects$ Targeted), with neither ExileOnMoved$ nor Duration$, so the
// grant lasts the turn and lapses when the target changes zone (a new object
// id). It is the haste or power-compare form of ValidBlocker$ (Speed, Young Avenger;
// Bright-Palm, Soul Awakener, Ghirapur Guide, Run for Your Life).
func (lc *lowerCtx) cantBlockTargetedUntilEOT(ps *params, obj lRef) bool {
	if obj.Kind != rTarget || ps.str("RememberObjects") != "Targeted" {
		return false
	}
	if _, ok := ps.m["ExileOnMoved"]; ok {
		return false
	}
	_, hasDur := ps.m["Duration"]
	return !hasDur
}
