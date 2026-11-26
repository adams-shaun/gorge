package oracle

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// cyclingAltOption reports whether the "ability" option o is a hand card's
// cycling activation at the alternative cost (AltCostIndex 1: New Perspectives'
// "pay {0} rather than pay cycling costs", gorge activate_static_altcost.go).
// The only other activation option at an alternative cost is an Equip's, which
// works from the battlefield.
func (g *GorgeSide) cyclingAltOption(o *decision.Option) bool {
	if o.AltCostIndex != 1 || o.SVar != "" || o.Keyword != "" || o.GrantSource != 0 || o.GainedSource != 0 || o.Ability < 0 || o.Ability > 0x3F {
		return false
	}
	ob := g.E.G.Obj(o.Obj)
	return ob != nil && ob.Zone == state.ZHand
}
