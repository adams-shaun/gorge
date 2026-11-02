package kgen

import (
	"strconv"
)

// Class enchantments (CR 716), lowered from gorge's kwClass
// expansion (engine/cards/kw_class.go): per level N above 1, an activator
// `AB$ ClassLevelUp | Level$ N | IsPresent$ Card.Self+classLevel_EQ<N-1> |
// SorcerySpeed$ True` and the level's granted bodies stamped ClassBand$ N.
// The kernel gates both on its CtrLevel count (level-1): the activator
// exactly at N-1, a granted trigger while the level is >= N. A granted
// static or replacement keeps ClassBand$ unread and refuses.

// classLevelUp lowers the activator's effect (gorge effClassLevelUp) and
// its level gate.
func (lc *lowerCtx) classLevelUp(ps *params) error {
	if lc.ab.Kind != akActivated || !lc.head || lc.depth != 1 || lc.ab.Gate != nil {
		return refuse("ClassLevelUp outside a Class activator")
	}
	n, err := strconv.Atoi(ps.str("Level"))
	if err != nil || n < 2 {
		return refuse("ClassLevelUp Level %q", ps.str("Level"))
	}
	if ip := ps.str("IsPresent"); ip != "Card.Self+classLevel_EQ"+strconv.Itoa(n-1) {
		return refuse("ClassLevelUp IsPresent %q", ip)
	}
	if !lc.ab.Sorcery || lc.ab.Zone != 0 && lc.ab.Zone != zBattlefield {
		return refuse("ClassLevelUp not a sorcery-speed battlefield activation")
	}
	lc.ab.Gate = &lGate{Level: n - 1, Eq: true}
	lc.emit(lInstr{Op: "ClassLevelUp", Obj: lRef{Kind: rSelf}, A: cst(n)})
	return nil
}

// classBand reads a granted trigger's ClassBand$ N (gorge addLevelGate):
// the trigger exists from level N on.
func (lc *lowerCtx) classBand(tp *params) error {
	v, ok := tp.get("ClassBand")
	if !ok || lc.ab.Gate != nil {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 2 {
		return refuse("ClassBand %q", v)
	}
	lc.ab.Gate = &lGate{Level: n}
	return nil
}

// classLevelGained lowers "when this Class becomes level N" (gorge
// classLevelGainedMatches: a crossing from below N to N or more). The
// kernel's level only moves through the activator, one level at a time,
// so the crossing is the new level being exactly N; the trigger is a
// self TrigCounterAddedOnce of CtrLevel gated Eq on N (which also covers
// its own ClassBand$ N).
func (lc *lowerCtx) classLevelGained(tp *params, tr *lTrigger) error {
	if vc := tp.str("ValidCard"); vc != "Card.Self" {
		return refuse("ClassLevelGained ValidCard %q", vc)
	}
	n, err := strconv.Atoi(tp.str("ClassLevel"))
	if err != nil || n < 2 {
		return refuse("ClassLevelGained ClassLevel %q", tp.str("ClassLevel"))
	}
	if b, ok := tp.get("ClassBand"); ok && b != strconv.Itoa(n) {
		return refuse("ClassLevelGained ClassLevel %d in band %q", n, b)
	}
	if lc.ab.Gate != nil {
		return refuse("ClassLevelGained with a second gate")
	}
	tr.F = &lFilter{Clauses: []lClause{{Flags: fSelf}}}
	lc.ab.Gate = &lGate{Level: n, Eq: true}
	return nil
}

// classLevelCounter is the counterKinds key of kop.CtrLevel, a Class's
// level-1 count: no Forge counter is named this, so a Level Up card's
// LEVEL counters never alias it.
const classLevelCounter = "__CLASSLEVEL"

// classBandCond is a granted static's or replacement's ClassBand$ N (gorge
// addLevelGate / classBandGateHolds): it exists while its Class is level N
// or higher, a self condition on the CtrLevel count (level-1) >= N-1.
func classBandCond(v string) (lCond, error) {
	n, err := strconv.Atoi(v)
	if err != nil || n < 2 {
		return lCond{}, refuse("ClassBand %q", v)
	}
	f := &lFilter{Clauses: []lClause{{Flags: fSelf, CmpStat: cmpCounters, CmpOp: opGE, CmpN: n - 1, CmpCounter: classLevelCounter}}}
	return lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}, nil
}
