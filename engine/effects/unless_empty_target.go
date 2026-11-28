package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// unlessControllerStandsIn reports whether the resolving controller is asked
// when sa's UnlessPayer$ selector spec named no player from targetList: only
// for the target-derived family (unlessPayerControllerFallbackSet), and never
// when the targets it reads were an answered empty set.
func unlessControllerStandsIn(c *Ctx, sa *cards.SA, spec string, targetList []state.Target) bool {
	return unlessPayerControllerFallback(spec) && !unlessTargetAnsweredEmpty(c, sa, targetList)
}

// unlessTargetAnsweredEmpty reports whether an up-to slot sa's payer reads was
// answered with zero targets (CR 115.6): either sa's own slot, covered by the
// placement ask (Ctx.OfferedSA), or the nearest targeting parent link's. There
// is then no "its controller" to take the CR 118.12 election, and CR 608.2d
// gives nobody an impossible choice. An ability with no such slot is not this
// case.
func unlessTargetAnsweredEmpty(c *Ctx, sa *cards.SA, targetList []state.Target) bool {
	if len(targetList) != 0 {
		return false
	}
	if c.OfferedSA != nil && c.OfferedSA.Line == sa.Line && TargetsOf(sa).Targeted() {
		return true
	}
	n := len(c.parentLinks)
	return n > 0 && len(c.parentLinks[n-1]) == 0
}
