package kgen

import (
	"reflect"
	"slices"
	"strings"
)

// targetedOtherSpec splits a PumpAll ValidCards$ of the Forge shape
// Targeted.Other+<props> (AbilityUtils filterListByType rewrites the base to
// Card and makes the first targeted card the source, so Other is "other than
// the target"; gorge sameNameContextBase) into the ordinary filter spelling of
// the same cards with the target included: a Creature prop becomes the head,
// Other drops out, YouCtrl is kept. ok is false for any other spelling.
func targetedOtherSpec(vc string) (spec string, ok bool) {
	base, props, _ := strings.Cut(strings.TrimSpace(vc), ".")
	if base != "Targeted" || strings.Contains(vc, ",") {
		return "", false
	}
	ps := strings.Split(props, "+")
	at := slices.Index(ps, "Other")
	if at < 0 {
		return "", false
	}
	ps = slices.Delete(ps, at, at+1)
	for _, p := range ps {
		if p != "YouCtrl" && p != "Creature" {
			return "", false // sameName and the rest keep their own refusals
		}
	}
	head := "Card"
	if i := slices.Index(ps, "Creature"); i >= 0 {
		head = "Creature"
		ps = slices.Delete(ps, i, i+1)
	}
	if len(ps) == 0 {
		return head, true
	}
	return head + "." + strings.Join(ps, "+"), true
}

// pumpAllOtherThanTarget lowers `Pump target creature +a/+b, then PumpAll each
// OTHER creature (than the target) matching F +c/+d` (Phalanx Tactics, Sigil
// Blessing) without a target-relative filter flag. The kernel has none, but
// the target is a creature the PumpAll filter admits (its own slot filter IS
// F), so the two are one PumpAll over F, target included, for +c/+d, plus the
// target alone for the difference +(a-c)/+(b-d): every pump is a layer-7c
// addition until end of turn, so the sum on each creature is the same, and a
// target that is illegal on resolution fizzles the whole spell (CR 608.2b)
// before either half. Anything else refuses by name.
func (lc *lowerCtx) pumpAllOtherThanTarget(ps *params, spec string) error {
	if _, ok := ps.get("ValidCards"); !ok {
		return refuse("PumpAll ValidCards$ Targeted.Other without ValidCards$")
	}
	// the first 60 characters are the refusal class (classes.go): keep them fixed
	const shape = "PumpAll Targeted.Other beyond a constant pump of the one target creature"
	t := lc.lastTarget()
	if len(lc.chainT) != 1 || t < 0 || len(lc.ab.Prog) == 0 {
		return refuse("%s (target slots %d)", shape, len(lc.chainT))
	}
	tg := lc.ab.Targets[t]
	if !tg.Min.isConst() || !tg.Max.isConst() || tg.Min.N != 1 || tg.Max.N != 1 {
		return refuse("%s (not exactly one target)", shape)
	}
	f, err := lc.filter(spec)
	if err != nil {
		return err
	}
	if tg.F == nil || !reflect.DeepEqual(tg.F, f) {
		return refuse("%s (the target filter is not the PumpAll filter)", shape)
	}
	prev := &lc.ab.Prog[len(lc.ab.Prog)-1]
	if prev.Op != "PumpUntilEOT" || prev.Obj != (lRef{Kind: rTarget, N: t}) ||
		!prev.A.isConst() || !prev.B.isConst() || prev.CS != "" || prev.F != nil || prev.Cond.Kind != cNone {
		return refuse("%s (the node before is not the target's plain pump)", shape)
	}
	att, def := cst(0), cst(0)
	if a, ok := ps.get("NumAtt"); ok {
		if att, err = lc.value(a); err != nil {
			return err
		}
	}
	if d, ok := ps.get("NumDef"); ok {
		if def, err = lc.value(d); err != nil {
			return err
		}
	}
	if !att.isConst() || !def.isConst() || att.N < 0 || def.N < 0 || prev.A.N < att.N || prev.B.N < def.N {
		return refuse("%s (the others' pump exceeds the target's)", shape)
	}
	prev.A, prev.B = cst(prev.A.N-att.N), cst(prev.B.N-def.N)
	prev.Note = "plus the PumpAll below: the target is one of its creatures"
	lc.emit(lInstr{Op: "PumpAll", F: f, A: att, B: def, Note: "the target too; its own pump above is the difference"})
	return nil
}
