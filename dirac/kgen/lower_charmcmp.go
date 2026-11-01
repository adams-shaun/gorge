package kgen

import (
	"strings"
)

// charmDynNum reads a spell charm's CharmNum$ written inline as
// Count$Compare <Y> <op><n>.<a>.<b> with constant arms ("choose one -- if you
// descended this turn, choose both instead": Molten Collapse's
// CharmNum$ Count$Compare Y GE1.2.1; Wail of the Forgotten's GE8.3.1).
// gorge CharmModeBounds (effects/charm.go) evaluates CharmNum$ at each ask
// with the caster's ctx and clamps the maximum to the number of modes; the
// offer gate reads only the minimum. MinCharmNum$ must be the literal
// smaller arm, so the minimum never moves. It returns the upper bound
// (clamped to nModes) with ok = true and records the lowered count in
// CardDef.ModesPickV; any other CharmNum$ returns ok = false and is left to
// teamworkCharmNum/charmCounts.
func (lc *lowerCtx) charmDynNum(ps *params, nModes int) (pick int, ok bool, err error) {
	v, has := ps.m["CharmNum"]
	if !has {
		return 0, false, nil
	}
	body := strings.TrimSpace(v)
	if !strings.HasPrefix(body, "Count$Compare ") && !strings.HasPrefix(body, "Count$Delirium.") {
		return 0, false, nil
	}
	if lc.ab.Kind != akSpell {
		return 0, false, refuse("dynamic charm count on a %s ability", abilityKindNames[lc.ab.Kind])
	}
	if lc.out.ModesPickV.Kind != vNone {
		return 0, false, refuse("two dynamic charm counts")
	}
	val, err := lc.svarValue(body, 0)
	if err != nil {
		return 0, false, err
	}
	arms := val.Sub
	switch val.Kind {
	case vCountAtLeast:
	case vCompare:
		arms = nil
		if len(val.Sub) == 2 && val.Sub[1].Kind == vBranches {
			arms = val.Sub[1].Sub
		}
	default:
		arms = nil
	}
	if len(arms) != 2 {
		return 0, false, refuse("charm count CharmNum$ %q", v)
	}
	var n [2]int
	for i, a := range arms {
		if a.Kind != vConst || a.Mul != 0 || a.Div != 0 || a.Add != 0 || a.Neg || a.N < 1 || a.N > 4 {
			return 0, false, refuse("charm count CharmNum$ %q arm", v)
		}
		n[i] = a.N
	}
	lo, hi := min(n[0], n[1]), max(n[0], n[1])
	mv, has := ps.m["MinCharmNum"]
	if mv = strings.TrimSpace(mv); !has || len(mv) != 1 || int(mv[0]-'0') != lo {
		return 0, false, refuse("charm count CharmNum$ %q with MinCharmNum$ %q", v, mv)
	}
	lc.out.ModesPickV = val
	return min(hi, nModes), true, nil
}
