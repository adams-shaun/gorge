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
	// A lowered hybrid activation's witness is printed-only, like gorge's
	// planHybridCost (dirac/mana.go's noGrant rule, CR 107.4e's mana
	// objects: a granted ability is the grant search, phase 2): the
	// announcement's trial here is the same resolution, so Insidious
	// Roots' any-colour token is not the {G} face the {G/W} prices.
	ng, ax := s.g.noGrant, s.g.availX
	s.g.noGrant, s.g.availX = true, s.g.availX|availPlain
	r := s.hybResolve(k, p, *c, 0)
	s.g.noGrant, s.g.availX = ng, ax
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
	a := &k.t.Cards[s.bld().e.card].Abilities[s.bld().e.ab]
	c0 := s.abPlannedCost(k, a, s.srcSlot(), p) // the planner's priced cost
	if c0.Hyb[0] == 0 || c0.X != 0 || s.poolPays(p, resolved) {
		return false
	}
	return s.hybResolve(k, p, c0, 0) != *resolved
}

// payReplanned pays a resolution-time mana cost the way gorge funds an
// elected "pay" (rules respay.go resPayFund): while the pool does not pay
// it, plan the rest and activate only that plan's first source, then plan
// again -- the plan of the whole cost can leave a different source
// untapped (kverify-c4: Digsite Conservator's {4} over a Swamp and five
// Forests). The manual window's decision carries no owed cost, so an
// any-colour source answers its first colour, W (windowTapNoOwed). The
// planner's witness has printed abilities only: a granted one (Great Divide
// Guide's any-colour mana) is a V1 exclusion (pay.PaymentAbility), so a
// payment only granted sources could fund has no witness and takes the
// manual window.
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
	dry.dry, dry.noGrant = true, true
	s.tapPlanNeed(k, p, &c, 0, NoObj, true, false, &dry)
	if dry.needT > 0 && !dry.found {
		return s.trigWindowPays(k, p, &c) // gorge's manual window: tap every useful source, no owed answer
	}
	for i := 0; i < MaxPerm && !s.poolPays(p, &c); i++ {
		pk := planner{noGrant: true}
		sl := s.tapForPK(k, p, &c, 0, NoObj, true, false, &pk)
		if sl == NoSlot {
			break
		}
		if s.parkedWaiting() {
			return false // a sacrificed source's order ask: the payment runs again after it
		}
	}
	if !s.poolPays(p, &c) {
		return s.trigWindowPays(k, p, &c)
	}
	s.payFromPool(p, &c, 0)
	return true
}

// trigWindowPays runs the manual window for a payment the planner has no
// witness for and charges the cost when what the window floated pays it, as
// gorge's resolution pay does after the window (Mardu Devotee turning one
// Swamp's black into the white a {1}{W} cost needs); a window that stops short
// leaves its taps and the mana floating, and a paused one finishes on its
// answer.
//
//go:noinline
func (s *State) trigWindowPays(k *prep, p uint8, c *kop.ManaCost) bool {
	if s.trigWindowTap(k, p, c) || !s.poolPays(p, c) {
		return false
	}
	s.payFromPool(p, c, 0)
	return true
}
