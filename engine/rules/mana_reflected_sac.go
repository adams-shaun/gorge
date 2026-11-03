package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// manaReflectedSacrificeLive is the offer gate of a ManaReflected ability whose
// Valid$ reads the permanent its own Sac<...> cost sacrifices (Squandered
// Resources, "Sacrifice a land: Add one mana of any type that land could
// produce"). Costs are paid after the ability is announced, so no object has
// been sacrificed yet and the reflected set read off an empty Remembered list
// is always empty -- the ability was never offered. It is live when at least
// one permanent the cost could sacrifice would reflect a mana.
func manaReflectedSacrificeLive(h effects.Host, pe pay.Engine, p state.PlayerID, source state.ObjID, cost costvocab.Cost, c *effects.Ctx, ma *cards.SA) bool {
	for _, part := range cost.Sac {
		for _, id := range pay.ManaSacrificeCandidates(pe, p, source, part) {
			c.Remembered = []state.Target{{Obj: id}}
			if len(effects.ManaReflectedCandidates(h, c, ma)) > 0 {
				return true
			}
		}
	}
	return false
}
