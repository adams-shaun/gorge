package kgen

import (
	"strings"
)

// charmDynNum reads a spell charm's CharmNum$ written inline as
// Count$Compare <Y> <op><n>.<a>.<b>, Count$Delirium.<a>.<b> or
// Count$Hellbent.<a>.<b> with constant arms ("choose one -- if you
// descended this turn, choose both instead": Molten Collapse's
// CharmNum$ Count$Compare Y GE1.2.1; Wail of the Forgotten's GE8.3.1;
// Captain James T. Kirk's triggers' Count$Hellbent.3.1), the same bodies
// behind a bare SVar name (gorge numText resolves the named body at the
// ask), or the paid-additional-cost two-arm
// Count$OptionalGenericCostPaid.<paid>.<unpaid> (Pyrrhic Strike: an
// OptionalCost blight part chooses both instead, and with MinCharmNum$
// absent gorge defaults the minimum to the evaluated count itself -- the
// kernel's static need is the larger arm and the runtime cap stops the ask
// at the evaluated count). It also reads a general count beside
// MinCharmNum$ 0 ("choose up to V": Bumi's Lessons in the graveyard, the
// Ruinous-crew-shaped X paid, read by the permanent's own enters trigger):
// the minimum is static 0 and the runtime cap floors the count at 1 and
// clamps it to the modes (modal_charmcmp.go).
// gorge CharmModeBounds (effects/charm.go) evaluates CharmNum$ at each ask
// with the caster's ctx and clamps the maximum to the number of modes; the
// offer gate reads only the minimum. An explicit MinCharmNum$ on a
// two-constant-arm count must be the literal smaller arm, so the minimum
// never moves. It returns the upper bound (clamped to nModes) with
// ok = true and records the lowered count in CardDef.ModesPickV; any other
// CharmNum$ returns ok = false and is left to
// teamworkCharmNum/charmCounts.
func (lc *lowerCtx) charmDynNum(ps *params, nModes int) (pick int, ok bool, err error) {
	v, has := ps.m["CharmNum"]
	if !has {
		return 0, false, nil
	}
	body := strings.TrimSpace(v)
	if !strings.Contains(body, "$") {
		if b, ok := lc.face.SVars[body]; ok {
			body = strings.TrimSpace(b)
		}
	}
	if !strings.HasPrefix(body, "Count$") || strings.HasPrefix(body, "Count$Teamwork") {
		// Count$Teamwork.<a>.<b> is the Teamwork card's own count
		// (teamworkCharmNum); every other Count$ body is read here
		return 0, false, nil
	}
	if lc.ab.Kind != akSpell && (lc.ab.Kind != akTriggered || lc.ab.Latent) {
		return 0, false, refuse("dynamic charm count on a %s ability", abilityKindNames[lc.ab.Kind])
	}
	val, err := lc.svarValue(body, 0)
	if err != nil {
		return 0, false, err
	}
	if lc.out.ModesPickV.Kind != vNone && lc.out.ModesPickV.String() != val.String() {
		// one card-wide count: Captain James T. Kirk's two triggers share
		// their charm, so they share its count
		return 0, false, refuse("two dynamic charm counts")
	}
	if lc.out.TrigCharmFixed {
		return 0, false, refuse("a dynamic triggered charm count beside a fixed one")
	}
	arms := val.Sub
	switch val.Kind {
	case vCountAtLeast, vKicked:
	case vCompare:
		arms = nil
		if len(val.Sub) == 2 && val.Sub[1].Kind == vBranches {
			arms = val.Sub[1].Sub
		}
	default:
		arms = nil
	}
	mv, hasMin := ps.m["MinCharmNum"]
	mv = strings.TrimSpace(mv)
	if len(arms) == 2 {
		var n [2]int
		twoArm := true
		for i, a := range arms {
			if a.Kind != vConst || a.Mul != 0 || a.Div != 0 || a.Add != 0 || a.Neg || a.N < 1 || a.N > 4 {
				twoArm = false
				break
			}
			n[i] = a.N
		}
		if twoArm {
			lo, hi := min(n[0], n[1]), max(n[0], n[1])
			if hasMin && (len(mv) != 1 || int(mv[0]-'0') != lo) {
				return 0, false, refuse("charm count CharmNum$ %q with MinCharmNum$ %q", v, mv)
			}
			lc.out.ModesPickV = val
			return min(hi, nModes), true, nil
		}
	}
	// a general count: only "choose up to V" lowers (MinCharmNum$ 0; the
	// minimum is the static 0 and the cap carries the count)
	if !hasMin || mv != "0" {
		return 0, false, refuse("charm count CharmNum$ %q with MinCharmNum$ %q", v, mv)
	}
	if _, built := exprKinds[val.Kind]; !built {
		return 0, false, refuse("charm count CharmNum$ %q", v)
	}
	lc.out.ModesPickV = val
	return nModes, true, nil
}
