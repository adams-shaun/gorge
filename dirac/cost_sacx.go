package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The announced-X floor (XMin<N> / XMin$ N: "X can't be 0") and the announced
// sacrifice Sac<X/Spec> ("sacrifice X creatures"; CR 107.3, 601.2b, 601.2g).
// The table encodes the floor as Cost.XMin (a byte; 0 is no floor) and the
// announced sacrifice as CostSac with Cost.N == 0 (a literal Sac<N/Spec> has
// N >= 1, so every fixed-sacrifice reader stays correct on it; the same
// shape as CostPayLife N 0, cost_paylifex.go). gorge: xAsk
// (cast_subcounter.go) folds the floor by maximum over the cost's XMin, the
// suspend bound and the ability's XMin$; an announced Sac part poses the X
// ask even with no {X} mana, the matching sacrifice candidates are its bound
// (announced-only it SETS the bound, beside a mana {X} it caps it), and a
// range with nothing in it aborts the cast without a decision. The settle
// (sacAsk) sacrifices exactly the announced x, asking nothing for x = 0.
// Cold code: the build reaches it only behind genPrep.anySacX / anyXMin.

// DebugXMinIgnored is the mutation switch: every floor reads 0.
var DebugXMinIgnored bool

// prepSacX sets anySacX and anyXMin.
func prepSacX(t *kop.Table, g *genPrep) {
	note := func(c *kop.Cost) {
		if sacXCost(c) {
			g.anySacX = true
		}
		if c.XMin != 0 {
			g.anyXMin = true
		}
	}
	for i := range t.Cards {
		note(&t.Cards[i].SpellCost)
		for j := range t.Cards[i].Abilities {
			note(&t.Cards[i].Abilities[j].Cost)
		}
	}
}

// sacXCost reports whether c sacrifices the announced X permanents.
func sacXCost(c *kop.Cost) bool {
	return c != nil && c.Flags&kop.CostSac != 0 && c.N == 0
}

// costXMin is cost c's announced-X floor.
func costXMin(c *kop.Cost) int {
	if DebugXMinIgnored {
		return 0
	}
	return int(c.XMin)
}

// sacXBuilding reports whether the entry being built sacrifices the announced
// X.
//
//go:noinline
func (s *State) sacXBuilding(k *prep) bool {
	return sacXCost(s.buildPickCost(k))
}

// xFloor is the least X the ask offers for the entry being built.
//
//go:noinline
func (s *State) xFloor(k *prep) int {
	if k.gen == nil || !k.gen.anyXMin {
		return 0
	}
	if pc := s.buildPickCost(k); pc != nil {
		return costXMin(pc)
	}
	return 0
}

// sacXClamp bounds the X ask of a Sac<X> cost being built by the permanents
// it could sacrifice: announced only (no {X} mana, cost.X == 0) the count is
// the bound, offered only when X = 0 is payable by the same avail (mx >= 0),
// else gorge's empty-range fallback offers X = 0 alone and the payment aborts;
// beside a printed {X} the count caps the mana bound mx. X is a uint8.
//
//go:noinline
func (s *State) sacXClamp(k *prep, cost *kop.ManaCost, mx int) int {
	pc := s.buildPickCost(k)
	if !sacXCost(pc) {
		return mx
	}
	e := &s.bld().e
	src := NoSlot
	if e.kind == seAbility {
		src = s.srcSlot()
	}
	n := min(s.countSac(k, e.ctrl, pc.Filter, src), 255)
	if cost.X == 0 {
		if mx < 0 {
			return 0
		}
		return n
	}
	return min(mx, n)
}

// xFloorCost is mana with its {X} priced at the floor xm: the offer's
// cheapest legal announcement (gorge mana.go: the cost WithX at XMin).
func xFloorCost(mana kop.ManaCost, xm int) kop.ManaCost {
	if xm == 0 || mana.X == 0 {
		return mana
	}
	return xFolded(mana, xm)
}

// sacXTargetsFeasible is the activation offer's target check for an ability
// whose only announced X is its Sac<X/Spec> count. gorge's offer
// (abilityTargetsAvailable) does not count that part among the costs that
// announce X (costAnnouncesX), so an X-bound target is judged at X = 0, not
// swept over the legal announcements: Ruthless Technomancer is offered with
// a power-0 creature card in the graveyard only.
//
//go:noinline
func (s *State) sacXTargetsFeasible(k *prep, a *kop.Ability, p uint8, sl Slot) bool {
	s.g.probe = probe{kind: probeAbility, src: sl, ctrl: p, x: 1}
	ok := s.abilityTargetsFeasible(k, a, p, sl, 0xff)
	s.g.probe = probe{}
	return ok
}
