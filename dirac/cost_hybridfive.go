package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Five hybrid pips (Ulalek, Fused Atrocity's {C/W}{C/U}{C/B}{C/R}{C/G}; CR
// 107.4e, 702.105). kop.ManaCost.Hyb holds five, one past what gorge's
// potential planner resolves: planHybridCost answers "unsupported
// cost:hybrid" for more than planHybridLimit pips, so no potential play is
// priced, none is lowered and none enters the prefix search
// (potentialScriptPlays). The plain cast is then offered only when the
// floating pool alone pays it, and its pips are announced and paid through
// the manual window as any hybrid cast's are. A may-play cast is gated by
// gorge's affordable check (legal_walk_alt.go mayPlayAffordable), not the
// planner, so the limit does not reach it.

//lockstep:const engine/rules.planHybridLimit
const planHybridLimit = 4

// hybPastPlanner reports whether c carries more hybrid pips than gorge's
// potential planner resolves (a twobrid or Phyrexian cost has its own planner).
func hybPastPlanner(c *kop.ManaCost) bool {
	return c.Hyb[0] != kop.HybTwobrid && c.Hyb[planHybridLimit] != 0
}
