package kgen

import "strings"

// selfSacrificeGate reports whether f is the lone `Card.Self` present check of
// a ConditionDefined$ Remembered gate over a chain whose only remember write
// is the source's own `Sacrifice … RememberSacrificed$ True`. The group such a
// gate reads can hold nothing but the source, so the persistent mark (non-empty
// list) answers it exactly: Monkey Cage's second trigger, resolving after an
// earlier one already sacrificed it, still meets the gate in gorge (the host's
// remembered list survives the resolution) and makes its tokens: the
// oracle text is an unconditional "sacrifice Monkey Cage and create ...", so
// the tokens are right by the rules too. A gate beside any other remember write
// keeps the per-resolution lowering, the mark being a bare non-emptiness test (cond:RemMark).
func (lc *lowerCtx) selfSacrificeGate(f *lFilter) bool {
	if f == nil || len(f.Clauses) != 1 || f.Players != 0 || f.Each || f.PickSumMax != nil || f.PickDistinctPower {
		return false
	}
	self := f.Clauses[0]
	if self.Flags != fSelf {
		return false
	}
	self.Flags = 0
	if !anyCardClause(self) {
		return false
	}
	sacs := 0
	for _, in := range lc.ab.Prog {
		if !strings.HasPrefix(in.Note, "remember") {
			continue
		}
		if !strings.HasPrefix(in.Note, noteRememberSacrificed) || in.Op != "Sacrifice" || in.F == nil ||
			len(in.F.Clauses) != 1 || in.F.Clauses[0].Flags != fSelf {
			return false
		}
		sacs++
	}
	return sacs > 0
}

// remMarkFilterOK reports whether the persistent mark may be ORed into a
// ConditionDefined$ Remembered gate whose present filter is f: an unrestricted
// one, or the source-only check of selfSacrificeGate. Never over a battle-link
// pick (gateOverPick): the gate then reads the pick's own set.
func (lc *lowerCtx) remMarkFilterOK(f *lFilter) bool {
	return (filterAnyCard(f) || lc.selfSacrificeGate(f)) && !lc.gateOverPick()
}
