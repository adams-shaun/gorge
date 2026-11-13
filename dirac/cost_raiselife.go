package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A RaiseCost static whose Cost$ is PayLife<N> (Terror of the Peaks: "Spells
// your opponents cast that target this creature cost an additional 3 life to
// cast"; CR 601.2f/h, 119.4). gorge composes the static's life into the
// cast's cost once a chosen target matches its ValidTarget$ (composeRaiseCost
// RaiseLife; costTargetsMatch reads the target against the static's source),
// drops a target candidate whose total life cost the caster cannot pay
// (affordableTargetCandidates: cost.Life > life) and pays the life with the
// cast's own. The kernel keeps the static a StatIncreaseCost with StRaiseLife
// (Static.A is life, never generic: raiseCostGen skips it). Cold code: every
// site is behind genPrep.anyRaiseLife.

// DebugRaiseLifeFree is the mutation switch: the life is neither gated nor paid.
var DebugRaiseLifeFree bool

// tableRaiseLife reports whether t has a life raise.
func tableRaiseLife(t *Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatIncreaseCost && a.Static.StFlags&kop.StRaiseLife != 0 {
				return true
			}
		}
	}
	return false
}

// raiseLifeOwed is the life the spell being built (card c, object o, cast by
// p) owes the battlefield life raises, given its chosen targets.
func (s *State) raiseLifeOwed(k *prep, p uint8, c CardID, o Obj) int {
	return s.raiseLifeFor(k, p, c, o, false)
}

// raiseLifeFor is raiseLifeOwed; anyTarget reads the statics as if every
// ValidTarget$ held (gorge paymentPlanTargetDependentFor drops the key).
func (s *State) raiseLifeFor(k *prep, p uint8, c CardID, o Obj, anyTarget bool) int {
	if DebugRaiseLifeFree {
		return 0
	}
	n := 0
	s.eachStatic(k, func(stc statSrc) bool {
		a := stc.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatIncreaseCost || a.Static.StFlags&kop.StRaiseLife == 0 {
			return true
		}
		st := &a.Static
		if f := st.StFlags; f&kop.StRaiseYou != 0 && stc.ctrl != p || f&kop.StRaiseOpp != 0 && stc.ctrl == p {
			return true
		}
		ctx := s.ctxStatic(stc)
		if st.Cond2 != 0 && !s.condEval(k, st.Cond2, ctx) || !s.matchSpellCard(k, st.Filter, c, p, ctx.ctrl, o, ctx.srcSl) {
			return true
		}
		if !anyTarget && st.StFlags&kop.StTargetsFilter2 != 0 && !s.raiseTargetAt(k, st.Filter2, p, c, stc.sl) {
			return true
		}
		n += int(st.A)
		return true
	})
	return n
}

// raiseTargetAt is raiseTargetOK with the static's source slot, so a
// Card.Self ValidTarget$ reads that permanent.
func (s *State) raiseTargetAt(k *prep, f kop.FilterID, p uint8, c CardID, src Slot) bool {
	if s.ctl != ctlBuild {
		return false
	}
	e := &s.bld().e
	if e.card != c || e.kind != seSpell {
		return false
	}
	for i := 0; i < int(e.nt); i++ {
		if s.costTargetMatchSrc(k, f, p, e.tgt[i], src) {
			return true
		}
	}
	x := s.xtOf(e) // the targets past the entry's own slots (Shifting Grift's second creature)
	for j := 0; j < int(x.n); j++ {
		if s.costTargetMatchSrc(k, f, p, x.code[j], src) {
			return true
		}
	}
	return false
}

// raiseLifeBuilt is the life the cast being built owes the raises.
func (s *State) raiseLifeBuilt(k *prep) int {
	e := &s.bld().e
	if e.kind != seSpell {
		return 0
	}
	return s.raiseLifeOwed(k, e.ctrl, e.card, e.obj)
}

// payRaiseLife pays the cast's raised life beside its other costs.
func (s *State) payRaiseLife(k *prep) {
	if n := s.raiseLifeBuilt(k); n > 0 {
		s.loseLife(k, s.bld().e.ctrl, n)
	}
}

// raiseLifeDropTargets drops from the target menu the candidates whose life
// raise the caster cannot pay on top of the spell's own life, each tried as
// the sole selection (gorge affordableTargetCandidates).
func (s *State) raiseLifeDropTargets(k *prep, buf *[MaxOptions]TCode, n int) int {
	e := &s.bld().e
	own := 0
	if ac := spellAddCost(k, e.card); ac.Flags&kop.CostPayLife != 0 {
		own = int(ac.N)
	}
	life := int(s.pl[e.ctrl].life)
	nt, t0 := e.nt, e.tgt[0]
	m := 0
	for i := 0; i < n; i++ {
		e.nt, e.tgt[0] = 1, buf[i]
		if owed := s.raiseLifeOwed(k, e.ctrl, e.card, e.obj); owed == 0 || owed+own <= life {
			buf[m] = buf[i]
			m++
		}
	}
	e.nt, e.tgt[0] = nt, t0
	return m
}

// spellTgtDep reports that p's cast of card c (object o) is priced by a
// target: the card's own ValidTarget$ cost static (cardTgtDepCost) or a
// battlefield life raise that applies to the spell whichever target it
// chooses and a spell that targets (gorge paymentPlanTargetDependentFor:
// "shape:target_dependent_cost", the planner declines it, no prefix search).
func (s *State) spellTgtDep(k *prep, p uint8, c CardID, o Obj) bool {
	return cardTgtDepCost(&k.t.Cards[c]) || s.boardLifeTgtDep(k, p, c, o) || s.stackTgtDep(k, c)
}

// boardLifeTgtDep is spellTgtDep's battlefield half.
func (s *State) boardLifeTgtDep(k *prep, p uint8, c CardID, o Obj) bool {
	if k.gen == nil || !k.gen.anyRaiseLife || tspec(k, c, spellAb(k, c), 0) == nil {
		return false
	}
	return s.raiseLifeFor(k, p, c, o, true) > 0
}
