package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"strings"
)

func (lc *lowerCtx) charm(ps *params) error {
	choices := strings.Split(ps.str("Choices"), ",")
	if len(choices) < 2 {
		return refuse("charm with %d choices", len(choices))
	}
	if lc.mode >= 0 {
		return refuse("nested charm")
	}
	if lc.ab.Modes != 0 {
		return refuse("two charms in one ability")
	}
	// Defined$ names who chooses the mode (gorge CharmParams.Defined);
	// "You" is the controller, the kernel's only chooser.
	if d, ok := ps.get("Defined"); ok && d != "You" {
		return refuse("charm chooser Defined$ %q", d)
	}
	twPick, plainPick, err := lc.teamworkCharmNum(ps, len(choices))
	if err != nil {
		return err
	}
	if dyn, ok, err := lc.charmDynNum(ps, len(choices)); err != nil {
		return err
	} else if ok {
		plainPick = dyn // CharmNum$ over Count$Compare (lower_charmcmp.go)
	} else if lc.ab.Kind == akTriggered {
		if lc.out.ModesPickV.Kind != vNone {
			return refuse("a fixed triggered charm count beside a dynamic one")
		}
		lc.out.TrigCharmFixed = true
	}
	pick, minPick, err := charmCounts(ps, len(choices), plainPick)
	if err != nil {
		return err
	}
	if max(pick, twPick) > 1 && lc.ab.Kind != akSpell && lc.ab.Kind != akTriggered && !(lc.ab.Kind == akLatent && lc.reflexMulti) {
		return refuse("multi-pick charm on a %s ability", abilityKindNames[lc.ab.Kind])
	}
	if lo := lc.out.ModesMinCount; lo != 0 {
		minPick = min(lo, pick) // the offer gate's minimum (CardDef.ModesMinCount)
	}
	rp, err := lc.charmRepeatPlan(ps, choices, pick, twPick, lc.out.ModesPickV.Kind != vNone) // lower_charmrepeat.go
	if err != nil {
		return err
	}
	lc.ab.Modes = len(choices)
	lc.ab.ModesPick, lc.ab.ModesMin = pick, minPick
	if rp != nil {
		lc.ab.RepMax, lc.ab.RepCost, lc.ab.RepBudget = rp.max, rp.cost, rp.budget
	}
	rootTargeted := make([]bool, len(choices)) // gorge Targeted(): the mode root's ValidTgts$ is non-empty
	for i, name := range choices {
		sa := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(name))
		if sa == nil {
			return refuse("charm choice %q is not an ability", name)
		}
		rootTargeted[i] = strings.TrimSpace(sa.Params["ValidTgts"]) != ""
		lc.mode = i
		mps, err := lc.modeCost(sa, i) // lower_modecost.go
		if err != nil {
			return err
		}
		c := i
		if rp != nil {
			if mps == nil {
				mps = newParams(sa.Params)
			}
			mps.consume("Pawprint") // the mode's price, read by charmRepeatPlan
			c = rp.c(i, 0)
		}
		saved := lc.chainT
		lc.chainT = append([]int(nil), lc.chainT...)
		at := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "Charm", C: c})
		if err := lc.chain(sa, mps); err != nil {
			return err
		}
		lc.ab.Prog[at].Skip = len(lc.ab.Prog) - at - 1
		if rp != nil {
			rp.replicate(lc, at, i)
		}
		lc.chainT = saved
	}
	lc.mode = -1
	if err := lc.charmRestrict(ps); err != nil {
		return err
	}
	if pick > 1 && len(choices) > 4 {
		return refuse("charm with more than 4 modes") // mode bits 0..3 only
	}
	if pick > len(choices) && rp == nil {
		// gorge clamps the maximum to the eligible modes; no carrier reads it
		return refuse("charm count above its modes (%d of %d)", pick, len(choices))
	}
	if max(pick, twPick) > 1 {
		if err := lc.charmMultiTargets(len(choices), rootTargeted); err != nil {
			return err
		}
		if rp != nil {
			return lc.charmRepeatTargets(rp, len(choices))
		}
	} else if lc.out.Entwine != nil {
		return lc.entwineTargets(len(choices), rootTargeted) // lower_entwine.go
	}
	return nil
}

// teamworkCharmNum reads a Teamwork card's CharmNum$ over an SVar whose
// body is Count$Teamwork.<a>.<b> (single digits >= 1; CR 702.194b: "choose
// both" when teamworked): <a> modes teamworked (CardDef.ModesPickTW), <b>
// otherwise. Any other non-literal CharmNum$ is left to charmCounts, which
// refuses it. It returns 0, 0 for a literal count.
func (lc *lowerCtx) teamworkCharmNum(ps *params, nModes int) (tw, plain int, err error) {
	v, ok := ps.m["CharmNum"]
	if !ok {
		return 0, 0, nil
	}
	body, ok := lc.face.SVars[strings.TrimSpace(v)]
	rest, isTW := strings.CutPrefix(strings.TrimSpace(body), "Count$Teamwork.")
	if !ok || !isTW {
		return 0, 0, nil
	}
	if lc.out.Teamwork == 0 {
		return 0, 0, refuse("charm count Count$Teamwork without Teamwork")
	}
	if _, has := ps.m["MinCharmNum"]; has {
		return 0, 0, refuse("teamwork charm count beside MinCharmNum$")
	}
	if lc.out.ModesPickTW != 0 {
		return 0, 0, refuse("two teamwork charms")
	}
	arms := strings.Split(rest, ".")
	if len(arms) != 2 {
		return 0, 0, refuse("teamwork charm count %q", rest)
	}
	var n [2]int
	for i, a := range arms {
		if len(a) != 1 || a[0] < '1' || a[0] > '9' {
			return 0, 0, refuse("teamwork charm count arm %q", a)
		}
		n[i] = int(a[0] - '0')
	}
	if n[0] > nModes || n[0] > 4 {
		return 0, 0, refuse("teamwork charm count %d of %d modes", n[0], nModes)
	}
	lc.out.ModesPickTW = n[0]
	return n[0], n[1], nil
}

// charmCounts reads CharmNum$/MinCharmNum$ (gorge CharmModeBounds): literal
// single digits only; the maximum defaults to 1 and the minimum to the
// maximum. AdditionalDescription$ is display text.
func charmCounts(ps *params, nModes int, num int) (pick, minPick int, err error) {
	ps.consume("AdditionalDescription")
	pick, minPick = 1, -1
	if num > 0 {
		// a Teamwork card's Count$Teamwork.<a>.<b>: <b> is the plain count
		// (lc.teamworkCharmNum read and checked it)
		ps.consume("CharmNum")
		pick = num
	}
	lit := func(key string) (int, bool, error) {
		v, ok := ps.get(key)
		if !ok {
			return 0, false, nil
		}
		v = strings.TrimSpace(v)
		if len(v) != 1 || v[0] < '0' || v[0] > '9' {
			return 0, false, refuse("charm count %s$ %q", key, v)
		}
		return int(v[0] - '0'), true, nil
	}
	if num == 0 {
		if n, ok, err := lit("CharmNum"); err != nil {
			return 0, 0, err
		} else if ok {
			pick = n
		}
	}
	if m, ok, err := lit("MinCharmNum"); err != nil {
		return 0, 0, err
	} else if ok {
		minPick = m
	}
	if pick < 1 {
		return 0, 0, refuse("charm count CharmNum$ %d", pick)
	}
	if minPick < 0 {
		minPick = pick
	}
	if minPick > pick {
		return 0, 0, refuse("charm count MinCharmNum$ %d above CharmNum$ %d", minPick, pick)
	}
	// pick 1, min 0: "choose up to one" (Hullbreaker Horror, Ertai
	// Resurrected), the kernel's multi-pick mode step
	return pick, minPick, nil
}

// charmMultiTargets refuses the target shapes of a multi-pick charm the
// kernel's combined target step does not read (gorge target_modes.go
// askCharmModeTargets: one KTarget when every slot is 1..1; a slot outside
// 1..1 takes the sequential askCharmSeqSlot, and TargetUnique another path).
// A mode's first declaration must be its root's own (rootTargeted, gorge
// Targeted()); later ones are chain links, announced after the combined ask
// (castSubAskLinks, target_unique_chain.go), which only a cast spell does.
func (lc *lowerCtx) charmMultiTargets(nModes int, rootTargeted []bool) error {
	for _, t := range lc.ab.Targets {
		if t.Mode < 0 {
			return refuse("multi-pick charm with a non-mode target")
		}
		if t.Mode >= nModes {
			return refuse("multi-pick charm with a target outside its modes")
		}
	}
	plural, chained := false, false
	for m := 0; m < nModes; m++ {
		n := 0
		for _, t := range lc.ab.Targets {
			if t.Mode != m {
				continue
			}
			n++
			if t.Unique {
				return refuse("multi-pick charm with TargetUnique")
			}
			if t.SameCtrl || t.DiffCtrl || t.SharedCrType || t.DistinctName || t.MaxMV != 0 || t.Divided != 0 || t.ParentPlus1 != 0 || t.AtRes ||
				!t.Min.isConst() || t.Min.N < 1 || t.Min.N > 2 || !t.Max.isConst() || t.Max.N < t.Min.N || t.Max.N > kop.MaxTargets {
				return refuse("multi-pick charm mode target bounds %s..%s", t.Min, t.Max)
			}
			// a slot of more than one target takes gorge's sequential per-mode asks
			// (askCharmSeqSlot), which the kernel derives from the slot
			// bounds (modal_charmseq.go); the chain links' place among
			// those asks is not read
			plural = plural || t.Max.N > 1
		}
		if n > 0 && !rootTargeted[m] {
			return refuse("multi-pick charm mode whose root declares no target")
		}
		if n > 1 && lc.ab.Kind != akSpell {
			return refuse("multi-pick charm mode chain targets on a triggered ability")
		}
		chained = chained || n > 1
	}
	if plural && chained {
		return refuse("multi-pick charm 1..N target slot beside a mode's chain targets")
	}
	return nil
}
