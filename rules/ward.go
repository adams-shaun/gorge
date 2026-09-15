package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

var wardSpecialCost = regexp.MustCompile(`^(AddCounterYou|Blight|CollectEvidence|Waterbend)<([0-9]+)(?:/[^>]*)?>$`)

// beginWardPayment handles the cost after the targeting object's controller
// selected "Pay". It either pays immediately or poses the one object-choice
// decision the corpus's non-mana Ward costs require.
func (e *Engine) beginWardPayment(rp *resumePoint, ctx *effects.Ctx) (paid, asked bool) {
	payer, ok := e.wardPayer(ctx)
	if !ok {
		return false, false
	}
	raw := strings.TrimSpace(rp.sa.Params["UnlessCost"])

	// Titania's Forge spelling uses the suffix after ':' as an alternative:
	// discard one card OR pay {2}.
	if i := strings.LastIndex(raw, ">:"); i >= 0 {
		discardRaw, manaRaw := raw[:i+1], raw[i+2:]
		cost := ParseCost(discardRaw)
		if len(cost.Discard) != 1 {
			return false, false
		}
		part := cost.Discard[0]
		candidates := e.discardCandidates(payer, ctx.Source, part, false, nil)
		d := &decision.Decision{Player: payer, Kind: decision.KChoose, Min: 1, Max: 1,
			Prompt: "Choose how to pay ward", ResumeKind: "ward_alt", ResumeSA: rp.sa}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "ward_discard", Obj: id,
				Label: "Discard " + e.G.Obj(id).Face().Name})
		}
		mana := ParseCost(manaRaw)
		if mana.payable(e.G.Players[payer].Pool, e.G.Players[payer].Life) {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "ward_mana", Amount: int(mana.Generic), Label: "Pay " + manaRaw})
		}
		if len(d.Options) == 0 {
			return false, false
		}
		e.Ask(d)
		e.resume.outer = rp.outer
		return false, true
	}

	if m := wardSpecialCost.FindStringSubmatch(raw); m != nil {
		n, _ := strconv.Atoi(m[2])
		switch m[1] {
		case "AddCounterYou":
			e.emit(events.Event{Kind: events.PlayerCounterChange, Player: payer, Counter: "POISON", Amount: int32(n)})
			return true, false
		case "Blight":
			return e.askWardObjects(rp, payer, "ward_blight", "Choose a creature to blight", 1, 1,
				e.wardPermanents(payer, ctx.Source, "Creature", true))
		case "CollectEvidence":
			ids := append([]state.ObjID(nil), e.G.Zone(state.ZGraveyard, payer)...)
			if wardManaValue(e.G, ids) < int32(n) {
				return false, false
			}
			return e.askWardObjects(rp, payer, "ward_evidence", "Exile evidence with total mana value "+strconv.Itoa(n), 1, len(ids), ids)
		case "Waterbend":
			need := n - int(e.G.Players[payer].Pool.Total())
			if need < 0 {
				need = 0
			}
			if need == 0 {
				return e.payMana(payer, Cost{Generic: int32(n)}), false
			}
			ids := e.wardPermanents(payer, ctx.Source, "Artifact,Creature", true)
			if len(ids) < need {
				return false, false
			}
			return e.askWardObjects(rp, payer, "ward_waterbend", "Tap permanents to waterbend", need, need, ids)
		}
	}

	cost := ParseCost(raw)
	if strings.HasPrefix(raw, "PayLife<X/") {
		life := e.Power(ctx.Source)
		if life < 0 {
			life = 0
		}
		cost = Cost{Life: life}
	}
	if len(cost.Sac) > 1 || len(cost.Discard) > 1 || (len(cost.Sac) > 0 && len(cost.Discard) > 0) || cost.X != 0 {
		return false, false
	}
	if len(cost.Sac) == 1 {
		part := cost.Sac[0]
		ids := e.wardPermanents(payer, ctx.Source, sacrificeMatchSpec(part.Spec), false)
		if int32(len(ids)) < part.N {
			return false, false
		}
		return e.askWardObjects(rp, payer, "ward_sac", "Choose permanents to sacrifice for ward", int(part.N), int(part.N), ids)
	}
	if len(cost.Discard) == 1 {
		part := cost.Discard[0]
		ids := e.discardCandidates(payer, ctx.Source, part, false, nil)
		if int32(len(ids)) < part.N {
			return false, false
		}
		if strings.EqualFold(part.Spec, "Random") {
			chosen := make([]state.ObjID, 0, part.N)
			for i := int32(0); i < part.N; i++ {
				pick := e.Rand(len(ids))
				chosen = append(chosen, ids[pick])
				ids = append(ids[:pick], ids[pick+1:]...)
			}
			if !e.payMana(payer, wardManaCost(cost)) {
				return false, false
			}
			for _, id := range chosen {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard, Text: "discarded for ward"})
			}
			return true, false
		}
		return e.askWardObjects(rp, payer, "ward_discard", "Choose cards to discard for ward", int(part.N), int(part.N), ids)
	}
	if cost.Tap {
		ids := e.wardPermanents(payer, ctx.Source, "Artifact,Creature", true)
		if len(ids) == 0 {
			return false, false
		}
		return e.askWardObjects(rp, payer, "ward_tap", "Choose a permanent to tap for ward", 1, 1, ids)
	}
	return e.payMana(payer, cost), false
}

func (e *Engine) askWardObjects(rp *resumePoint, payer state.PlayerID, kind, prompt string, min, max int, ids []state.ObjID) (bool, bool) {
	d := &decision.Decision{Player: payer, Kind: decision.KChoose, Min: min, Max: max,
		Prompt: prompt, ResumeKind: kind, ResumeSA: rp.sa}
	for _, id := range ids {
		o := e.G.Obj(id)
		label := fmt.Sprintf("object #%d", id)
		if o != nil && o.Face() != nil {
			label = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: kind, Obj: id, Label: label})
	}
	e.Ask(d)
	e.resume.outer = rp.outer
	return false, true
}

func (e *Engine) wardPayer(ctx *effects.Ctx) (state.PlayerID, bool) {
	cause := e.G.Obj(ctx.TriggerStack)
	if cause == nil || cause.Zone != state.ZStack || int(cause.Controller) >= len(e.G.Players) {
		return 0, false
	}
	return cause.Controller, true
}

func (e *Engine) wardPermanents(p state.PlayerID, source state.ObjID, spec string, untapped bool) []state.ObjID {
	var out []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o == nil || (untapped && o.Tapped) || !effects.MatchesSpecFrom(e.G, spec, id, p, source) {
			continue
		}
		out = append(out, id)
	}
	return out
}

func wardManaValue(g *state.Game, ids []state.ObjID) int32 {
	var n int32
	for _, id := range ids {
		if o := g.Obj(id); o != nil && o.Face() != nil {
			n += o.Face().Cmc()
		}
	}
	return n
}

// settleWardPayment validates and applies the object choice from a non-mana
// Ward payment. A malformed/stale answer pays nothing and therefore declines.
func (e *Engine) settleWardPayment(kind string, sa *cards.SA, ctx *effects.Ctx, chosen []decision.Option) bool {
	payer, ok := e.wardPayer(ctx)
	if !ok {
		return false
	}
	raw := strings.TrimSpace(sa.Params["UnlessCost"])
	ids := make([]state.ObjID, 0, len(chosen))
	seen := map[state.ObjID]bool{}
	for _, opt := range chosen {
		if opt.Obj != 0 && !seen[opt.Obj] {
			seen[opt.Obj] = true
			ids = append(ids, opt.Obj)
		}
	}

	switch kind {
	case "ward_alt":
		if len(chosen) != 1 {
			return false
		}
		if chosen[0].Kind == "ward_mana" {
			_, manaRaw, _ := strings.Cut(raw, ">:")
			return e.payMana(payer, ParseCost(manaRaw))
		}
		if chosen[0].Kind != "ward_discard" || len(ids) != 1 || !containsObj(e.G.Zone(state.ZHand, payer), ids[0]) {
			return false
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: ids[0], From: state.ZHand, To: state.ZGraveyard, Text: "discarded for ward"})
		return true
	case "ward_blight":
		if len(ids) != 1 || !containsObj(e.wardPermanents(payer, ctx.Source, "Creature", true), ids[0]) {
			return false
		}
		m := wardSpecialCost.FindStringSubmatch(raw)
		n, _ := strconv.Atoi(m[2])
		e.emit(events.Event{Kind: events.CounterChange, Obj: ids[0], Counter: "M1M1", Amount: int32(n)})
		return true
	case "ward_evidence":
		for _, id := range ids {
			if !containsObj(e.G.Zone(state.ZGraveyard, payer), id) {
				return false
			}
		}
		m := wardSpecialCost.FindStringSubmatch(raw)
		n, _ := strconv.Atoi(m[2])
		if wardManaValue(e.G, ids) < int32(n) {
			return false
		}
		for _, id := range ids {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZExile, Text: "collected as evidence for ward"})
		}
		return true
	case "ward_waterbend":
		for _, id := range ids {
			if !containsObj(e.wardPermanents(payer, ctx.Source, "Artifact,Creature", true), id) {
				return false
			}
		}
		m := wardSpecialCost.FindStringSubmatch(raw)
		n, _ := strconv.Atoi(m[2])
		remaining := n - len(ids)
		if remaining < 0 || !e.payMana(payer, Cost{Generic: int32(remaining)}) {
			return false
		}
		for _, id := range ids {
			e.emit(events.Event{Kind: events.Tap, Obj: id})
		}
		return true
	case "ward_tap":
		if len(ids) != 1 || !containsObj(e.wardPermanents(payer, ctx.Source, "Artifact,Creature", true), ids[0]) {
			return false
		}
		e.emit(events.Event{Kind: events.Tap, Obj: ids[0]})
		return true
	case "ward_sac", "ward_discard":
		cost := ParseCost(raw)
		var part CostPart
		var zone state.Zone
		if kind == "ward_sac" && len(cost.Sac) == 1 {
			part, zone = cost.Sac[0], state.ZBattlefield
		} else if kind == "ward_discard" && len(cost.Discard) == 1 {
			part, zone = cost.Discard[0], state.ZHand
		} else {
			return false
		}
		if len(ids) != int(part.N) {
			return false
		}
		for _, id := range ids {
			valid := containsObj(e.G.Zone(zone, payer), id)
			if kind == "ward_sac" {
				valid = valid && effects.MatchesSpecFrom(e.G, sacrificeMatchSpec(part.Spec), id, payer, ctx.Source)
			} else {
				valid = valid && containsObj(e.discardCandidates(payer, ctx.Source, part, false, nil), id)
			}
			if !valid {
				return false
			}
		}
		if !e.payMana(payer, wardManaCost(cost)) {
			return false
		}
		toText := "discarded for ward"
		if kind == "ward_sac" {
			toText = "sacrificed for ward"
		}
		for _, id := range ids {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: zone, To: state.ZGraveyard, Text: toText})
		}
		return true
	}
	return false
}

// wardManaCost removes payment components already settled by a Ward choice.
// payMana deliberately rejects Sac/Discard/Tap, but a Ward cost can combine
// one with mana and must charge that mana after its object component.
func wardManaCost(c Cost) Cost {
	c.Tap = false
	c.Sac = nil
	c.Discard = nil
	return c
}

func containsObj(ids []state.ObjID, want state.ObjID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
