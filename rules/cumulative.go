package rules

import (
	"strconv"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Cumulative upkeep is expanded by cards into an ordinary beginning-of-upkeep
// Phase trigger. Consequently it is ordered and placed with every other upkeep
// trigger and can be responded to or countered before it resolves (CR 702.46a).
// This file owns only that trigger's resolution-time age counter and payment
// window. triggeredEffectCost is the sibling window for a Cost$ on a normal
// trigger effect, notably Mana Vault's optional pay-{4} Untap.
const (
	chooseCumulative    chooseFor = chooseManaDiscard + 1
	chooseTriggeredCost chooseFor = chooseManaDiscard + 2
)

type cumulativeUpkeep struct {
	stackObj   state.ObjID
	source     state.ObjID
	player     state.PlayerID
	amount     Cost
	costLabel  string
	windowDone bool
}

type triggeredEffectCost struct {
	resume     *resumePoint
	source     state.ObjID
	player     state.PlayerID
	amount     Cost
	costLabel  string
	windowDone bool
}

// scaleCost repeats every payable component once per age counter.
func scaleCost(c Cost, n int32) Cost {
	if n <= 1 {
		return c
	}
	out := Cost{}
	for i := range c.Colored {
		out.Colored[i] = c.Colored[i] * n
	}
	out.Generic, out.Life = c.Generic*n, c.Life*n
	for i := int32(0); i < n; i++ {
		out.Hybrid = append(out.Hybrid, c.Hybrid...)
		out.Phyrexian = append(out.Phyrexian, c.Phyrexian...)
	}
	out.Tap, out.X = c.Tap, c.X
	out.Sac = append(out.Sac, c.Sac...)
	out.Discard = append(out.Discard, c.Discard...)
	out.SubCounter = append(out.SubCounter, c.SubCounter...)
	out.AddCounter = append(out.AddCounter, c.AddCounter...)
	return out
}

// startCumulativeUpkeep starts resolution of the already-stacked keyword
// trigger. No age counter exists before this point: responses and another
// simultaneous upkeep trigger therefore observe the pre-resolution value.
func (e *Engine) startCumulativeUpkeep(stackObj, source state.ObjID, sa *cards.SA) {
	o := e.G.Obj(source)
	if o == nil || o.Zone != state.ZBattlefield {
		e.finishResumption(stackObj)
		return
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: source, Counter: "AGE", Amount: 1})
	label := sa.Params["Cost"]
	e.cumulative = &cumulativeUpkeep{
		stackObj: stackObj, source: source, player: o.Controller,
		amount: scaleCost(ParseCost(label), o.Counter("AGE")), costLabel: label,
	}
	e.cumulativePaymentAsk()
}

// startTriggeredEffectCost parks a normal trigger effect before it runs and
// opens the same mana-ability-only payment window used by cumulative upkeep.
func (e *Engine) startTriggeredEffectCost(rp *resumePoint, source state.ObjID) {
	o := e.G.Obj(rp.obj)
	if o == nil || o.Zone != state.ZStack || rp.sa == nil {
		return
	}
	label := rp.sa.Params["Cost"]
	e.triggerCost = &triggeredEffectCost{resume: rp, source: source,
		player: o.Controller, amount: ParseCost(label), costLabel: label}
	e.triggeredCostPaymentAsk()
}

// paymentWindowAsk is the continuation mana activation calls after it adds
// mana. Only one resolution-time payment can be live on the single stack.
func (e *Engine) paymentWindowAsk() {
	if e.triggerCost != nil {
		e.triggeredCostPaymentAsk()
		return
	}
	e.cumulativePaymentAsk()
}

func (e *Engine) paymentManaAsk(player state.PlayerID, source state.ObjID, amount Cost, windowDone bool, prompt string, flow chooseFor) bool {
	if windowDone || !amount.Priceable() || e.costPayable(player, source, false, amount) {
		return false
	}
	var sources []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, player) {
		if e.untappedManaSource(player, id) {
			sources = append(sources, id)
		}
	}
	if len(sources) == 0 {
		return false
	}
	d := &decision.Decision{Player: player, Kind: decision.KChoose, Min: 1, Max: 1, Prompt: prompt, Source: source}
	for _, id := range sources {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "activate", Obj: id,
			Label: "Tap " + e.G.Obj(id).Face().Name + " for mana"})
	}
	d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "done", Label: "Done"})
	e.choosing = flow
	e.ask(d)
	return true
}

func (e *Engine) cumulativePaymentAsk() {
	cu := e.cumulative
	if cu == nil {
		return
	}
	o := e.G.Obj(cu.source)
	if o == nil || o.Zone != state.ZBattlefield {
		e.finishCumulative()
		return
	}
	if e.paymentManaAsk(cu.player, cu.source, cu.amount, cu.windowDone,
		"Activate mana abilities to pay cumulative upkeep", chooseCumulative) {
		return
	}
	age := strconv.FormatInt(int64(o.Counter("AGE")), 10)
	var opts []decision.Option
	if cu.amount.Priceable() && e.costPayable(cu.player, cu.source, false, cu.amount) {
		opts = append(opts, decision.Option{Index: 0, Kind: "cumulative_pay", Obj: cu.source,
			Label: "Pay " + cu.costLabel + " per age (" + age + " age counter(s))"})
	}
	opts = append(opts, decision.Option{Index: len(opts), Kind: "cumulative_sac", Obj: cu.source,
		Label: "Sacrifice " + o.Face().Name})
	e.choosing = chooseCumulative
	e.ask(&decision.Decision{Player: cu.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: o.Face().Name + " — cumulative upkeep: pay or sacrifice", Source: cu.source, Options: opts})
}

func (e *Engine) triggeredCostPaymentAsk() {
	tc := e.triggerCost
	if tc == nil {
		return
	}
	if e.paymentManaAsk(tc.player, tc.source, tc.amount, tc.windowDone,
		"Activate mana abilities to pay "+tc.costLabel, chooseTriggeredCost) {
		return
	}
	name := "triggered ability"
	if o := e.G.Obj(tc.source); o != nil && o.Face() != nil {
		name = o.Face().Name
	}
	opts := []decision.Option{{Index: 0, Kind: "trigger_cost_pay", Obj: tc.source, Label: "Pay " + tc.costLabel},
		{Index: 1, Kind: "trigger_cost_decline", Obj: tc.source, Label: "Do not pay"}}
	e.choosing = chooseTriggeredCost
	e.ask(&decision.Decision{Player: tc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: name + " — pay " + tc.costLabel + "?", Source: tc.source, Options: opts})
}

func (e *Engine) cumulativeAnswer(chosen []decision.Option) {
	cu := e.cumulative
	if cu == nil || len(chosen) == 0 {
		return
	}
	e.choosing = chooseNone
	switch chosen[0].Kind {
	case "activate":
		e.activatePaymentMana(cu.player, chosen[0].Obj)
		return
	case "done":
		cu.windowDone = true
		e.cumulativePaymentAsk()
		return
	}
	paid := chosen[0].Kind == "cumulative_pay" && cu.amount.Priceable() &&
		e.payManaConv(cu.player, cu.amount, e.paymentConv(cu.player, cu.source, false))
	if !paid {
		if o := e.G.Obj(cu.source); o != nil && o.Zone == state.ZBattlefield {
			e.emit(events.Event{Kind: events.MoveZone, Obj: cu.source, From: state.ZBattlefield,
				To: state.ZGraveyard, Text: "sacrificed for cumulative upkeep"})
		}
	}
	e.finishCumulative()
}

func (e *Engine) finishCumulative() {
	cu := e.cumulative
	e.cumulative = nil
	e.choosing = chooseNone
	if cu == nil {
		return
	}
	e.finishResumption(cu.stackObj)
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
}

func (e *Engine) triggeredCostAnswer(chosen []decision.Option) {
	tc := e.triggerCost
	if tc == nil || len(chosen) == 0 {
		return
	}
	e.choosing = chooseNone
	switch chosen[0].Kind {
	case "activate":
		e.activatePaymentMana(tc.player, chosen[0].Obj)
		return
	case "done":
		tc.windowDone = true
		e.triggeredCostPaymentAsk()
		return
	}
	paid := chosen[0].Kind == "trigger_cost_pay" && tc.amount.Priceable() &&
		e.payManaConv(tc.player, tc.amount, e.paymentConv(tc.player, tc.source, false))
	rp := tc.resume
	e.triggerCost = nil
	if paid {
		rp.kind = "effect_paid"
		e.resumeResolution(rp, nil)
		return
	}
	if rp.outer != nil {
		e.resumeResolution(rp.outer, nil)
		return
	}
	e.finishResumption(rp.obj)
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
}

func init() {
	effects.RegisterNonAPI("kw:Cumulative upkeep", "stat:UntapOtherPlayer")
}
