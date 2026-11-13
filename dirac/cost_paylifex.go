package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An announced PayLife<X> ("as an additional cost, pay X
// life", Fix What's Broken, Vicious Rivalry, Krumar Initiate's
// {X}{B},{T}, Pay X life; CR 601.2b, 107.3a). The table encodes the
// announced form as CostPayLife with Cost.N == 0 (a literal PayLife<N> has
// N >= 1, so every fixed-life reader stays correct on it: loseLife(0) is a
// no-op, `life < 0` never holds). The spell or activation announces X
// (ChX, mode = X) and pays that much life at the point the fixed life is
// paid; the body reads the announcement as Count$xPaid, the stack entry's
// X. gorge: Cost.LifeX, xAsk (cast_subcounter.go: a LifeX part poses the
// ask even with no {X} mana; announced-only it SETS the bound, the cap is
// life, floored at 0; beside a printed {X} it min-clamps the mana bound),
// settled as one LifeChange of -x per part, only when x > 0
// (cast_commit.go). Cold code: the build reaches it only behind
// genPrep.anyLifeX.

// DebugLifeXNoPay is the mutation switch: the announced life payment pays 0.
var DebugLifeXNoPay bool

// prepLifeX sets anyLifeX.
func prepLifeX(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if lifeXCost(&t.Cards[i].SpellCost) {
			g.anyLifeX = true
		}
		for j := range t.Cards[i].Abilities {
			if lifeXCost(&t.Cards[i].Abilities[j].Cost) {
				g.anyLifeX = true
			}
		}
	}
}

// lifeXCost reports whether c pays an announced X life.
func lifeXCost(c *kop.Cost) bool {
	return c != nil && c.Flags&kop.CostPayLife != 0 && c.N == 0
}

// lifeXBuilding reports whether the entry being built pays an announced
// life part.
//
//go:noinline
func (s *State) lifeXBuilding(k *prep) bool {
	return lifeXCost(s.buildPickCost(k))
}

// lifeXClamp bounds the X ask of a LifeX cost being built: announced only
// (no {X} mana, cost.X == 0) the cap is the payer's life, offered only when
// X = 0 is payable by the same avail (mx >= 0), else gorge's empty-range
// fallback offers X = 0 alone and payCast aborts (cast_subcounter.go: the
// restored 0..maxOld); beside a printed {X} the life min-clamps the mana
// bound mx. X is a uint8.
//
//go:noinline
func (s *State) lifeXClamp(k *prep, cost *kop.ManaCost, mx int) int {
	if !s.lifeXBuilding(k) {
		return mx
	}
	life := int(s.pl[SeatIx(s.bld().e.ctrl)].life)
	life = max(0, min(life, 255))
	if cost.X == 0 {
		if mx < 0 {
			return 0
		}
		return life
	}
	return min(mx, life)
}

// lifeCostOwed is the amount c's life part charges: Cost.N for a fixed
// PayLife<N>, Cost.Ctr for a may-play raise's PayLife<N> beside its sacrifice
// (cost_mayplayraise.go), and 0 for the announced PayLife<X> form (N == 0, no
// sacrifice), which payLifeCost settles from the announced x instead.
func lifeCostOwed(c *kop.Cost) int {
	if c.Flags&kop.CostSac != 0 {
		return int(c.Ctr)
	}
	return int(c.N)
}

// payLifeCost pays a cost's life part: its fixed amount (lifeCostOwed), or the
// announced x for the PayLife<X> form (owed == 0).
//
//go:noinline
func (s *State) payLifeCost(k *prep, p uint8, c *kop.Cost, x uint8) {
	n := lifeCostOwed(c)
	if n == 0 && !DebugLifeXNoPay {
		n = int(x)
	}
	s.loseLife(k, p, n)
}
