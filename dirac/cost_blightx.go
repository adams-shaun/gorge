package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An announced Blight<X> as a spell's own additional cost ("as an additional
// cost to cast this spell, blight X. X can't be greater than the greatest
// toughness among creatures you control", Soul Immolation; CR 701.68, 601.2b,
// 107.3). The table encodes it as CostBlight with Cost.N == 0 (a literal
// Blight<N> has N >= 1, so every fixed-blight reader stays correct on it; the
// same shape as CostPayLife N 0, cost_paylifex.go). The spell prints no {X}:
// gorge's xAsk poses the announcement for the part alone, bounds it by the
// greatest toughness among the creatures the payer controls
// (pay.BlightCandidates, e.Toughness; the printed `XMax$ GrTo`), and
// blightCostAsk then asks the one creature the counters go on (a single
// candidate is recorded unasked, even at X = 0). The kernel's X ask offers
// 0..that toughness, its cost-pick stage poses the same creature pick
// (act_forage.go) and payBlightPicks puts X counters on it. Cold code: the
// build reaches it only behind genPrep.anyBlightX.

// DebugBlightXCapIgnored is the mutation switch: the X ask offers every X up
// to 255, not the greatest toughness.
var DebugBlightXCapIgnored bool

// prepBlightX sets anyBlightX.
func prepBlightX(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if blightXCost(&t.Cards[i].SpellCost) {
			g.anyBlightX = true
		}
		for j := range t.Cards[i].Abilities {
			if blightXCost(&t.Cards[i].Abilities[j].Cost) {
				g.anyBlightX = true // a spell ability carries the spell's own additional cost (spellAddCost)
			}
		}
	}
}

// blightXCost reports whether c blights the announced X counters.
func blightXCost(c *kop.Cost) bool {
	return c != nil && c.Flags&kop.CostBlight != 0 && c.N == 0
}

// blightXOwed is the number of -1/-1 counters cost c's CostBlight part puts on
// its creature: the announced x for Blight<X>, else the printed count.
func blightXOwed(c *kop.Cost, x int) int {
	if blightXCost(c) {
		return x
	}
	return int(c.N)
}

// blightXBuilding reports whether the entry being built blights the announced
// X.
//
//go:noinline
func (s *State) blightXBuilding(k *prep) bool {
	return blightXCost(s.buildPickCost(k))
}

// blightXClamp bounds the X ask of the entry being built by the greatest
// toughness among the creatures its blight part could choose (gorge xAsk's
// blightX cap: a creature with a negative toughness counts 0). The spell
// prints no {X}, so the cap is the bound itself (cost.X == 0), offered only
// when the mana is payable (mx >= 0), as sacXClamp; beside a printed {X} it
// would cap the mana bound.
//
//go:noinline
func (s *State) blightXClamp(k *prep, cost *kop.ManaCost, mx int) int {
	pc := s.buildPickCost(k)
	if !blightXCost(pc) {
		return mx
	}
	e := &s.bld().e
	src := NoSlot
	if e.kind == seAbility {
		src = s.srcSlot()
	}
	n := 0
	for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
		sl := m.first()
		if !s.matchPerm(k, pc.Filter, sl, e.ctrl, src) {
			continue
		}
		if _, t := s.ptGen(k, sl); t > n {
			n = t
		}
	}
	n = min(n, 255)
	if DebugBlightXCapIgnored {
		n = 255
	}
	if cost.X == 0 {
		if mx < 0 {
			return 0
		}
		return n
	}
	return min(mx, n)
}
