package kgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// cold lowering passes for the value heads that read an
// object or a number the kernel already tracks. The hot value switch stays in
// value.go; the structural checks that make a read exact live here.

// rememberPowerRead is how an lValue of vPowerOf over rRemembered prints
// (lValue.String / refNames): the read refuseRememberedPower guards.
const rememberPowerRead = "PowerOf(Remembered)"

// refuseRememberedPower guards Remembered$CardPower (gorge
// engine/effects/count_ref_property.go:165-177): gorge SUMS the power over
// the remembered set, while the kernel's ExPowerOf over rRemembered yields
// ONE object (refPT -> refObjCtx). The read lowers only when the set is
// provably one card: the ability's only remember writer is a ChangeZone
// (Return/Bounce RememberChanged$) of exactly one target, the write precedes
// the read, and no other ability's program, trigger condition, static or
// replacement writes the set (gorge's list also persists on the source).
func (lc *lowerCtx) refuseRememberedPower() {
	const shape = "Remembered$CardPower over a set of more than one"
	reads, readAb := 0, -1
	for ai := range lc.out.Abilities {
		a := lc.out.Abilities[ai]
		var held []string
		if a.Trigger != nil {
			t := a.Trigger
			held = append(held, fmt.Sprint(t.F), fmt.Sprint(t.F2), fmt.Sprint(t.Cond))
		}
		held = append(held, fmt.Sprint(a.ActCond))
		for _, t := range a.Targets {
			held = append(held, fmt.Sprint(t.F))
		}
		for _, s := range held {
			if strings.Contains(s, rememberPowerRead) {
				lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "%s (a read outside the program)", shape)
				return
			}
		}
		for _, in := range a.Prog {
			if strings.Contains(in.String(), rememberPowerRead) {
				reads++
				readAb = ai
			}
		}
	}
	if reads == 0 {
		return
	}
	if reads > 1 || readAb < 0 {
		lc.refuseNode("card", "%s (more than one read)", shape)
		return
	}
	for _, s := range lc.out.Statics {
		if strings.Contains(fmt.Sprint(s.F), rememberPowerRead) || strings.Contains(fmt.Sprint(s.Cond), rememberPowerRead) {
			lc.refuseNode("card", "%s (a static read)", shape)
			return
		}
	}
	for _, r := range lc.out.Repls {
		for _, in := range r.Prog {
			if strings.Contains(in.String(), rememberPowerRead) || kremWrites(in) {
				lc.refuseNode("card", "%s (a replacement)", shape)
				return
			}
		}
	}
	// exactly one remember writer, in the reading ability, before the read
	writerAt := -1
	for ai := range lc.out.Abilities {
		for i, in := range lc.out.Abilities[ai].Prog {
			if !kremWrites(in) {
				continue
			}
			if ai != readAb || writerAt >= 0 {
				lc.refuseNode("card", "%s (another remember writer)", shape)
				return
			}
			writerAt = i
		}
	}
	a := lc.out.Abilities[readAb]
	if writerAt < 0 {
		lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "%s (no remember writer)", shape)
		return
	}
	readAt := -1
	for i, in := range a.Prog {
		if strings.Contains(in.String(), rememberPowerRead) {
			readAt = i
		}
	}
	if readAt < writerAt {
		lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "%s (the write follows the read)", shape)
		return
	}
	in := a.Prog[writerAt]
	// a ChangeZone RememberChanged$ of one target: Return/Bounce with the
	// MvRemember flag and a target slot bounded to exactly one.
	if (in.Op != "Return" && in.Op != "Bounce") || !strings.Contains(in.CS, "Remember") || in.Obj.Kind != rTarget ||
		in.Obj.N < 0 || in.Obj.N >= len(a.Targets) {
		lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "%s (the writer is not a one-card ChangeZone)", shape)
		return
	}
	tg := a.Targets[in.Obj.N]
	if !isConstN(tg.Min, 1) || !isConstN(tg.Max, 1) {
		lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "%s (the writer is not a one-card ChangeZone)", shape)
		return
	}
}

// trigRemSacPower marks a reflexive latent whose body read
// Spawner>Sacrificed$CardPower: the parent's IfPaid Sacrifice sum is captured
// into the ReflexiveTrigger's B (value_ktrigrem.go). One captured amount.
const trigRemSacPower = "sacPower"

// ifPaidSacrificeIndex is the index of the trigger-body IfPaid Sacrifice
// whose "If you do" chain is being lowered, or -1. Every instruction of the
// chain is emitted after the wrapper, so a value read anywhere in the body
// (the Draw itself or a later SubAbility's filter, CR 608.2h) sees exactly
// one such wrapper in the program so far; two refuse (the read could belong
// to either capture).
func (lc *lowerCtx) ifPaidSacrificeIndex() int {
	at := -1
	for i := range lc.ab.Prog {
		in := &lc.ab.Prog[i]
		if in.Op == "IfPaid" && in.CS == "Sacrifice" {
			if at >= 0 {
				return -1
			}
			at = i
		}
	}
	return at
}

// ifPaidSacrificeNote marks the parent's IfPaid Sacrifice instruction with
// the characteristic the reflexive body reads (B = 1 + the SacSum bit in
// build_ab.go, consumed by kernel sacRemember). Exactly one such writer is
// admitted, and it must be the last IfPaid Sacrifice in the program.
func (lc *lowerCtx) markParentIfPaidSacrifice(suffix string) error {
	pr := lc.ab.Prog
	at := -1
	for i := range pr {
		if pr[i].Op == "IfPaid" && pr[i].CS == "Sacrifice" {
			if at >= 0 {
				return refuse("Spawner>Sacrificed$ beside two IfPaid Sacrifice costs")
			}
			at = i
		}
	}
	if at < 0 {
		return refuse("Spawner>Sacrificed$ without a parent IfPaid Sacrifice cost")
	}
	pr[at].Note = noteRememberSacrificed + suffix
	return nil
}

// opPrime is vCompare's "Sub[0] is prime" (the cmp ops of
// filter.go are 1..6).
const opPrime uint8 = 64

func init() {
	cmpOps[opPrime] = kop.CmpPrime
	exprKinds[vUnlockedDoors] = kop.ExUnlockedDoors
}

// headArm reads one arm of a Count$ head: an integer or a face SVar.
func (lc *lowerCtx) headArm(head, t string, depth int) (lValue, error) {
	if n, err := strconv.Atoi(t); err == nil {
		return cst(n), nil
	}
	sb, ok := lc.face.SVars[t]
	if !ok {
		return lValue{}, refuse("Count$%s arm %q", head, t)
	}
	return lc.svarValue(sb, depth+1)
}

// countHeads8 lowers the Count$ heads of (gorge
// count_body_heads.go InOwnMainPhase, count_zone_fold.go ValidLibrary,
// count_body_simple.go IsPrime, unlockdoor.go countUnlockedDoors); ok is
// false for any other head.
func (lc *lowerCtx) countHeads8(rest string, depth int) (v lValue, ok bool, err error) {
	switch {
	case strings.HasPrefix(rest, "InOwnMainPhase."):
		parts := strings.Split(strings.TrimPrefix(rest, "InOwnMainPhase."), ".")
		if len(parts) != 2 {
			return lValue{}, true, refuse("Count$%s", rest)
		}
		var arms [2]lValue
		for i, t := range parts {
			if arms[i], err = lc.headArm("InOwnMainPhase", t, depth); err != nil {
				return lValue{}, true, err
			}
		}
		return lValue{Kind: vCompare, N: 1, Op: opGE, Sub: []lValue{{Kind: vYourTurns, N: 2}, {Kind: vBranches, Sub: arms[:]}}}, true, nil
	case strings.HasPrefix(rest, "ValidLibrary "):
		spec := strings.TrimPrefix(rest, "ValidLibrary ")
		if strings.ContainsAny(spec, "$,") {
			return lValue{}, true, refuse("Count$ValidLibrary %q", spec)
		}
		f, err := lc.filter(spec)
		if err != nil {
			return lValue{}, true, err
		}
		return lValue{Kind: vCountHand, N: 1, F: f}, true, nil
	case strings.HasPrefix(rest, "IsPrime "):
		parts := strings.Split(strings.TrimPrefix(rest, "IsPrime "), ".")
		if len(parts) != 3 {
			return lValue{}, true, refuse("Count$%s", rest)
		}
		sb, has := lc.face.SVars[parts[0]]
		if !has {
			return lValue{}, true, refuse("Count$IsPrime operand %q", parts[0])
		}
		val, err := lc.svarValue(sb, depth+1)
		if err != nil {
			return lValue{}, true, err
		}
		var arms [2]lValue
		for i, t := range parts[1:] {
			if arms[i], err = lc.headArm("IsPrime", t, depth); err != nil {
				return lValue{}, true, err
			}
		}
		return lValue{Kind: vCompare, N: 0, Op: opPrime, Sub: []lValue{val, {Kind: vBranches, Sub: arms[:]}}}, true, nil
	case rest == "UnlockedDoors":
		return lValue{Kind: vUnlockedDoors}, true, nil
	}
	return lValue{}, false, nil
}
