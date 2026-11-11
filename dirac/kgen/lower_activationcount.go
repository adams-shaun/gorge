package kgen

import "strings"

// activationLimitCondition lowers a node's ConditionActivationLimit$ <op><n>
// ("if this ability has been activated four or more times this turn": Dragon
// Whelp, Nalathni Dragon) to a compare of kop.ExActivations, the resolving
// ability's source's non-mana activations this turn (gorge
// effects/conditions.go: Ctx.ActivationsThisTurn, the ability's own count,
// including the activation that is resolving).
//
// The kernel keeps the count per object (object.acts), not per ability, so
// the shape lowers only where the two agree: a printed battlefield activated
// ability that is the card's one non-mana activated ability, which the
// face's keywords (Equip, Crew, ...) would add to. Any other Condition*$ key
// beside it refuses, as gorge reads the key alone.
func (lc *lowerCtx) activationLimitCondition(ps *params, cmp string) (lCond, error) {
	if beside := conditionKeysBesides(ps, "ConditionActivationLimit"); len(beside) > 0 {
		return lCond{}, refuse("ConditionActivationLimit$ beside %s$", beside[0])
	}
	if lc.ab == nil || lc.ab.Kind != akActivated || lc.ab.Latent || lc.ab.Zone != 0 && lc.ab.Zone != zBattlefield {
		return lCond{}, refuse("ConditionActivationLimit$ outside a printed battlefield activated ability")
	}
	n := 0
	for _, sa := range lc.face.Abilities {
		if sa != nil && sa.Kind == "AB" && sa.API != "Mana" && sa.API != "ManaReflected" {
			n++
		}
	}
	if n != 1 {
		return lCond{}, refuse("ConditionActivationLimit$ on a card with %d non-mana activated abilities (the kernel counts activations per object)", n)
	}
	cmp = strings.TrimSpace(cmp)
	op, b, err := lc.cmpSpec(cmp)
	if err != nil {
		return lCond{}, err
	}
	if !b.isConst() {
		return lCond{}, refuse("ConditionActivationLimit$ %q: a computed limit", cmp)
	}
	return lCond{Kind: cValue, A: lValue{Kind: vActivations}, Op: op, B: b}, nil
}
