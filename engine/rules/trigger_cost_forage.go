package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// The triggered-effect window's forage cost (CR 701.61a): Bushy Bodyguard's,
// Treetop Sentries', Corpseberry Cultivator's, Curious Forager's and Euru
// Acorn Scrounger's `Cost$ Forage` bodies. CR 117.3 / 603.3: the controller
// may pay a cost it can pay, so the window offers "pay" whenever the payer's
// graveyard holds three cards or they control a Food, and the pay arm poses
// the same either-or election the cast path's forageAsk poses (the shared
// forage_exile / forage_food option kinds, so the bot's existing arms answer
// both flows). Only a forage beside a pure priceable mana/life half is
// modelled: any other component keeps the decline-only ask (the
// half-paid-commitment rule the rest of the window keeps).

// foragePayable is the window gate's Forage arm: the either-or is payable
// (three graveyard cards or a controlled Food, the cast path's
// nonManaCastable read) and the rest of the cost is a priceable mana/life
// half the pool covers right now (the same pricing the pay arm charges).
func (tc *triggeredEffectCost) foragePayable(e *Engine, rest Cost) bool {
	other := rest
	other.Forage = false
	if !other.Priceable() {
		return false
	}
	if !triggerForagePayableNow(e, tc.player, tc.source) {
		return false
	}
	return e.triggeredCostManaHalfPayable(tc, other)
}

// triggerForagePayableNow reads CR 701.61a's either-or at the window: exile
// three cards from the payer's graveyard, or sacrifice a Food they control.
// The same read the cast path's forageAsk offers and pay.manaForagePayable
// gates.
func triggerForagePayableNow(e *Engine, p state.PlayerID, source state.ObjID) bool {
	if len(e.G.Zone(state.ZGraveyard, p)) >= 3 {
		return true
	}
	return len(pay.CostCandidates(asPayer(e), p, source, state.ZBattlefield, "Food.YouCtrl", false, false)) > 0
}

// forageElectionAsk poses the either-or election (exile three graveyard
// cards OR sacrifice a Food), sharing the cast path's option kinds so the
// bot's existing arms answer it. A single live arm settles without an ask.
// It reports whether an ask was posed; the answer (forage_exile /
// forage_food) records the arm and re-enters the pay arm.
func (tc *triggeredEffectCost) forageElectionAsk(e *Engine, announced Cost) bool {
	if tc.forageDone {
		return false
	}
	tc.forageDone = true
	graveOK := len(e.G.Zone(state.ZGraveyard, tc.player)) >= 3
	foods := pay.CostCandidates(asPayer(e), tc.player, tc.source, state.ZBattlefield, "Food.YouCtrl", false, false)
	arms := len(foods)
	if graveOK {
		arms++
	}
	switch {
	case arms == 0:
		// The gate's read can no longer be honoured; the settle re-proves
		// and the pay arm declines (nothing has been moved to un-pay).
		return false
	case arms == 1 && !graveOK:
		// The lone Food arm is deterministic.
		tc.forageFood = foods[0]
		return false
	case arms == 1:
		// The lone exile arm is deterministic.
		return false
	}
	d := &decision.Decision{Player: tc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Choose how to forage", Source: tc.source}
	if graveOK {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "forage_exile",
			Label: "Exile three cards from your graveyard"})
	}
	for _, id := range foods {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "forage_food", Obj: id,
			Label: "Sacrifice " + e.targetName(id)})
	}
	windowAsk(e, d, chooseTriggeredCost)
	return true
}

// forageSettle charges the cost: the mana/life half from the pool (any
// energy beside it), then the elected arm -- one Sacrifice for the Food, or
// the top three graveyard cards exiled (the payer's graveyard order is
// deterministic, the mana path's SettleManaForage read) -- and the
// ForageAction marker the forage triggers key on. It re-proves payability
// first, so a paid answer is never a partial payment.
func (tc *triggeredEffectCost) forageSettle(e *Engine, announced Cost) bool {
	if !tc.forageDone || !tc.foragePayable(e, announced) {
		return false
	}
	if tc.forageFood != 0 {
		foods := pay.CostCandidates(asPayer(e), tc.player, tc.source, state.ZBattlefield, "Food.YouCtrl", false, false)
		if !slices.Contains(foods, tc.forageFood) {
			return false
		}
	} else if len(e.G.Zone(state.ZGraveyard, tc.player)) < 3 {
		return false
	}
	lower := announced.WithoutEnergy()
	lower.Forage = false
	if !lower.Priceable() || !pay.PayManaConv(asPayer(e), tc.player, lower, asEval(e).Conv(tc.player, tc.source, false)) {
		return false
	}
	pay.ChargeEnergyCost(asPayer(e), tc.player, tc.amount, tc.xPaid)
	if tc.forageFood != 0 {
		e.emit(events.Sacrifice(tc.forageFood))
	} else {
		n := 0
		for _, id := range e.G.Zone(state.ZGraveyard, tc.player) {
			if n >= 3 {
				break
			}
			if o := e.G.Obj(id); o != nil {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZExile,
					Text: "exiled as a forage cost"})
				n++
			}
		}
	}
	e.emit(events.Event{Kind: events.ForageAction, Player: tc.player})
	return true
}
