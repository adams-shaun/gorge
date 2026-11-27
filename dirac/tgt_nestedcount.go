package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The nested target count (kop.ExCountTgtEntry, gorge
// engine/effects/count_body.go's TargetedByTarget$Valid <spec> arm): the object
// targets, matching the spec, of the spell or ability the evaluating entry
// targets. entryTgtClause (dirac/tgt_countershapes.go) walks an entry's OWN
// targets for a TargetValidTargeting$ clause; this walks one level further, so
// Not of This World's "costs {7} less if it targets a spell or ability that
// targets a creature you control with power 7 or greater" reads the counter's
// target, not its own.
//
// The subject entry -- the one whose targets are walked -- is the resolution's
// own (gorge's Ctx.Targets), the spell being built during the CR 601.2c/h
// reprice (the cost-modifier static path binds the cast's chosen targets), or,
// at the offer gate, the spell's potential-target census: gorge binds
// costAmountTargets, the first resolvedTargetBounds' maximum candidates of
// costPotentialTargets, so a cast whose reduction only a target can unlock is
// offered at the reduced price, exactly as a ValidTarget$ reduction is (the
// kernel's optimistic costTargetOK arm). A spell with no legal candidate
// targets reads zero, as gorge's nil census does.

// DebugTgtEntryCountOff (mutation, TestTargetedByTargetLockstepMutation): the
// nested count always reads zero, so a reduction gated on it never applies.
var DebugTgtEntryCountOff bool

// countTgtEntryMatches is the kernel's read of the nested target count for the
// evaluating context: over the subject entry's targets, those that name an
// object (never a player, an ability code or an empty slot) and whose OWN
// targets include one matching f. ctrl is the evaluating controller, the "you"
// of the spec (gorge's SpecContext(c.Controller)).
func (s *State) countTgtEntryMatches(k *prep, f kop.FilterID, ctx evalCtx) int {
	if DebugTgtEntryCountOff {
		return 0
	}
	if e := s.tgtEntrySubject(ctx); e != nil {
		return s.countNestedTgts(k, f, e, ctx.ctrl)
	}
	return s.offerNestedTgtCount(k, f, ctx)
}

// tgtEntrySubject is the entry whose own targets the nested read walks: the
// resolving entry, or the spell being built (gorge binds the cast's chosen
// targets onto the context a cost modifier's Amount$ reads). nil when neither
// holds, which is the offer gate.
func (s *State) tgtEntrySubject(ctx evalCtx) *stackEntry {
	if e := ctx.e; e != nil {
		return e
	}
	if s.ctl == ctlBuild {
		return &s.bld().e
	}
	return nil
}

// countNestedTgts counts the targets of every stack entry entry e's own targets
// name: a spell by its object, an activated or triggered ability by its stack
// index. Player targets and empty slots contribute nothing, as gorge's
// IsPlayer and Obj == 0 skips do.
func (s *State) countNestedTgts(k *prep, f kop.FilterID, e *stackEntry, ctrl uint8) int {
	n := 0
	s.eachEntryTgt(e, func(c TCode) {
		inner := s.tgtStackEntry(c)
		if inner == nil {
			return
		}
		s.eachEntryTgt(inner, func(ic TCode) {
			if s.tgtMatchesFor(k, f, ic, ctrl) {
				n++
			}
		})
	})
	return n
}

// eachEntryTgt walks entry e's target bytes: its slots and the extra
// multi-object ones, the walk entryTgtClause makes. An unset slot is no
// target and no matcher accepts it.
func (s *State) eachEntryTgt(e *stackEntry, fn func(TCode)) {
	for _, c := range e.tgt {
		fn(c)
	}
	x := s.xtOf(e)
	for j := 0; j < int(x.n); j++ {
		fn(x.code[j])
	}
}

// tgtStackEntry is the stack entry target byte c names: an ability by its stack
// index, a spell by the object it will move (an object off the stack names no
// entry). nil for a player, an empty slot or anything not on the stack.
func (s *State) tgtStackEntry(c TCode) *stackEntry {
	if isStackTarget(c) {
		i := int(c - StackTarget0)
		if i >= int(s.sp) {
			return nil // seat-ok: a stack index, not a seat
		}
		return &s.stack[i]
	}
	if !isObjTarget(c) || Obj(c) == NoObj {
		return nil
	}
	if s.objs[Obj(c)].zone != kop.ZoneStack {
		return nil
	}
	if i := s.stackIndexOf(Obj(c)); i >= 0 {
		return &s.stack[i]
	}
	return nil
}

// tgtMatchesFor: target byte c names an object now that matches f for
// controller ctrl. A player target and an empty slot match nothing (gorge skips
// both before the spec read); an object that no longer exists matches nothing,
// as gorge's g.Obj(it.Obj) == nil skip does. The match is the one a ref read
// makes (countRefMatches' RefSelf arm): a battlefield permanent through the
// layer-walk matcher, so a derived power reads as gorge's FilterDerivedPT bind
// does, and a moved card as the card in its new zone. ctx.e is dropped: a
// nested target is not this resolution's own target, so no source LKI applies.
func (s *State) tgtMatchesFor(k *prep, f kop.FilterID, c TCode, ctrl uint8) bool {
	if !isObjTarget(c) || Obj(c) == NoObj {
		return false
	}
	isc := evalCtx{ctrl: ctrl, src: Obj(c), srcSl: s.objs[Obj(c)].slot}
	return s.countRefMatches(k, kop.RefSelf, f, isc) == 1
}

// offerNestedTgtCount is the offer gate's optimistic read: the count over the
// spell ctx prices (its object in hand, exile or graveyard) as its target
// candidates. gorge's costAmountTargets trims the potential census to a
// complete legal assignment -- the first resolvedTargetBounds' maximum
// candidates -- and this mirrors that trim with the slot's own bounds, so the
// price the offer shows is the price the announcement can reach.
func (s *State) offerNestedTgtCount(k *prep, f kop.FilterID, ctx evalCtx) int {
	// the spell being priced: adjustCost prices a card still in hand (or exile,
	// or a graveyard) with no source object, and names it only through
	// evalCtx.uncounted, the one-past object its zone counts skip.
	o := ctx.src
	if o == NoObj && ctx.uncounted != 0 {
		o = ctx.uncounted - 1
	}
	if int(o) >= len(s.objs) || o == NoObj {
		return 0
	}
	c := s.objs[o].card
	t := tspec(k, c, spellAb(k, c), 0)
	if t == nil {
		return 0
	}
	var buf [MaxOptions]TCode
	pr := s.g.probe
	if pr.kind == 0 {
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: ctx.ctrl}
	}
	n := s.targetCands(k, t, ctx.ctrl, NoSlot, NoObj, &buf)
	s.g.probe = pr
	if max := int(t.Max); max > 0 && max < n {
		n = max // the reduction-favourable witness gorge's trim keeps
	}
	count := 0
	for i := 0; i < n; i++ {
		inner := s.tgtStackEntry(buf[i])
		if inner == nil {
			continue
		}
		s.eachEntryTgt(inner, func(ic TCode) {
			if s.tgtMatchesFor(k, f, ic, ctx.ctrl) {
				count++
			}
		})
	}
	return count
}

// exprReadsNestedTgts reports whether an expression tree reads the nested
// target count, so a cost static's Amount$ is target-dependent and its spell's
// target menu must be priced candidate by candidate (affordableTargets).
func exprReadsNestedTgts(t *kop.Table, v kop.ValueRef) bool {
	if v.K != kop.VExpr || int(v.N) >= len(t.Exprs) {
		return false
	}
	x := &t.Exprs[v.N]
	if x.Kind == kop.ExCountTgtEntry {
		return true
	}
	return exprReadsNestedTgts(t, x.Sub[0]) || exprReadsNestedTgts(t, x.Sub[1])
}
