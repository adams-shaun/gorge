package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// unpayableTriggerCostWindowDone is the oracle harness's fallback for the
// mana-activation window an optional triggered cost poses ahead of its
// pay/decline ask (paymentManaAsk: one "activate" option per untapped source,
// then "done"), in a generated compliance scenario. The runner's generic
// fallback takes the first Min options, which TAPS the first source; when
// the cost cannot be paid even with every source floated, that tap only
// leaves mana in the pool before the decline-only ask follows (Esoteric
// Duplicator's "you may pay {2}" with one Llanowar Elves: gorge's pool G,
// XMage's empty, because XMage never activates a mana ability for a cost it
// cannot pay). Answering "done" there declines exactly as XMage does. A cost
// the seat's potential mana (PotentialMana, the pool plus every source's
// upper bound) can cover keeps the tapping fallback, so every payable
// scenario plays as before. Reports the "done" option's index.
func (r *oracleRun) unpayableTriggerCostWindowDone(d *decision.Decision) (int, bool) {
	e := r.e
	tc := e.triggerCost
	if !r.xmageFixture || tc == nil || d.Kind != decision.KChoose || len(d.Options) < 2 ||
		d.Player != tc.player {
		return 0, false
	}
	done := len(d.Options) - 1
	if d.Options[done].Kind != "done" {
		return 0, false
	}
	for _, o := range d.Options[:done] {
		if o.Kind != "activate" {
			return 0, false
		}
	}
	amt := tc.announcedCost()
	if !amt.Priceable() {
		return 0, false
	}
	potential := e.PotentialMana(tc.player)
	if pay.CostPayablePool(asPayer(e), tc.player, tc.source, pay.SAOther, amt, potential, e.G.Players[tc.player].ManaUnits()) {
		return 0, false
	}
	return d.Options[done].Index, true
}
