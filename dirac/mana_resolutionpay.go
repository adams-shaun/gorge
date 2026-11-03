package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// lowerHybridActivation taps the witness of a hybrid activation the
// oracle lowers (kverify-c4: Puresight Merrow's {W/U}{Q}). gorge offers
// an activation as is through its autopay offer only when the cost has
// generic or coloured mana (rules abilityCostNeedsMana: Silkbind Faerie's
// {1}{W/U} is offered and paid in its mana window, as the kernel pays);
// a hybrid-only cost is offered only when the pool pays, else its
// potential play is lowered: the planner's witness (planHybridCost: the
// first pip resolution the sources afford) is tapped before activating,
// so the mana floats through the pip ask. A pool that already pays it
// taps nothing.
//
//go:noinline
func (s *State) lowerHybridActivation(k *prep, c *kop.ManaCost) {
	p := s.bld().e.ctrl
	if s.poolPays(p, c) {
		return
	}
	r := s.hybResolve(k, p, *c, 0)
	s.tapFor(k, p, &r, 0, NoObj)
}

// hybAbilityOffPlan: gorge arms a hybrid activation the pool does not pay
// with its planner's witness (rules armAbilityAutoPay, over the cost
// after the pip asks), and the witness resolves each pip as
// planHybridCost does (the first resolution the pool and sources afford):
// a pip answered with another colour changes the cost, and gorge falls
// back to its manual window (paymentPlanCheck's cost_changed), which the
// oracle answers source by source (kverify-c4: Silkbind Faerie's
// {1}{W/U} paid {1}{U}; a lowered Puresight Merrow whose witness {W}
// floats, paid {U}). resolved is the cost after the pip asks.
//
//go:noinline
func (s *State) hybAbilityOffPlan(k *prep, p uint8, resolved *kop.ManaCost) bool {
	c0 := k.t.Cards[s.bld().e.card].Abilities[s.bld().e.ab].Cost.Mana
	if c0.Hyb[0] == 0 || c0.X != 0 || s.poolPays(p, resolved) {
		return false
	}
	return s.hybResolve(k, p, c0, 0) != *resolved
}

// windowTapHeld taps p's sources the way gorge's manual cast window is
// answered, leaving the mana in the pool for the payment (payCastWindow
// without the spend).
//
//go:noinline
func (s *State) windowTapHeld(k *prep, p uint8, cost *kop.ManaCost) {
	_, held := s.windowTap(k, p, cost, true)
	for c := range held {
		s.pl[p].pool[c] += held[c]
	}
}

// payReplanned pays a resolution-time mana cost the way gorge funds an
// elected "pay" (rules respay.go resPayFund): while the pool does not pay
// it, plan the rest and activate only that plan's first source, then plan
// again -- the plan of the whole cost can leave a different source
// untapped (kverify-c4: Digsite Conservator's {4} over a Swamp and five
// Forests). The manual window's decision carries no owed cost, so an
// any-colour source answers its first colour, W (windowTapNoOwed).
// When the planner has no witness at all -- a pay
// gorge offered on its potential bound the sources cannot actually fund
// (respay.go:84, potential.go:324) -- gorge funds through that same manual
// window, tapping every useful source, then the charge fails and the flow
// declines. The taps and the floated mana stay; nothing panics. The dry
// probe is that no-witness test (a dry plan reports found without tapping:
// mana.go plan), and the final poolPays check is the same fallback for a
// plan that stops short.
//
//go:noinline
func (s *State) payReplanned(k *prep, p uint8, cost *kop.ManaCost) bool {
	c := *cost
	if c.Hyb[0] != 0 {
		c = s.hybResolve(k, p, c, 0)
	}
	var dry planner
	dry.dry = true
	s.tapPlanNeed(k, p, &c, 0, NoObj, true, false, &dry)
	if dry.needT > 0 && !dry.found {
		s.trigWindowTap(k, p, &c) // gorge's manual window: tap every useful source, no owed answer
		return false
	}
	for i := 0; i < MaxPerm && !s.poolPays(p, &c); i++ {
		sl := s.tapForN(k, p, &c, 0, NoObj, true)
		if sl == NoSlot {
			break
		}
		if s.parkedWaiting() {
			return false // a sacrificed source's order ask: the payment runs again after it
		}
	}
	if !s.poolPays(p, &c) {
		s.trigWindowTap(k, p, &c)
		return false
	}
	s.payFromPool(p, &c, 0)
	return true
}
