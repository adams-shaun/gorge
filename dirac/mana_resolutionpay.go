package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// lowerHybridOnly lowers the activation being built (ability ab of the build's
// card) when its mana cost is hybrid pips alone: the activation gorge lowers
// by tapping the planner's witness before activating, however the source
// is activated (a battlefield permanent's ability, a hand or graveyard
// ability such as cycling, a granted one).
func (s *State) lowerHybridOnly(k *prep, ab uint8) {
	if s.cfg&cfgGorgeUnpriced == 0 || ab >= abCascade {
		return
	}
	if c := &k.t.Cards[s.bld().e.card].Abilities[ab].Cost.Mana; c.Hyb[0] != 0 && !c.Phyrexian() && c.X == 0 && c.Generic == 0 && pipMaskOf(c) == 0 {
		s.lowerHybridActivation(k, c)
	}
}

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
	// The activated source's own {T} cost is paid from it (CR 602.2b,
	// 601.2h), so it is no mana source for the witness: gorge's planner
	// reserves it, and the manual window holds it tapped across the mana
	// step the same way (mana_window.go). Without this the witness taps an
	// Azorius Locket for the {W/U} its own draw ability then needs to tap for.
	sl := s.srcSlot()
	held := sl != NoSlot && k.t.Cards[s.bld().e.card].Abilities[s.bld().e.ab].Cost.Flags&kop.CostTap != 0 && s.perms[sl].flags&pfTapped == 0
	if held {
		s.perms[sl].flags |= pfTapped
	}
	r := s.hybResolve(k, p, *c, 0)
	s.g.noGrant, s.g.availX = ng, ax
	s.tapFor(k, p, &r, 0, NoObj)
	if held {
		s.perms[sl].flags &^= pfTapped
	}
}

// hybAbilityUnwitnessed: gorge arms no witness for this activation: the cost
// its arm gate (rules armAbilityAutoPay's abilityCostNeedsMana) reads prices
// no generic or coloured pip, a cost of hybrid pips alone, so no plan is
// armed at the activation's begin whatever the pool and sources fund, and
// the pips' answers never re-plan one. Its mana is the ordinary manual
// CR 601.2g window's, not a plan's.
func (s *State) hybAbilityUnwitnessed(k *prep, e *stackEntry, a *kop.Ability, sl Slot) bool {
	c := s.abPlannedCost(k, a, sl, e.ctrl)
	return c.Hyb[0] != 0 && c.Generic == 0 && pipMaskOf(&c) == 0
}

// hybWindowPay pays a hybrid-only activation through gorge's ordinary manual
// CR 601.2g window (the case arm hybAbilityUnwitnessed approved): the first
// useful source in battlefield order (Creakwood Ghoul's {B/G}{B/G} over a
// {G} pool: Vivid Meadow's plain {T} floats a {W} before Forest's {G} pays).
// The window can run dry when a source's first ability is not the colour owed
// (a granted basic land type: static_landtype.go; a filter's free {C}); gorge
// then aborts the activation and keeps the taps, as for the off-plan window.
// It reports whether the activation continues.
func (s *State) hybWindowPay(k *prep, e *stackEntry, cost *kop.ManaCost, saved *[len(build{}.picks)]uint8, hostSl, sl Slot, selfUntapped bool) bool {
	if !s.windowTapHeldOK(k, e.ctrl, cost) && k.gen != nil && (k.gen.blgOn || k.gen.anyFilt) {
		s.reverseActivation(k, saved, hostSl, sl, selfUntapped)
		return false
	}
	return true
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
