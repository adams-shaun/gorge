package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// paidSourceWindow is what triggeredCostXPaidNet reads of the engine: the board,
// each source's probe-admitted mana abilities, the compiled cost one activation
// pays and the mana it makes. Its fields are the Engine's own method values
// (e.castWindowProbeAbilities, e.manaAbilityCC, e.castWindowAmount), so the
// census adds no Engine surface.
type paidSourceWindow struct {
	g              *state.Game
	probeAbilities func(p state.PlayerID, id state.ObjID) []*cards.SA
	feeCost        func(p state.PlayerID, id state.ObjID, ma *cards.SA) *compiledCost
	amount         func(p state.PlayerID, id state.ObjID, o *state.Object, ma *cards.SA) (int32, bool)
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
	for _, id := range pay.BattlefieldManaSourceIDs(w.g, p) {
		o := w.g.Obj(id)
		if priced[id] || o == nil || o.Face() == nil {
			continue
		}
		bestNet, bestFee := int32(0), int32(0)
		for _, ma := range w.probeAbilities(p, id) {
			mp := effects.ManaOf(ma)
			if mp.RestrictValid != "" && !admits(id, ma) {
				continue
			}
			cost := w.feeCost(p, id, ma).Cost
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

// triggeredCostXFeeFree is units with every alternative whose activation costs
// mana left out, and a unit left with none dropped. The payment-plan census
// lists a fixed-production ability whose cost is more than a bare tap at its
// gross production (Viridescent Bog's "{1}, {T}: Add {B}{G}" as two mana),
// which triggeredCostXSupply would count in full though the fee comes out of
// mana the other sources supply. Left out of the units, such a source is priced
// at its net by triggeredCostXPaidNet instead (CR 605.3b, 601.2b: an announced
// X must be one the payment can settle). A cost with no mana part (a
// self-sacrifice "Sac<1/CARDNAME>: Add {C}") stays in the units at its gross.
func triggeredCostXFeeFree(w paidSourceWindow, p state.PlayerID, units []pay.WindowUnit) []pay.WindowUnit {
	out := make([]pay.WindowUnit, 0, len(units))
	for _, u := range units {
		var keep []pay.WindowAlt
		for _, a := range u.Alts {
			if a.Ma != nil {
				cost := w.feeCost(p, u.ID, a.Ma).Cost
				fee := cost.Generic
				for _, n := range cost.Colored {
					fee += n
				}
				if fee > 0 {
					continue
				}
			}
			keep = append(keep, a)
		}
		if len(keep) == 0 {
			continue
		}
		u.Alts = keep
		out = append(out, u)
	}
	return out
}
