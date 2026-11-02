package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// "distribute X +1/+1 counters among any number of target
// creatures" with X the paid {X} of the spell (Elusive Otter's Grove's
// Bounty; gorge counters_put.go AnnouncedDivision, the division announced at
// cast over the chosen targets, each at least 1: CR 601.2d). CounterNum$,
// TargetMax$ and DividedAsYouChoose$ all name one SVar whose body is
// Count$xPaid, TargetMin$ 0, P1P1, on a spell whose own cost has {X}. The slot
// is 0..X targets (TgfBoundsV) with Divide = kop.DivMaxV: the kernel's total is
// the slot's evaluated maximum. Any other variable spelling refuses.
func (lc *lowerCtx) dividedCountersAnyX(ps *params) (bool, error) {
	name := strings.TrimSpace(ps.str("DividedAsYouChoose"))
	if name == "" || strings.TrimSpace(lc.face.SVars[name]) != "Count$xPaid" {
		return false, nil
	}
	if ps.str("CounterNum") != name || ps.str("TargetMax") != name || ps.str("TargetMin") != "0" || ps.str("CounterType") != "P1P1" {
		return true, refuse("divided PutCounter shape (CounterNum$ %q, TargetMax$ %q, TargetMin$ %q, CounterType$ %q)", ps.str("CounterNum"), ps.str("TargetMax"), ps.str("TargetMin"), ps.str("CounterType"))
	}
	if lc.ab == nil || lc.ab.Kind != akSpell || lc.ab.Latent || lc.depth != 1 || !lc.out.Cost.X {
		return true, refuse("divided PutCounter by X outside a spell with an {X} cost")
	}
	vt, ok := ps.get("ValidTgts")
	if !ok {
		return true, refuse("divided PutCounter without ValidTgts$")
	}
	for _, k := range []string{"TargetUnique", "TargetsWithSameController", "TargetsWithDifferentControllers", "TgtZone", "Defined", "Choices"} {
		if _, ok := ps.get(k); ok {
			return true, refuse("divided PutCounter with %s$", k)
		}
	}
	f, err := lc.filter(vt)
	if err != nil {
		return true, err
	}
	if f.Players != 0 {
		return true, refuse("divided PutCounter target filter %q names players", vt)
	}
	x, err := lc.value(name)
	if err != nil {
		return true, err
	}
	if x.Kind != vX {
		return true, refuse("divided PutCounter total SVar %s is not the paid X", name)
	}
	ps.consume("TgtPrompt")
	t := lc.addTarget(lTarget{F: f, Zone: zBattlefield, Min: cst(0), Max: x, Divided: kop.DivMaxV})
	lc.emit(lInstr{Op: "DividedCounters", Obj: lRef{Kind: rTarget, N: t}, Note: "distribute X +1/+1 counters among 0..X targets"})
	return true, nil
}
