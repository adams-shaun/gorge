package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// chorus_cost.go is Chorus of the Conclave's static: "As an additional cost to
// cast creature spells, you may pay any amount of mana. If you do, that
// creature enters with that many additional +1/+1 counters on it." (CR 601.2b,
// 118.8). The carrier is a battlefield permanent, the payer is its controller
// casting ANY creature spell, and the amount is a free integer announced
// before payment; the amount paid rides the cast's AddsCounters grants
// (pay.AppendAnyAmountGrant) into the spell's battlefield entry.

// chorusKeywordLine is the exact sentence the corpus prints as the bare K:
// line (the keyword reader keeps the whole sentence as the head).
const chorusKeywordLine = "As an additional cost to cast creature spells, you may pay any amount of mana. If you do, that creature enters with that many additional +1/+1 counters on it."

// chorusAnswerKind is the option kind the amount decision carries.
const chorusAnswerKind = "chorus"

// chorusInEffect reports whether p controls a battlefield permanent whose
// current (layer-derived) keywords, read through derivedKeywords, still carry
// the static. It takes the game and a keyword reader, not the engine.
func chorusInEffect(g *state.Game, p state.PlayerID, derivedKeywords func(state.ObjID) []string) bool {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil || !printsChorusLine(o.Face().Keywords) {
			continue
		}
		if printsChorusLine(derivedKeywords(id)) {
			return true
		}
	}
	return false
}

func printsChorusLine(kws []string) bool {
	for _, k := range kws {
		if strings.TrimSpace(k) == chorusKeywordLine {
			return true
		}
	}
	return false
}

// preCostAsks runs the additional-cost count asks of the cast announcement in
// order (squad, then Chorus's pay-any-amount), reporting whether one posed a
// decision. One stage keeps continueCast's branch count flat.
func preCostAsks(e *Engine) bool {
	return e.squadAsk() || chorusAsk(e)
}

// chorusAsk poses the amount question once per creature-spell cast made while
// the static applies (a special action -- suspend, plot, foretell, a land
// drop -- casts no spell, CR 116.2, so it is never asked): how much extra mana to pay (0 declines), bounded by the
// most the caster can still pay on top of the pending cost (the same
// affordability walk the payment window's composed total faces, so the offer
// and the charge agree). With no applicable static, or nothing more payable,
// no decision is posed.
func chorusAsk(e *Engine) bool {
	pc := e.cast
	if pc == nil || pc.chorusDone || pc.isAbility() || pc.isSpecialAction() {
		return false
	}
	pc.chorusDone = true
	derivedKeywords := func(id state.ObjID) []string { return e.Derived(id).Keywords }
	if !chorusInEffect(e.G, pc.player, derivedKeywords) || !e.IsCreature(pc.card) {
		return false
	}
	max := pay.MaxAnyAmount(func(n int32) bool {
		cand := pc.cost
		cand.Generic += n
		return e.countCandPayable(pc, cand)
	})
	if max == 0 {
		return false
	}
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Pay any amount of mana: the creature enters with that many +1/+1 counters?", Source: pc.card}
	for n := int32(0); n <= max; n++ {
		label := "Pay no extra mana"
		if n > 0 {
			label = "Pay " + formatCost(Cost{Generic: n}) + " extra mana"
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: chorusAnswerKind,
			Label: label, Amount: int(n)})
	}
	e.choosing = chooseCast
	e.ask(d)
	return true
}

// answerChorus folds the answered amount into the pending cost as generic
// mana and records it for the pay-time grant.
func answerChorus(pc *pendingCast, chosen []decision.Option) {
	if len(chosen) == 0 || chosen[0].Amount <= 0 {
		return
	}
	n := int32(chosen[0].Amount)
	pc.cost.Generic += n
	pc.chorusPaid += n
}
