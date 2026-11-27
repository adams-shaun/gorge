package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// cost_targetmv.go is the target-bound cost filter kgen lowers from
// Targeted$CardManaCost: Kozilek, the Great Distortion's
// Discard<1/Card.cmcEQX> and Behemoth of Vault 0's UnlessCost$ PayEnergy<X>.
// gorge settles these after the target stage (engine/rules/cost_target_bound.go):
// before targets the comparison is open, the target menu keeps only candidates
// whose X leaves the part payable, and payment reads the chosen target. The
// kernel's targetMV already reads a chosen target; the helpers here are the
// offer, menu and mutation surfaces around it.

// TargetCmcStats counts the target-bound discard-filter checks the new
// lowering makes: the lockstep test requires it to fire, so a green game is not
// one where the shape never reached a cost.
type TargetCmcStats struct {
	CostReads    int // a target-bound discard filter was checked
	UnlessEnergy int // an unless PayEnergy amount read a chosen target's mana value
}

// DebugTargetCmcStats collects TargetCmcStats when non-nil.
var DebugTargetCmcStats *TargetCmcStats

// DebugNoTgtMVCost makes a target-bound cost filter ignore the target's mana
// value, leaving the comparison open. The mutation test must diverge.
var DebugNoTgtMVCost bool

// tgtMVBoundV reports whether v is a target's mana value (the ExMVOf over a
// target slot that kgen's Targeted$CardManaCost lowers to).
func tgtMVBoundV(k *prep, v kop.ValueRef) bool {
	if v.K != kop.VExpr || int(v.N) < 0 || int(v.N) >= len(k.t.Exprs) {
		return false
	}
	ex := &k.t.Exprs[v.N]
	return ex.Kind == kop.ExMVOf && ex.Ref >= kop.RefTarget0 && ex.Ref <= kop.RefTarget2
}

// costTgtMVBound reports whether filter chain f compares against a target's
// mana value.
func costTgtMVBound(k *prep, f kop.FilterID) bool {
	for f != 0 {
		fl := &k.t.Filters[f]
		if tgtMVBoundV(k, fl.PredV) {
			return true
		}
		f = fl.Or
	}
	return false
}

// tgtMVBoundPending: before the target stage the comparison is open (gorge's
// cost_target_bound.go StripVarCompares): some target choice fits every
// candidate. Once the referenced slot has an answer the ordinary targetMV read
// decides it.
func (s *State) tgtMVBoundPending(k *prep, fl *kop.Filter, ctx *evalCtx) bool {
	if !tgtMVBoundV(k, fl.PredV) {
		return false
	}
	if DebugNoTgtMVCost {
		return true
	}
	if ctx == nil || ctx.e == nil {
		return true
	}
	ex := &k.t.Exprs[fl.PredV.N]
	idx := int(ex.Ref - kop.RefTarget0)
	if idx < 0 || idx >= kop.MaxTargets {
		return false
	}
	return ctx.e.tgt[idx] == NoTarget
}

// costHasTgtMVDiscard reports whether c's filtered discard reads a target's
// mana value.
func costHasTgtMVDiscard(k *prep, c *kop.Cost) bool {
	if c.Flags&kop.CostDiscard == 0 {
		return false
	}
	_, f, filtered := discardPart(c)
	return filtered && f != 0 && costTgtMVBound(k, f)
}

// tgtMVDiscardPayableAt reports whether p's hand pays c's target-bound discard
// part when the target slot holds code. The stack entry gives the filter the
// target a chosen answer would record, so the targetMV comparison reads it.
func (s *State) tgtMVDiscardPayableAt(k *prep, e *stackEntry, c *kop.Cost, slot int, code TCode) bool {
	n, f, filtered := discardPart(c)
	if !filtered || f == 0 || !costTgtMVBound(k, f) {
		return true
	}
	if DebugTargetCmcStats != nil {
		DebugTargetCmcStats.CostReads++
	}
	save := e.tgt[slot]
	e.tgt[slot] = code
	ctx := s.ctxEntry(e)
	have := s.discardCandNAt(k, e.ctrl, f, e.obj, &ctx)
	e.tgt[slot] = save
	return have >= int(n)
}

// hasTgtMVDiscardTarget reports whether some legal target of slot keeps the
// ability's target-bound discard payable (gorge abilityTargetBoundAvailable).
func (s *State) hasTgtMVDiscardTarget(k *prep, a *kop.Ability, p uint8, src Slot, slot int, excl Obj) bool {
	var buf [MaxOptions]TCode
	n := s.targetCands(k, &a.Targets[slot], p, src, excl, &buf)
	if n == 0 {
		return false
	}
	e := stackEntry{kind: seAbility, ctrl: p}
	if src != NoSlot {
		e.obj = s.perms[src].obj
	}
	for i := 0; i < n; i++ {
		if s.tgtMVDiscardPayableAt(k, &e, &a.Cost, slot, buf[i]) {
			return true
		}
	}
	return false
}

// tgtMVDiscardCands keeps the target-menu candidates whose target leaves the
// ability's target-bound discard payable (gorge targetBoundCandidates).
func (s *State) tgtMVDiscardCands(k *prep, a *kop.Ability, slot int, buf *[MaxOptions]TCode, n int) int {
	if n == 0 || !costHasTgtMVDiscard(k, &a.Cost) {
		return n
	}
	e := &s.bld().e
	out := 0
	for i := 0; i < n; i++ {
		if s.tgtMVDiscardPayableAt(k, e, &a.Cost, slot, buf[i]) {
			buf[out] = buf[i]
			out++
		}
	}
	return out
}

// buildTgtMVDiscardCands narrows the build's target menu to the candidates
// whose target leaves the ability's target-bound discard payable (gorge
// targetBoundCandidates); any other build keeps the menu.
func (s *State) buildTgtMVDiscardCands(k *prep, slot int, buf *[MaxOptions]TCode, n int) int {
	if a := s.buildAbility(k); a != nil {
		return s.tgtMVDiscardCands(k, a, slot, buf, n)
	}
	return n
}

// targetPricedStale reports whether an empty target menu means the offer went
// stale: no candidate's own equip price is payable, or the last target leaving
// the ability's target-bound discard payable is gone.
func (s *State) targetPricedStale(k *prep) bool {
	if _, _, ok := s.equipTargetPriced(k); ok {
		return true
	}
	a := s.buildAbility(k)
	return a != nil && costHasTgtMVDiscard(k, &a.Cost)
}

// noteUnlessEnergyTargetMV counts an unless energy payment whose amount reads
// the target's mana value (Behemoth of Vault 0); the lockstep test requires
// it to fire. A no-op unless DebugTargetCmcStats is set.
func noteUnlessEnergyTargetMV(k *prep, in *kop.Instr, kind int, amt int) {
	if DebugTargetCmcStats != nil && kind == kop.PayEnergy && amt > 0 && tgtMVBoundV(k, in.A) {
		DebugTargetCmcStats.UnlessEnergy++
	}
}
