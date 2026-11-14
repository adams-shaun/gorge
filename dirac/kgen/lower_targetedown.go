package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Defined$ Targeted.<qualifier> beside the node's OWN ValidTgts$ (A Killer
// Among Us: `AB$ PutCounter | ValidTgts$ Creature.token+attacking | Defined$
// Targeted.ChosenType`, "if target attacking creature token is the chosen
// type, put three +1/+1 counters on it"). gorge definedSpec reads the
// chain's targets, which at the head are the node's own, and keeps the ones
// the qualifier admits: the target is chosen without the qualifier and the
// effect acts on it only if it matches at resolution. The node lowers as
// its own target slot (the effect declares it) guarded by a SkipIf on that
// slot matching Card.<qualifier>, the same guard node() builds for a link's
// Targeted.<qualifier>. Only PutCounter carries the spelling in the corpus,
// so only PutCounter lowers it.

// ownTargetedGuard returns the guard of a PutCounter node whose Defined$
// Targeted.<qualifier> reads its own ValidTgts$ slot, consuming Defined$;
// ok is false for any other node. The guard names the slot the effect is
// about to declare; ownTargetedSlot checks it did.
func (lc *lowerCtx) ownTargetedGuard(sa *cards.SA, ps *params) (lCond, bool, error) {
	d, ok := ps.m["Defined"]
	if !ok || !strings.HasPrefix(d, "Targeted.") || sa.API != "PutCounter" {
		return lCond{}, false, nil
	}
	if _, ok := ps.m["ValidTgts"]; !ok {
		return lCond{}, false, nil
	}
	if len(lc.chainT) != 0 {
		return lCond{}, true, refuse("Defined$ %q beside its own ValidTgts$ after %d chain target slots", d, len(lc.chainT))
	}
	f, err := lc.filter("Card." + strings.TrimPrefix(d, "Targeted."))
	if err != nil {
		return lCond{}, true, err
	}
	lc.targetedDefined = d
	return lCond{Kind: cRefMatches, Ref: lRef{Kind: rTarget, N: len(lc.ab.Targets)}, F: f, Op: opGE, A: cst(1)}, true, nil
}

// ownTargetedSlot checks the effect declared exactly the guarded slot, and
// that the slot holds one object at most: the kernel keeps only a slot's
// first object in its target cell, while gorge's definedSpec reads them all.
func (lc *lowerCtx) ownTargetedSlot(guard lCond) error {
	want := guard.Ref.N
	if len(lc.ab.Targets) != want+1 || lc.lastTarget() != want {
		return refuse("Defined$ Targeted.<qualifier> beside its own ValidTgts$: the effect declared no single target slot")
	}
	tg := lc.ab.Targets[want]
	if !tg.Min.isConst() || !tg.Max.isConst() || tg.Min.N > 1 || tg.Max.N > 1 {
		return refuse("Defined$ Targeted.<qualifier> beside its own ValidTgts$ on a multi-target slot")
	}
	return nil
}
