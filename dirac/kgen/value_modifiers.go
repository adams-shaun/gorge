package kgen

import (
	"fmt"
	"strings"
)

// rememberedModSplit splits a Remembered$ body's trailing arithmetic
// modifier. gorge's evalRememberedOK (engine/effects/count_ref.go) cuts the
// body at the FIRST "/" and hands the whole remaining suffix to applyCountOp
// as ONE op name; a chain "Plus.1/Times.2" arrives as the single string
// "Plus.1/Times.2", whose Atoi("1/Times.2") fails and leaves the value
// unchanged. A chain is therefore refused here rather than lowered into
// arithmetic gorge never evaluates.
//
// Only Amount and CardManaCost take an arithmetic suffix (Amount through
// evalRememberedOK, CardManaCost through evalRefProperty); every other
// Remembered$ spelling has none, and a Valid filter may itself contain "/",
// so it is never split.
//
// The hand-written Amount/DivideEvenlyDown.2 token spelling lowers to
// vTokensCreated (not the remembered count), so it stays whole and keeps its
// meaning; it carries exactly one op and its chained form is refused above.
func rememberedModSplit(rest string) (body, mod string, split bool, err error) {
	var prop string
	switch {
	case strings.HasPrefix(rest, "Amount/"):
		prop = "Amount"
	case strings.HasPrefix(rest, "CardManaCost/"):
		prop = "CardManaCost"
	default:
		return rest, "", false, nil
	}
	tail := rest[len(prop)+1:]
	if tail == "" {
		return rest, "", false, nil
	}
	if strings.Contains(tail, "/") {
		return "", "", false, fmt.Errorf("Remembered$%s: chained modifiers (gorge applies one op)", rest)
	}
	if prop == "Amount" && tail == "DivideEvenlyDown.2" {
		return rest, "", false, nil
	}
	return prop, tail, true, nil
}

// kvModMultiply applies a multiplicative modifier k to v after v's own
// modifiers. The kernel evaluates ExSum's operands first and then the outer
// value's Mul/Div/Add/Neg (dirac/value_exprs.go), so a value already carrying Div
// or Add is wrapped in a sum with zero: (Y/7)*2 and (Y-1)*2 lower exactly,
// where one Expr could not carry the order. A value with Mul or Neg still
// refuses in the caller (its multiply would compose the wrong way).
// limitValue is gorge applyCountOp's LimitMin.N (op opGE: max(v, N)) and
// LimitMax.N (op opLE: min(v, N)) as ExCompare: v keeps its own value when
// it passes the bound, else N. Shared by the /LimitMin /LimitMax modifiers
// and FlipCoin's Amount$ clamp (lower_dice.go).
func limitValue(v lValue, op uint8, n int) lValue {
	return lValue{Kind: vCompare, N: n, Op: op, Sub: []lValue{v, {Kind: vBranches, Sub: []lValue{v, cst(n)}}}}
}

func kvModMultiply(v lValue, k int) lValue {
	if v.Mul == 0 && !v.Neg && (v.Div != 0 || v.Add != 0) {
		v = lValue{Kind: vSum, Sub: []lValue{v, cst(0)}}
	}
	v.Mul = k
	return v
}

// kvDiscoverTailRefused reports whether a program is the Discover shape whose
// free cast is followed by a board-affecting tail: OpDiscover (the exile/pick),
// OpCastFree over the remembered card, OpDiscover C=1 (the uncast card to hand),
// then anything except ClearRemembered.
//
// The kernel's OpCastFree only queues the cast; the rest of the resolution's
// program runs before the queue drains (turn_gen). gorge announces the cast
// (asking its targets/modes) and runs the tail only after that ask, so a tail
// that reads or changes the board diverges at the cast's ask. Hit the Mother
// Lode's DB$ Token tail (the Treasure count) is the corpus carrier; the
// standard Discover tail (hand + clear) is order-independent and stays
// accepted. Refuse rather than lower a card the kernel cannot copy exactly;
// remove this guard when kernel-discover-freecast-order lands.
func kvDiscoverTailRefused(p []lInstr) bool {
	for i := 0; i+2 < len(p); i++ {
		if p[i].Op != "Discover" || p[i].C != 0 ||
			p[i+1].Op != "CastFree" || p[i+1].Obj.Kind != rRemembered || p[i+1].C != zExile ||
			p[i+2].Op != "Discover" || p[i+2].C != 1 {
			continue
		}
		for j := i + 3; j < len(p); j++ {
			if p[j].Op != "ClearRemembered" {
				return true
			}
		}
	}
	return false
}
