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
	return g
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
