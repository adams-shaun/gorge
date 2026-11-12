package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// raiseCostGen is the generic mana the generated StatIncreaseCost statics
// add to card c (object o) cast by p (gorge costModifiersCompose's raise
// group): each battlefield static whose controller relation matches
// (StRaiseYou: its controller casts it; StRaiseOpp: an opponent does;
// neither: anyone) and whose filter and condition hold, and the card's own
// "this spell costs more" (Zone ZoneCommand).
//
//go:noinline
func (s *State) raiseCostGen(k *prep, p uint8, c CardID, o, priced Obj, pips *[kop.NumManaKinds]uint8) int {
	g := 0
	add := func(a *kop.Ability, ctx evalCtx) {
		ctx.uncounted = priced + 1
		st := &a.Static
		if st.StFlags&kop.StRaiseLife != 0 {
			return // life, not generic mana (cost_raiselife.go)
		}
		if st.Cond2 != 0 && !s.condEval(k, st.Cond2, ctx) {
			return
		}
		if !s.matchSpellCard(k, st.Filter, c, p, ctx.ctrl, o, ctx.srcSl) {
			return
		}
		if st.StFlags&kop.StTargetsFilter2 != 0 && !s.raiseTargetOK(k, st.Filter2, p, c) {
			return // a ValidTarget$ raise: the charge with a matching target only (cost_reducespell.go)
		}
		n := 1
		if st.StFlags&kop.StRaisePerTarget != 0 {
			if n = s.raisePerTargetCharge(c, p); n == 0 {
				return
			}
			addRaisePips(pips, st.B, n)
		}
		if st.StFlags&kop.StDynamic != 0 {
			g += n * max(s.evalV(k, st.VA, ctx), 0)
		} else {
			g += n * int(st.A)
		}
	}
	s.eachStatic(k, func(stc statSrc) bool {
		a := stc.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatIncreaseCost || a.Static.Zone == kop.ZoneCommand {
			return true
		}
		if f := a.Static.StFlags; f&kop.StRaiseYou != 0 && stc.ctrl != p || f&kop.StRaiseOpp != 0 && stc.ctrl == p {
			return true
		}
		add(a, s.ctxStatic(stc))
		return true
	})
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatIncreaseCost && a.Static.Zone == kop.ZoneCommand {
			add(a, evalCtx{src: o, srcSl: NoSlot, ctrl: p})
		}
	}
	g += s.raiseStackSelfGen(k, p, c, priced)
	return g
}

// raiseStackSelfGen is raiseCostGen's stack-source arm: a RaiseCost static
// with EffectZone$ Stack (kop.ZoneStack on an IncreaseCost) is live while its
// source spell sits on the stack and taxes a spell targeting it ("as long as
// CARDNAME is on the stack, spells that target it cost {2} more": Kaervek's
// Torch, CR 113.6, 601.2f). The charge is read from the cast being built's
// chosen targets (e.nt/e.tgt: CR 601.2c's announcement, before 601.2f), and
// only when the target is that source spell (ValidTarget$ Spell.Self; the
// lowered shape is a literal generic amount). A cast priced before its
// targets are chosen owes nothing yet, as gorge's offer composition skips a
// ValidTarget$ raise; the planner's target-dependent shape (stackTgtDep)
// prices it per announcement instead.
func (s *State) raiseStackSelfGen(k *prep, p uint8, c CardID, priced Obj) int {
	if !s.buildLive() {
		return 0
	}
	e := &s.bld().e
	if e.kind != seSpell || e.card != c || e.ctrl != p {
		return 0
	}
	g := 0
	for i := 0; i < int(s.sp); i++ {
		se := &s.stack[i]
		if se.kind != seSpell {
			continue
		}
		src := se.obj
		if src == NoObj || int(src) >= MaxObj {
			continue
		}
		ob := &s.objs[src]
		if ob.zone != kop.ZoneStack || ob.slot == slotFaceDownSpell {
			continue // the source must be on the stack; a face-down spell has no printed abilities (CR 708.4)
		}
		cd := &k.t.Cards[ob.card]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatIncreaseCost || a.Static.Zone != kop.ZoneStack {
				continue
			}
			st := &a.Static
			if st.StFlags&(kop.StRaiseLife|kop.StDynamic|kop.StRaisePerTarget) != 0 {
				continue // the lowered stack shape is a literal generic raise
			}
			if f := st.StFlags; f&kop.StRaiseYou != 0 && se.ctrl != p || f&kop.StRaiseOpp != 0 && se.ctrl == p {
				continue
			}
			hit := false
			for t := 0; t < int(e.nt); t++ {
				if Obj(e.tgt[t]) == src {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
			if st.Cond2 != 0 {
				ctx := evalCtx{src: src, srcSl: NoSlot, ctrl: se.ctrl}
				ctx.uncounted = priced + 1
				if !s.condEval(k, st.Cond2, ctx) {
					continue
				}
			}
			g += int(st.A)
		}
	}
	return g
}

// stackRaiseOnStack reports whether a spell currently on the stack carries a
// stack-zone RaiseCost static (the shape raiseStackSelfGen prices): the
// "spells that target it cost more while it is on the stack" class.
func (s *State) stackRaiseOnStack(k *prep) bool {
	for i := 0; i < int(s.sp); i++ {
		se := &s.stack[i]
		if se.kind != seSpell {
			continue
		}
		src := se.obj
		if src == NoObj || int(src) >= MaxObj {
			continue
		}
		ob := &s.objs[src]
		if ob.zone != kop.ZoneStack || ob.slot == slotFaceDownSpell {
			continue
		}
		cd := &k.t.Cards[ob.card]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatIncreaseCost && a.Static.Zone == kop.ZoneStack {
				return true
			}
		}
	}
	return false
}

// stackTgtDep reports that p's cast of spell c is priced by its target
// because a stack-zone RaiseCost is live (raiseStackSelfGen): gorge's
// paymentPlanTargetDependentFor declines that cast's witness
// ("shape:target_dependent_cost"), so the planner prices it per announcement.
func (s *State) stackTgtDep(k *prep, c CardID) bool {
	if k.gen == nil || !k.gen.anyRaise || tspec(k, c, spellAb(k, c), 0) == nil {
		return false
	}
	return s.stackRaiseOnStack(k)
}

// tableRaises reports whether t has a generated StatIncreaseCost. A free cast
// is charged the raise on what it still owes (CR 601.2f, 118.9d; gorge
// manaToPay composes the modifiers on the empty cost): buildCostGenPre.
func tableRaises(t *Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatIncreaseCost {
				return true
			}
		}
	}
	return false
}

// freeOwes reports whether a spell cast without paying its mana cost can still
// owe something to the cost statics: a table with a RaiseCost, whose generic
// lands on the empty cost.
func (s *State) freeOwes(k *prep) bool { return k.gen != nil && k.gen.anyRaise }

// raisePerTargetCharge is the number of times a StRaisePerTarget raise is
// owed on card c cast by p: the targets the cast being built holds, less the
// first. It is 0 outside a build of that card (the offer and the priority
// walk price no targets yet; gorge modAmountX counts none in the potential
// pass).
func (s *State) raisePerTargetCharge(c CardID, p uint8) int {
	if !s.buildLive() {
		return 0
	}
	e := &s.bld().e
	if e.kind != seSpell || e.card != c || e.ctrl != p {
		return 0
	}
	return max(0, s.ksrTotal(e)-1)
}

// addRaisePips adds n charges of the packed pips b (StReduceColor's packing)
// to pips, saturating at 255.
func addRaisePips(pips *[kop.NumManaKinds]uint8, b int16, n int) {
	for i := 0; i < kop.NumManaKinds; i++ {
		w := int(b>>(kop.ReduceColorBits*i)) & (1<<kop.ReduceColorBits - 1)
		pips[i] = uint8(min(int(pips[i])+w*n, 255))
	}
}
