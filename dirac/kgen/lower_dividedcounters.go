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

// dividedCountersPick lowers the dies trigger "you may distribute a number of
// +1/+1 counters equal to the number it had among any number of creatures you
// control" (Vastwood Hydra; gorge counters_put.go putCounterPickDistribute):
// a Dies Self trigger whose PutCounter P1P1 names a Choices$ pool, reads
// CounterNum$ and ChoiceAmount$ as one SVar TriggeredCard$CardCounters.P1P1
// (the last-known counters) and MinChoiceAmount$ 1. No target is declared:
// the controller picks the recipients as the trigger resolves
// (OpExt ExtDistCounters). ok=false leaves the targeted shapes alone.
func (lc *lowerCtx) dividedCountersPick(ps *params) (bool, error) {
	pool, ok := ps.get("Choices")
	if !ok {
		return false, nil
	}
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "Dies" || lc.depth != 1 || tr.F == nil || len(tr.F.Clauses) != 1 || tr.F.Players != 0 ||
		tr.F.Clauses[0] != (lClause{Flags: fSelf}) {
		return true, refuse("DividedAsYouChoose$ PutCounter outside a Dies Self trigger")
	}
	name := strings.TrimSpace(ps.str("DividedAsYouChoose"))
	if name == "" || ps.str("CounterNum") != name || ps.str("ChoiceAmount") != name || ps.str("MinChoiceAmount") != "1" || ps.str("CounterType") != "P1P1" {
		return true, refuse("divided PutCounter shape (CounterNum$ %q, ChoiceAmount$ %q, MinChoiceAmount$ %q, CounterType$ %q)", ps.str("CounterNum"), ps.str("ChoiceAmount"), ps.str("MinChoiceAmount"), ps.str("CounterType"))
	}
	if lc.face.SVars[name] != "TriggeredCard$CardCounters.P1P1" {
		return true, refuse("divided PutCounter total SVar %s=%q", name, lc.face.SVars[name])
	}
	for _, k := range []string{"ValidTgts", "TargetMax", "TargetMin", "TargetUnique", "TgtZone", "Defined", "Chooser"} {
		if _, ok := ps.get(k); ok {
			return true, refuse("divided PutCounter pick with %s$", k)
		}
	}
	ps.consume("ChoiceTitle")
	f, err := lc.filter(pool)
	if err != nil {
		return true, err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return true, refuse("divided PutCounter pick pool %q names players", pool)
	}
	n, err := lc.value(name)
	if err != nil {
		return true, err
	}
	lc.emit(lInstr{Op: "DistCounters", B: n, F: f, Note: "distribute the source's last-known +1/+1 counters among creatures picked on resolution"})
	return true, nil
}
