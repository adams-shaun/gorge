package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Cumulative upkeep is expanded by cards into an ordinary beginning-of-upkeep
// Phase trigger. Consequently it is ordered and placed with every other upkeep
// trigger and can be responded to or countered before it resolves (CR 702.46a).
// This file owns that trigger's resolution-time age counter and payment window.
// triggeredEffectCost is the narrower sibling window for Mana Vault's Cost$-
// bearing triggered Untap effect.
const (
	chooseCumulative    chooseFor = chooseManaDiscard + 1
	chooseTriggeredCost chooseFor = chooseManaDiscard + 2
)

type cumulativeAction struct {
	kind string
	n    int32
	spec string
}

type cumulativeUpkeep struct {
	stackObj   state.ObjID
	source     state.ObjID
	player     state.PlayerID
	amount     Cost
	action     *cumulativeAction
	costLabel  string
	windowDone bool

	// actionRemaining counts repeated action payments still to make. Most
	// action costs ask once per age counter; object sacrifices/discards ask
	// for the full scaled set at once, while PutCardToLibFromSameGrave uses
	// actionOwner between its graveyard and card-selection asks.
	actionRemaining int32
	actionOwner     state.PlayerID
}

type triggeredEffectCost struct {
	resume     *resumePoint
	source     state.ObjID
	player     state.PlayerID
	amount     Cost
	costLabel  string
	windowDone bool
}

// scaleCost repeats every mana/life payment component once per age counter.
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
	return out
}

// parseCumulativeAction recognizes the complete non-mana/action vocabulary in
// the pinned corpus. These are actions performed once per age counter, not
// malformed mana symbols; keeping the parser local prevents ordinary Cost$
// callers from acquiring cumulative-upkeep-only semantics.
func parseCumulativeAction(label string) (*cumulativeAction, bool) {
	label = strings.TrimSpace(label)
	open := strings.IndexByte(label, '<')
	if open <= 0 || !strings.HasSuffix(label, ">") {
		return nil, false
	}
	kind := label[:open]
	fields := strings.Split(label[open+1:len(label)-1], "/")
	if len(fields) == 0 {
		return nil, false
	}
	n64, err := strconv.ParseInt(fields[0], 10, 32)
	if err != nil || n64 <= 0 {
		return nil, false
	}
	a := &cumulativeAction{kind: kind, n: int32(n64)}
	switch kind {
	case "Sac", "Discard":
		if len(fields) < 2 {
			return nil, false
		}
		a.spec = fields[1]
	case "AddCounter":
		if len(fields) < 2 {
			return nil, false
		}
		a.spec = fields[1]
		if len(fields) >= 3 {
			a.spec += "/" + fields[2]
		}
	case "AddMana":
		if len(fields) < 2 || len(fields[1]) != 1 || !strings.ContainsRune("WUBRGC", rune(fields[1][0])) {
			return nil, false
		}
		a.spec = fields[1]
	case "Draw":
		if len(fields) < 2 || fields[1] != "You" {
			return nil, false
		}
	case "ExileFromTop":
		if len(fields) < 2 || fields[1] != "Card" {
			return nil, false
		}
	case "FlipCoin":
	case "GainControl":
		if len(fields) < 2 {
			return nil, false
		}
		a.spec = fields[1]
	case "GainLife":
		if len(fields) < 2 || fields[1] != "Player.Opponent" {
			return nil, false
		}
	case "PutCardToLibFromSameGrave":
		if len(fields) < 3 || fields[2] != "Card" {
			return nil, false
		}
		a.spec = fields[2]
	default:
		return nil, false
	}
	return a, true
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
	action, actionOK := parseCumulativeAction(label)
	cu := &cumulativeUpkeep{stackObj: stackObj, source: source, player: o.Controller,
		amount: scaleCost(ParseCost(label), o.Counter("AGE")), costLabel: label,
		actionRemaining: o.Counter("AGE")}
	if actionOK {
		cu.action = action
		cu.amount = Cost{}
	}
	e.cumulative = cu
	e.cumulativePaymentAsk()
}

// startTriggeredEffectCost parks Mana Vault's triggered Untap before it runs
// and opens the same mana-ability-only payment window used by mana cumulative
// upkeep. Callers gate this helper on API == Untap.
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
	if cu.action == nil && e.paymentManaAsk(cu.player, cu.source, cu.amount, cu.windowDone,
		"Activate mana abilities to pay cumulative upkeep", chooseCumulative) {
		return
	}
	age := strconv.FormatInt(int64(o.Counter("AGE")), 10)
	var opts []decision.Option
	payable := cu.action != nil && e.cumulativeActionPayable(cu)
	if cu.action == nil {
		payable = cu.amount.Priceable() && e.costPayable(cu.player, cu.source, false, cu.amount)
	}
	if payable {
		opts = append(opts, decision.Option{Index: 0, Kind: "cumulative_pay", Obj: cu.source,
			Label: "Pay " + cu.costLabel + " per age (" + age + " age counter(s))"})
	}
	opts = append(opts, decision.Option{Index: len(opts), Kind: "cumulative_sac", Obj: cu.source,
		Label: "Sacrifice " + o.Face().Name})
	e.choosing = chooseCumulative
	e.ask(&decision.Decision{Player: cu.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: o.Face().Name + " — cumulative upkeep: pay or sacrifice", Source: cu.source, Options: opts})
}

func (e *Engine) cumulativeObjects(cu *cumulativeUpkeep, zone state.Zone, spec string) []state.ObjID {
	var out []state.ObjID
	players := []state.PlayerID{cu.player}
	if zone == state.ZBattlefield && (strings.Contains(spec, "OppCtrl") || strings.Contains(spec, "YouDontCtrl")) {
		players = e.G.AliveFrom(0)
	}
	if zone == state.ZGraveyard {
		players = e.G.AliveFrom(0)
	}
	for _, p := range players {
		for _, id := range e.G.Zone(zone, p) {
			if effects.MatchesSpecFrom(e.G, spec, id, cu.player, cu.source) {
				out = append(out, id)
			}
		}
	}
	return out
}

func (e *Engine) cumulativeActionPayable(cu *cumulativeUpkeep) bool {
	a := cu.action
	if a == nil || cu.actionRemaining <= 0 {
		return false
	}
	total := int(a.n * cu.actionRemaining)
	switch a.kind {
	case "Sac":
		return len(e.cumulativeObjects(cu, state.ZBattlefield, a.spec)) >= total
	case "Discard":
		return len(e.cumulativeObjects(cu, state.ZHand, a.spec)) >= total
	case "Draw", "ExileFromTop":
		return len(e.G.Zone(state.ZLibrary, cu.player)) >= total
	case "AddMana", "FlipCoin":
		return true
	case "AddCounter":
		if !strings.Contains(a.spec, "/") {
			return e.G.Obj(cu.source) != nil
		}
		_, targetSpec, _ := strings.Cut(a.spec, "/")
		return len(e.cumulativeObjects(cu, state.ZBattlefield, targetSpec)) > 0
	case "GainControl":
		return len(e.cumulativeObjects(cu, state.ZBattlefield, a.spec)) >= total
	case "GainLife":
		return len(e.G.AliveFrom(cu.player)) > 1
	case "PutCardToLibFromSameGrave":
		groups := 0
		for _, p := range e.G.AliveFrom(0) {
			groups += len(e.G.Zone(state.ZGraveyard, p)) / int(a.n)
		}
		return groups >= int(cu.actionRemaining)
	}
	return false
}

func (e *Engine) cumulativeObjectDecision(cu *cumulativeUpkeep, ids []state.ObjID, min, max int, kind, prompt string) {
	d := &decision.Decision{Player: cu.player, Kind: decision.KChoose, Min: min, Max: max,
		Prompt: prompt, Source: cu.source}
	for _, id := range ids {
		label := "card"
		if o := e.G.Obj(id); o != nil && o.Face() != nil {
			label = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: kind, Obj: id, Label: label})
	}
	e.choosing = chooseCumulative
	e.ask(d)
}

func (e *Engine) continueCumulativeAction() {
	cu := e.cumulative
	if cu == nil || cu.action == nil {
		return
	}
	if cu.actionRemaining <= 0 {
		e.finishCumulative()
		return
	}
	a := cu.action
	total := int(a.n * cu.actionRemaining)
	switch a.kind {
	case "Sac":
		e.cumulativeObjectDecision(cu, e.cumulativeObjects(cu, state.ZBattlefield, a.spec), total, total,
			"cumulative_action_sac", "Choose permanents to sacrifice for cumulative upkeep")
	case "Discard":
		e.cumulativeObjectDecision(cu, e.cumulativeObjects(cu, state.ZHand, a.spec), total, total,
			"cumulative_action_discard", "Choose cards to discard for cumulative upkeep")
	case "Draw":
		for i := 0; i < total; i++ {
			effects.DrawFor(e, cu.player)
		}
		cu.actionRemaining = 0
		e.finishCumulative()
	case "ExileFromTop":
		for i := 0; i < total; i++ {
			lib := e.G.Zone(state.ZLibrary, cu.player)
			if len(lib) == 0 {
				break
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: lib[0], From: state.ZLibrary, To: state.ZExile})
		}
		cu.actionRemaining = 0
		e.finishCumulative()
	case "AddMana":
		e.emit(events.Event{Kind: events.ManaAdd, Player: cu.player, Counter: a.spec, Amount: int32(total)})
		cu.actionRemaining = 0
		e.finishCumulative()
	case "FlipCoin":
		for i := 0; i < total; i++ {
			outcome := "tails"
			if e.Rand(2) == 0 {
				outcome = "heads"
			}
			e.emit(events.Event{Kind: events.Note, Player: cu.player, Obj: cu.source, Text: "flips " + outcome})
		}
		cu.actionRemaining = 0
		e.finishCumulative()
	case "AddCounter":
		counter, targetSpec, targeted := strings.Cut(a.spec, "/")
		if !targeted {
			e.emit(events.Event{Kind: events.CounterChange, Obj: cu.source, Counter: counter, Amount: int32(total)})
			cu.actionRemaining = 0
			e.finishCumulative()
			return
		}
		e.cumulativeObjectDecision(cu, e.cumulativeObjects(cu, state.ZBattlefield, targetSpec), 1, 1,
			"cumulative_action_counter", "Choose a permanent to receive a "+counter+" counter")
	case "GainControl":
		e.cumulativeObjectDecision(cu, e.cumulativeObjects(cu, state.ZBattlefield, a.spec), total, total,
			"cumulative_action_control", "Choose permanents to gain control of")
	case "GainLife":
		d := &decision.Decision{Player: cu.player, Kind: decision.KChoose, Min: 1, Max: 1,
			Prompt: "Choose an opponent to gain life", Source: cu.source}
		for _, p := range e.G.AliveFrom(cu.player) {
			if p == cu.player {
				continue
			}
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "cumulative_action_life",
				Player: p, Label: e.G.Players[p].Name})
		}
		e.choosing = chooseCumulative
		e.ask(d)
	case "PutCardToLibFromSameGrave":
		var owners []state.PlayerID
		for _, p := range e.G.AliveFrom(0) {
			if len(e.G.Zone(state.ZGraveyard, p)) >= int(a.n) {
				owners = append(owners, p)
			}
		}
		if len(owners) == 1 {
			cu.actionOwner = owners[0]
			e.cumulativeObjectDecision(cu, e.G.Zone(state.ZGraveyard, owners[0]), int(a.n), int(a.n),
				"cumulative_action_grave_card", "Choose cards from one graveyard")
			return
		}
		d := &decision.Decision{Player: cu.player, Kind: decision.KChoose, Min: 1, Max: 1,
			Prompt: "Choose a graveyard", Source: cu.source}
		for _, p := range owners {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "cumulative_action_grave",
				Player: p, Label: e.G.Players[p].Name + "'s graveyard"})
		}
		e.choosing = chooseCumulative
		e.ask(d)
	}
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
	case "cumulative_pay":
		if cu.action != nil {
			e.continueCumulativeAction()
			return
		}
		if e.payManaConv(cu.player, cu.amount, e.paymentConv(cu.player, cu.source, false)) {
			e.finishCumulative()
			return
		}
	case "cumulative_action_sac":
		for _, option := range chosen {
			if o := e.G.Obj(option.Obj); o != nil && o.Zone == state.ZBattlefield {
				e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZBattlefield,
					To: state.ZGraveyard, Text: "sacrificed for cumulative upkeep"})
			}
		}
		cu.actionRemaining = 0
		e.finishCumulative()
		return
	case "cumulative_action_discard":
		for _, option := range chosen {
			if o := e.G.Obj(option.Obj); o != nil && o.Zone == state.ZHand && o.Owner == cu.player {
				e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZHand,
					To: state.ZGraveyard, Text: "discarded for cumulative upkeep"})
			}
		}
		cu.actionRemaining = 0
		e.finishCumulative()
		return
	case "cumulative_action_counter":
		counter, _, _ := strings.Cut(cu.action.spec, "/")
		e.emit(events.Event{Kind: events.CounterChange, Obj: chosen[0].Obj, Counter: counter, Amount: cu.action.n})
		cu.actionRemaining--
		e.continueCumulativeAction()
		return
	case "cumulative_action_control":
		for _, option := range chosen {
			e.emit(events.Event{Kind: events.ChangeControl, Obj: option.Obj, Player: cu.player})
		}
		cu.actionRemaining = 0
		e.finishCumulative()
		return
	case "cumulative_action_life":
		e.emit(events.Event{Kind: events.LifeChange, Player: chosen[0].Player, Amount: cu.action.n})
		cu.actionRemaining--
		e.continueCumulativeAction()
		return
	case "cumulative_action_grave":
		cu.actionOwner = chosen[0].Player
		e.cumulativeObjectDecision(cu, e.G.Zone(state.ZGraveyard, cu.actionOwner), int(cu.action.n), int(cu.action.n),
			"cumulative_action_grave_card", "Choose cards from one graveyard")
		return
	case "cumulative_action_grave_card":
		for _, option := range chosen {
			if o := e.G.Obj(option.Obj); o != nil && o.Zone == state.ZGraveyard && o.Owner == cu.actionOwner {
				e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZGraveyard, To: state.ZLibrary})
			}
		}
		cu.actionRemaining--
		e.continueCumulativeAction()
		return
	}
	if o := e.G.Obj(cu.source); o != nil && o.Zone == state.ZBattlefield {
		e.emit(events.Event{Kind: events.MoveZone, Obj: cu.source, From: state.ZBattlefield,
			To: state.ZGraveyard, Text: "sacrificed for cumulative upkeep"})
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
