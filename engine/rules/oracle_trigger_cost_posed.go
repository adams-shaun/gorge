package rules

import (
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// xmagePosesTriggerCost reports a triggered optional cost whose XMage
// DoIfCostPaid poses its chooseUse in the current state, however little mana
// gorge could pay. DoIfCostPaid skips the ask only when the cost's canPay is
// false, and a mana cost's canPay is "true all the time"; so the ask is posed
// exactly when every NON-mana component is payable now:
//   - discard your hand: always (DiscardHandCost.canPay is unconditional);
//   - pay N life: the payer's life is at least N (PayLifeCost.canPay);
//   - an exile cost: each part has at least N candidates in its zones;
//   - collect evidence N: the graveyard's total mana value reaches N
//     (CollectEvidenceCost.canPay).
//
// Any other component (sacrifice, tap, counters, a card-count discard, ...)
// keeps the unposed reading: gorge's decline-only ask over it stays unmarked.
func xmagePosesTriggerCost(b exileCostBoard, p state.PlayerID, source state.ObjID, c Cost) bool {
	for _, part := range c.Discard {
		if part.Spec != "Hand" {
			return false
		}
	}
	c.Discard = nil
	g := b.Game()
	if c.Life > 0 {
		if int(p) >= len(g.Players) || g.Players[p].Life < c.Life {
			return false
		}
		c.Life = 0
	}
	if !pay.EvidencePayable(g, p, &c) {
		return false
	}
	c.Evidence = nil
	for _, part := range c.Exile {
		if part.Announced || isWholeZoneExileSpec(part.Spec) || int32(len(xmageExileCandidates(b, p, source, part))) < part.N {
			return false
		}
	}
	c.Exile = nil
	return c.Priceable()
}

// xmageExileCandidates lists the objects that could pay one Exile cost part
// of a triggered cost: every zone the part names, minus a battlefield object
// a cost-block static protects, filtered by the part's spec.
func xmageExileCandidates(b exileCostBoard, p state.PlayerID, source state.ObjID, part CostPart) []state.ObjID {
	var out []state.ObjID
	for _, zone := range exilePartZones(part) {
		for _, oid := range pay.ExileCostCandidates(b.Game(), zone, p, part) {
			if zone == state.ZBattlefield && b.exileBlockedForCost(oid, pay.CostCauseForAbility(true)) {
				continue
			}
			if b.matchesSpecFrom(part.Spec, oid, p, source) {
				out = append(out, oid)
			}
		}
	}
	return out
}
