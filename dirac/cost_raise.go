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
func (s *State) raiseCostGen(k *prep, p uint8, c CardID, o, priced Obj) int {
	g := 0
	add := func(a *kop.Ability, ctx evalCtx) {
		ctx.uncounted = priced + 1
		st := &a.Static
		if st.Cond2 != 0 && !s.condEval(k, st.Cond2, ctx) {
			return
		}
		if !s.matchSpellCard(k, st.Filter, c, p, ctx.ctrl, o, ctx.srcSl) {
			return
		}
		if st.StFlags&kop.StTargetsFilter2 != 0 && !s.raiseTargetOK(k, st.Filter2, p, c) {
			return // a ValidTarget$ raise: the charge with a matching target only (cost_reducespell.go)
		}
		if st.StFlags&kop.StDynamic != 0 {
			g += max(s.evalV(k, st.VA, ctx), 0)
		} else {
			g += int(st.A)
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

// tableRaises reports whether t has a generated StatIncreaseCost, and
// whether it also has a free cast (OpCastFree, a play-from-zone static
// without the mana cost): gorge charges the raise on a free cast (CR
// 601.2f; manaToPay composes the modifiers on the empty cost), which the
// kernel's castFree path does not price -- that table stalls loud.
func tableRaises(t *Table) (raise, free bool) {
	for i := range t.Cards {
		if t.Cards[i].HasPlot || t.Cards[i].Types&kop.TBattle != 0 {
			// (a Battle's defeat cast is free too:)
			// a plotted card's later cast is free, and gorge composes a
			// raise on that empty cost (CR 601.2f): the kernel's free
			// path does not price it
			free = true
		}
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatIncreaseCost {
				raise = true
			}
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom && a.Static.StFlags&kop.StWithoutManaCost != 0 {
				free = true
			}
			for _, in := range a.Prog {
				if in.Op == kop.OpCastFree || in.Ext() == kop.ExtPlotGrant {
					// (an effect's plotted grant, the same
					// later free cast)
					free = true
				}
			}
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			for _, in := range t.Emblems[i].Abilities[j].Prog {
				if in.Op == kop.OpCastFree {
					free = true
				}
			}
		}
	}
	return raise, raise && free
}
