package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Defiler cycle (Defiler of Dreams, Faith, Flesh, Instinct, Vigor;
// kop.StatDefiler): "As an additional cost to cast <colour> permanent spells,
// you may pay N life. Those spells cost {C} less to cast if you paid life this
// way. This effect reduces only the amount of <colour> mana you pay."
// Copied from gorge (rules defiler_reducecolor.go, cast_begin.go):
//
//   - bsDefiler, between the multikicker ask and the squad ask: one yes/no
//     (ChGenConfirm, yes first) per applicable static of the caster's, in
//     battlefield order. An ask is skipped, with nothing posed, when the
//     spell's cost has no pip of that colour left after the earlier
//     reductions, or when the life is not payable beside the spell's own
//     life and the earlier yeses (CR 119.4).
//   - a yes adds the static's life to the spell's cost (paid once, in one
//     loss, beside the cost's other life: payDefilerLife) and one pip of its
//     colour to the reductions (defilerTake: after the statics, never spilling
//     to generic, CR 601.2f). The answers live in build scratch (defN..defRed),
//     which a reversed cast drops with the rest of the build.
//   - the offer gate: a plain cast unaffordable at full price is offered when
//     it is payable with every applicable Defiler paid, greedily in order,
//     while the life covers it and the colour pip remains
//     (reducedOptionalCastable, defilerOfferCost).
//
// No State field: the answers are build scratch (scratch.go, arm B), which
// the scratch region's size (armCombat's) leaves room for.

// useDefiler is the genPrep.uses bit for a table with a StatDefiler static: a
// registered use, so the fixed State layout is unchanged
// (TestNoNewFixedStateFields) and a table without one pays one bit test.
var useDefiler = newGenUse(func(t *kop.Table) bool { return tableHasStatic(t, kop.StatDefiler) })

// defilerOn reports whether the table holds a Defiler static.
func defilerOn(k *prep) bool { return k.gen != nil && k.gen.uses.has(useDefiler) }

// maxDefilers bounds the applicable statics one cast considers; a ninth is a
// declared capacity stall (StallBoard).
const maxDefilers = 8

// DebugDefilerNoPipGate (test only, TestDefilerMutation) skips the "no pip of
// that colour left" gate: the ask is posed with nothing to reduce.
var DebugDefilerNoPipGate bool

// DebugDefilerNoOffer (test only) leaves the offer gate's second chance off.
var DebugDefilerNoOffer bool

// DebugDefilerSpill (test only) makes a paid Defiler take generic instead of
// a pip of its colour.
var DebugDefilerSpill bool

// DefilerCounts are the tests' witnesses that a lockstep run reached the
// asks: yes and no answers, casts that paid two or more Defilers, and plain
// casts the offer gate admitted only on the reduction. Not read by the kernel.
var DefilerCounts struct{ Yes, No, Multi, OfferOnly atomic.Uint64 }

type defilerView struct{ col, life uint8 }

// defilerViews lists, in battlefield order, the Defiler statics of p that
// apply to card c (object o) as a spell p casts (gorge reduceColorOptionalViews).
func (s *State) defilerViews(k *prep, p uint8, c CardID, o Obj, out *[maxDefilers]defilerView) int {
	if !defilerOn(k) || !s.statM.and(s.ctrlM[p]).any() {
		return 0
	}
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(p, s.statM.and(s.ctrlM[p]), ord[:], 0)
	m := 0
	for _, sl := range ord[:n] {
		if s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatDefiler || !s.matchSpellCard(k, a.Static.Filter, c, p, p, o, sl, 0) {
				continue
			}
			if m == len(out) {
				s.capFull(StallBoard)
				return m
			}
			out[m] = defilerView{col: uint8(a.Static.B), life: uint8(a.Static.A)}
			m++
		}
	}
	return m
}

// defilerOwnLife is the life the build already owes: the spell's own PayLife
// part (gorge pc.cost.Life).
func (s *State) defilerOwnLife(k *prep) int {
	if pc := s.buildPickCost(k); pc != nil && pc.Flags&kop.CostPayLife != 0 {
		return lifeCostOwed(pc)
	}
	return 0
}

// defilerOptions poses the next applicable Defiler's yes/no, or reports
// false when none is left. A re-step with an ask posed poses it again.
func (s *State) defilerOptions(k *prep, d *Decision) bool {
	b := s.bld()
	e := &b.e
	if b.defPend {
		s.defilerAsk(d)
		return true
	}
	var vs [maxDefilers]defilerView
	n := s.defilerViews(k, e.ctrl, e.card, e.obj, &vs)
	for int(b.defN) < n {
		v := vs[b.defN]
		b.defN++
		if (s.buildCost(k).Pips[v.col] == 0 && !DebugDefilerNoPipGate) ||
			int(s.pl[e.ctrl].life) < s.defilerOwnLife(k)+int(b.defLife)+int(v.life) {
			continue
		}
		b.defPend, b.defCol, b.defAsk = true, v.col, v.life
		s.defilerAsk(d)
		return true
	}
	return false
}

// defilerAsk fills d with the posed ask: yes first (gorge's options "yes",
// "no").
func (s *State) defilerAsk(d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChGenConfirm, s.bld().e.ctrl
	d.add(MakeAction(AkChoose, 0, 0, 1))
	d.add(MakeAction(AkChoose, 0, 0, 0))
}

// answerDefiler records the answer: yes adds the life and the colour pip. The
// stage stays, so the next Defiler is asked or the stage ends.
func (s *State) answerDefiler(a Action) {
	b := s.bld()
	b.defPend = false
	if a.Mode() != 1 {
		DefilerCounts.No.Add(1)
		return
	}
	DefilerCounts.Yes.Add(1)
	b.defLife += b.defAsk
	b.defRed[b.defCol]++
}

// defilerTake takes the answered Defilers' pips off base: one per yes, of its
// colour, never from generic (gorge onePip, IgnoreGeneric). The result is the
// generic the mutation DebugDefilerSpill takes instead, else 0.
func (s *State) defilerTake(base *kop.ManaCost) (spill int) {
	b := s.bld()
	for i := range b.defRed {
		if DebugDefilerSpill {
			spill += int(b.defRed[i]) // the reduction takes generic, not the pip
			continue
		}
		base.Pips[i] -= min(b.defRed[i], base.Pips[i])
	}
	return spill
}

// defilerPricing: the spell being built is card c, cast with Defiler answers
// to apply.
func (s *State) defilerPricing(c CardID) bool {
	if s.ctl != ctlBuild {
		return false
	}
	b := s.bld()
	return b.e.kind == seSpell && b.e.card == c && b.defRed != [5]uint8{}
}

// payDefilerLife pays the answered Defilers' life in one loss.
func (s *State) payDefilerLife(k *prep) {
	if n := s.bld().defLife; n > 0 {
		if s.bld().defRed[0]+s.bld().defRed[1]+s.bld().defRed[2]+s.bld().defRed[3]+s.bld().defRed[4] > 1 {
			DefilerCounts.Multi.Add(1)
		}
		s.loseLife(k, s.bld().e.ctrl, int(n))
	}
}

// defilerOfferCost is the offer gate's second chance (gorge
// reducedOptionalCastable): for a plain cast of card c (object o) by p
// unaffordable at full price, the cost priced with every applicable Defiler
// paid, taken greedily in order while the life covers it and the colour pip
// remains in raw, the unmodified base cost. own is the life the spell's own
// additional cost owes. ok is false when no Defiler applies.
func (s *State) defilerOfferCost(k *prep, p uint8, c CardID, o Obj, raw *kop.ManaCost, own int, priced kop.ManaCost) (kop.ManaCost, bool) {
	var vs [maxDefilers]defilerView
	n := s.defilerViews(k, p, c, o, &vs)
	if n == 0 {
		return priced, false
	}
	budget := int(s.pl[p].life) - own
	left := raw.Pips
	took := false
	for _, v := range vs[:n] {
		if left[v.col] == 0 || int(v.life) > budget {
			continue
		}
		left[v.col]--
		budget -= int(v.life)
		if priced.Pips[v.col] > 0 {
			priced.Pips[v.col]--
		}
		took = true
	}
	return priced, took
}

// defilerPoolPays is the offer gate's second chance (gorge
// reducedOptionalCastable): the plain cast of card c (object o) by p,
// unaffordable at the priced cost, is offered when the FLOATING POOL pays the
// cost with the applicable Defilers paid. gorge's planner declines a cast with
// an applicable Defiler (shape:optional_cost, payment_plan.go), so a cast the
// pool cannot pay at the composed cost reaches the seat only through its
// potential planner, which prices no Defiler; the engine's own offer, with the
// reduction, is the pool's alone. raw is the unmodified base cost, own the life
// the spell's own additional cost owes. It returns the reduced cost.
func (s *State) defilerPoolPays(k *prep, p uint8, c CardID, o Obj, raw *kop.ManaCost, own int, cost kop.ManaCost) (kop.ManaCost, bool) {
	rd, ok := s.defilerOfferCost(k, p, c, o, raw, own, cost)
	if !ok || DebugDefilerNoOffer {
		return rd, false
	}
	var pav avail
	s.availPool(p, &pav)
	if s.rbAny(p) {
		s.restrAvail(k, p, c, s.restrOriginOfObj(o), &pav)
	}
	if !pav.affords(&rd, pipMaskOf(&rd), 0) {
		return rd, false
	}
	DefilerCounts.OfferOnly.Add(1)
	return rd, true
}

// defilerApplies reports an applicable Defiler for card c (object o) cast by
// p: gorge's cast planner declines the cast then.
func (s *State) defilerApplies(k *prep, p uint8, c CardID, o Obj) bool {
	var vs [maxDefilers]defilerView
	n := s.defilerViews(k, p, c, o, &vs)
	return n > 0
}
