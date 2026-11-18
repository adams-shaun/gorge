package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// paidSourceWindow is what triggeredCostXPaidNet reads of the engine: the board,
// each source's probe-admitted mana abilities, the cost one activation pays and
// the mana it makes.
type paidSourceWindow interface {
	game() *state.Game
	probeAbilities(p state.PlayerID, id state.ObjID) []*cards.SA
	feeCost(p state.PlayerID, id state.ObjID, ma *cards.SA) Cost
	amount(p state.PlayerID, id state.ObjID, o *state.Object, ma *cards.SA) (int32, bool)
}

// engineWindow adapts the Engine to paidSourceWindow.
type engineWindow struct{ e *Engine }

func (w engineWindow) game() *state.Game { return w.e.G }
func (w engineWindow) probeAbilities(p state.PlayerID, id state.ObjID) []*cards.SA {
	return w.e.castWindowProbeAbilities(p, id)
}
func (w engineWindow) feeCost(p state.PlayerID, id state.ObjID, ma *cards.SA) Cost {
	return w.e.manaAbilityCC(p, id, ma).Cost
}
func (w engineWindow) amount(p state.PlayerID, id state.ObjID, o *state.Object, ma *cards.SA) (int32, bool) {
	return w.e.castWindowAmount(p, id, o, ma)
}

// triggeredCostXPaidNet is the mana a source the payment-window census leaves
// out adds to the announced-X supply: a mana ability whose cost is a tap plus a
// literal mana fee (Ferrous Lake's "{1}, {T}: Add {U}{R}") makes more than it
// costs, and the fee comes out of mana the other sources already supplied, so
// the source adds its production less the fee (CR 605.3b: a mana ability
// resolves immediately, so the fee mana is spent before the production lands).
// Sources already in units are priced there. A source adds its net only while
// the supply so far can fund the fee, and each source counts once at its best
// net. admits is the payment's own spend restriction (triggeredCostXSupply).
func triggeredCostXPaidNet(w paidSourceWindow, p state.PlayerID, supply int32, units []pay.WindowUnit,
	admits func(id state.ObjID, ma *cards.SA) bool) int32 {
	priced := map[state.ObjID]bool{}
	for _, u := range units {
		priced[u.ID] = true
	}
	total := supply
	for _, id := range pay.BattlefieldManaSourceIDs(w.game(), p) {
		o := w.game().Obj(id)
		if priced[id] || o == nil || o.Face() == nil {
			continue
		}
		bestNet, bestFee := int32(0), int32(0)
		for _, ma := range w.probeAbilities(p, id) {
			mp := effects.ManaOf(ma)
			if mp.RestrictValid != "" && !admits(id, ma) {
				continue
			}
			cost := w.feeCost(p, id, ma)
			if pay.ActivationTapCostUnavailable(o, &cost) || !castWindowGenericCostShape(cost) {
				continue
			}
			amt, ok := w.amount(p, id, o, ma)
			if !ok || mp.NeedsChosen {
				continue
			}
			made := int32(0)
			if mp.CountsAny {
				made = amt
			} else {
				for _, n := range mp.Counts {
					made += n
				}
				made *= amt
			}
			fee := cost.Generic
			for _, n := range cost.Colored {
				fee += n
			}
			if net := made - fee; net > bestNet {
				bestNet, bestFee = net, fee
			}
		}
		if bestNet > 0 && total >= bestFee {
			total += bestNet
		}
	}
	return total - supply
}
