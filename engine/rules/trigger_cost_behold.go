package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// The triggered-effect window's behold cost (CR 701.4a): Sarkhan, Dragon
// Ascendant's `AB$ Token | Cost$ Behold<1/Dragon>` ETB body ("you may behold
// a Dragon. If you do, ..."). To behold, the payer chooses a permanent they
// control or reveals a card from their hand with the quality; nothing moves.
// The window offers "pay" whenever every Behold part has enough candidates
// (the cast flow's beholdCostAsk pool: the battlefield, then the hand), the
// pay arm poses the "beholdcost" pick (unasked when the candidates are
// exactly the part's count), and the settle announces the beheld ids with
// the same Note the cast and unless-cost flows emit. Only a behold beside a
// pure priceable mana/life half is modelled: any other component keeps the
// decline-only ask (the half-paid-commitment rule the rest of the window
// keeps).

// beholdCandidates is a Behold part's candidate pool, excluding the objects
// already beheld for an earlier part of the same cost.
func (tc *triggeredEffectCost) beholdCandidates(e *Engine, spec string, reserved []state.ObjID) []state.ObjID {
	all := append(pay.CostCandidates(asPayer(e), tc.player, tc.source, state.ZBattlefield, spec, false, false),
		pay.CostCandidates(asPayer(e), tc.player, tc.source, state.ZHand, spec, true, false)...)
	out := all[:0]
	for _, id := range all {
		if !slices.Contains(reserved, id) {
			out = append(out, id)
		}
	}
	return out
}

// beholdPayable is the window gate's Behold arm: every part has its count of
// candidates (reserved across parts, so one object cannot pay two) and the
// rest of the cost is, WHOLE, a priceable mana/life half the pool covers
// right now (the same pricing the pay arm charges).
func (tc *triggeredEffectCost) beholdPayable(e *Engine, rest Cost) bool {
	other := rest
	other.Behold = nil
	if !other.PureManaLife() {
		return false
	}
	var reserved []state.ObjID
	for _, part := range rest.Behold {
		if part.N <= 0 {
			return false
		}
		cands := tc.beholdCandidates(e, part.Spec, reserved)
		if int32(len(cands)) < part.N {
			return false
		}
		reserved = append(reserved, cands[:part.N]...)
	}
	return e.triggeredCostManaHalfPayable(tc, other)
}

// beholdAsk walks the cost's Behold parts in order, recording a part whose
// candidates are exactly its count and posing the "beholdcost" pick
// otherwise. It reports whether an ask was posed; the answer records the
// picks and re-enters the pay arm. A part that can no longer be paid stops
// the walk unasked, and beholdSettle declines it.
func (tc *triggeredEffectCost) beholdAsk(e *Engine, announced Cost) bool {
	for tc.beholdIdx < len(announced.Behold) {
		part := announced.Behold[tc.beholdIdx]
		cands := tc.beholdCandidates(e, part.Spec, tc.beholds)
		if int32(len(cands)) < part.N {
			return false
		}
		if int32(len(cands)) == part.N {
			tc.beholds = append(tc.beholds, cands...)
			tc.beholdIdx++
			continue
		}
		d := &decision.Decision{Player: tc.player, Kind: decision.KChoose, Min: int(part.N), Max: int(part.N),
			Prompt: "Choose permanents or cards to behold", Source: tc.source}
		for _, id := range cands {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "beholdcost", Obj: id, Label: e.targetName(id)})
		}
		windowAsk(e, d, chooseTriggeredCost)
		return true
	}
	return false
}

// beholdAnswer records one "beholdcost" pick: every chosen object must be a
// live candidate of the current part, or the window declines (nothing has
// been charged yet to un-pay).
func (tc *triggeredEffectCost) beholdAnswer(e *Engine, chosen []decision.Option) bool {
	announced := tc.announcedCost()
	if tc.beholdIdx >= len(announced.Behold) || len(chosen) != int(announced.Behold[tc.beholdIdx].N) {
		return false
	}
	cands := tc.beholdCandidates(e, announced.Behold[tc.beholdIdx].Spec, tc.beholds)
	picks := make([]state.ObjID, 0, len(chosen))
	for _, opt := range chosen {
		if !slices.Contains(cands, opt.Obj) || slices.Contains(picks, opt.Obj) {
			return false
		}
		picks = append(picks, opt.Obj)
	}
	tc.beholds = append(tc.beholds, picks...)
	tc.beholdIdx++
	return true
}

// beholdSettle charges the cost: every part must be settled and every beheld
// object still a candidate, then the mana/life half from the pool (any
// energy beside it), then the one public "beheld ... as a cost" Note carrying
// the ids (the cast flow's EmitChoiceCosts and the unless flow's announcement).
func (tc *triggeredEffectCost) beholdSettle(e *Engine, announced Cost) bool {
	if tc.beholdIdx < len(announced.Behold) {
		return false
	}
	var live []state.ObjID
	for _, part := range announced.Behold {
		live = append(live, tc.beholdCandidates(e, part.Spec, nil)...)
	}
	for _, id := range tc.beholds {
		if !slices.Contains(live, id) {
			return false
		}
	}
	lower := announced.WithoutEnergy()
	lower.Behold = nil
	if !lower.PureManaLife() || !pay.PayManaConv(asPayer(e), tc.player, lower, asEval(e).Conv(tc.player, tc.source, pay.SAOther)) {
		return false
	}
	pay.ChargeEnergyCost(asPayer(e), tc.player, tc.amount, tc.xPaid)
	names := ""
	for i, id := range tc.beholds {
		if i > 0 {
			names += ", "
		}
		names += e.targetName(id)
	}
	e.emit(events.Event{Kind: events.Note, Player: tc.player, Obj: tc.source,
		IDs:  append([]state.ObjID(nil), tc.beholds...),
		Text: "beheld " + names + " as a cost"})
	return true
}
