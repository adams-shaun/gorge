package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A StatRaiseAbility static (RaiseCost Type$ Ability; CR 601.2f, 602.2b):
// activated abilities of battlefield permanents matching its filter cost
// Static.A (VA under StDynamic) generic more, pay Static.A life (StRaiseLife)
// or owe a Sac<1/<filter>> pick (kop.RaiseAbPick, the pick's permanents in
// Static.Filter2) at the activation's cost stage. gorge composes every
// matching static's raise onto the total before the reductions (pay.ApplyTo:
// activate.go's mods applied at the payment); the kernel composes the generic
// into the priced cost the same way (abPlannedCost for the offer's planner
// price, abRaisedCharge at the build charge, abRaised at the offer gate),
// pays the life beside the cost's own (payAbRaiseLife) and poses the pick as
// the entry's pick cost (buildPickCost, gorge's sac ask covering the raise's
// part). Every matching static composes; the pick part poses only the first
// one's pick (a second pick cost is a different Cost than the one a build
// poses, unmodelled: cardset's raise guard refuses a pool holding two).
// Mana abilities the mana window prices pay no raise; the guard keeps a pool
// whose raiser can match a mana-ability card out (cardset.AbRaiseRefusal).
// Cold code: every site is behind genPrep.anyAbRaise.

// DebugNoAbRaise drops the raise (TestAbraiseMutation proves the pool
// activates a raised ability at its raised price).
var DebugNoAbRaise bool

// AbRaisedPriced counts the ability pricings a StatRaiseAbility lowered
// (read by TestAbraiseLockstep; nothing in the engine reads it).
var AbRaisedPriced atomic.Int64

// abRaiseStaticApplies is the per-static gate of a StatRaiseAbility static st
// on ability a of the battlefield source sl activated by ctrl: the condition
// and the ValidCard$ filter over the source.
func (s *State) abRaiseStaticApplies(k *prep, st statSrc, a *kop.Ability, sl Slot, ctrl uint8) bool {
	sa := &st.a.Static
	if st.a.Kind != kop.AbStatic || sa.Kind != kop.StatRaiseAbility {
		return false
	}
	if sa.B&kop.RaiseAbMana != 0 && a.Kind != kop.AbMana {
		return false // a mana-abilities-only raise never raises another ability
	}
	return s.statCond(k, st) && s.matchPermGen(k, sa.Filter, sl, st.ctrl, st.sl, layerFull)
}

// abRaiseParts walks the raise statics of ability a of the battlefield source
// sl activated by ctrl: the generic raise, the life and the pick filter of the
// first matching pick part (none: ok false).
func (s *State) abRaiseParts(k *prep, a *kop.Ability, sl Slot, ctrl uint8) (g, life int, pick kop.FilterID, ok bool) {
	if DebugNoAbRaise || a.Kind != kop.AbActivated && a.Kind != kop.AbMana || sl == NoSlot {
		return 0, 0, 0, false
	}
	s.eachStaticG(k, func(st statSrc) bool {
		if !s.abRaiseStaticApplies(k, st, a, sl, ctrl) {
			return true
		}
		sa := &st.a.Static
		n := int(sa.A)
		if sa.StFlags&kop.StDynamic != 0 {
			n = s.evalV(k, sa.VA, s.ctxStatic(st))
		}
		if n > 0 {
			if sa.StFlags&kop.StRaiseLife != 0 {
				life += n
			} else {
				g += n
			}
		}
		if sa.B&kop.RaiseAbPick != 0 && !ok {
			pick, ok = sa.Filter2, true
		}
		return true
	})
	return g, life, pick, ok
}

// abRaised is the composition the offer gate and the planner price: c with
// every matching static's generic raise on top (raises before reductions, CR
// 601.2f; the life and the pick part ride the build's own payments, the offer
// gate abRaiseOK prices them).
func (s *State) abRaised(k *prep, a *kop.Ability, sl Slot, ctrl uint8, c kop.ManaCost) kop.ManaCost {
	g, _, _, ok := s.abRaiseParts(k, a, sl, ctrl)
	if !ok && g == 0 {
		return c
	}
	if g > 0 {
		c.Generic = uint8(min(int(c.Generic)+g, 255))
		AbRaisedPriced.Add(1)
	}
	return c
}

// abRaisedCharge is abRaised for an ability stack entry being charged
// (buildCostPreRaw's seAbility arm): the source is the entry's object.
func (s *State) abRaisedCharge(k *prep, a *kop.Ability, o Obj, ctrl uint8, c kop.ManaCost) kop.ManaCost {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return c
	}
	return s.abRaised(k, a, ob.slot, ctrl, c)
}

// abRaiseOK is the offer gate: p's activation of a owes every matching
// static's life and pick part, and can pay them (gorge's offer gate prices the
// composed cost: pay.NonManaCastable over the raise's life and pick).
func (s *State) abRaiseOK(k *prep, sl Slot, a *kop.Ability, p uint8) bool {
	if k.gen == nil || !k.gen.anyAbRaise {
		return true
	}
	_, life, pick, ok := s.abRaiseParts(k, a, sl, p)
	if life > 0 && int(s.pl[p].life) < life {
		return false
	}
	if ok && s.countCostSac(k, p, &kop.Cost{Flags: kop.CostSac, N: 1, Filter: pick}, NoSlot) < 1 {
		return false
	}
	return true
}

// abRaisePick is the pick part the entry being built owes, or nil.
func (s *State) abRaisePick(k *prep, e *stackEntry) *kop.Cost {
	a := s.buildAbility(k)
	if a == nil {
		return nil
	}
	_, _, pick, ok := s.abRaiseParts(k, a, s.srcSlot(), e.ctrl)
	if !ok {
		return nil
	}
	return &kop.Cost{Flags: kop.CostSac, N: 1, Filter: pick}
}

// abRaiseLifeBuilt is the life the entry being built owes the raises.
func (s *State) abRaiseLifeBuilt(k *prep) int {
	e := &s.bld().e
	if e.kind != seAbility {
		return 0
	}
	a := s.buildAbility(k)
	if a == nil {
		return 0
	}
	_, life, _, _ := s.abRaiseParts(k, a, s.srcSlot(), e.ctrl)
	return life
}

// payAbRaiseLife pays the activation's raised life beside its other costs.
func (s *State) payAbRaiseLife(k *prep) {
	if n := s.abRaiseLifeBuilt(k); n > 0 {
		s.loseLife(k, s.bld().e.ctrl, n)
	}
}
