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
// bearing triggered Untap effect. Successive merges kept renumbering these
// two above main's growing chooseFor enum (chooseManaExile, then
// chooseOpening/chooseSuspendCast, now chooseStation/chooseUnlock): chooseFor
// values are transient engine RAM — Clone copies the struct, no event or wire
// field ever carries one — so the two windows move above the current highest.
const (
	chooseCumulative         chooseFor = chooseUnlock + 1
	chooseTriggeredCost      chooseFor = chooseUnlock + 2
	chooseTriggeredMandatory chooseFor = chooseExert + 1
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
	// tapIdx is the next unsettled dynamic tapXType part (rules/mana.go's
	// dynTapCost heads) of amount: the tap election is this window's payment
	// for those parts, and the index keeps a multi-part cost asking in order.
	tapIdx int
	// mandatory marks a `Cost$ Mandatory <...>` body (TrigsMand1): a
	// mandatory cost has no "may" and therefore no pay/decline election, so
	// this window walks its choice-bearing non-mana components (Sac, Exile),
	// poses a real pick where one exists and a no-ask settle where it does
	// not, emits the settled events, and only then runs the parked body. A
	// component that cannot be paid skips the body (Forge: the payment gates
	// the effect). Only the Sac/Exile shapes enter this path
	// (mandatorySettleShape); everything else keeps the free-executor
	// carve-out in triggerBodyNeedsCostWindow.
	mandatory bool
	// part is the cursor into the flat choice-bearing component list
	// (triggeredMandatoryParts: Sac then Exile then Discard) a settle walks.
	part int
	// sacs, exiles and discards accumulate the settled picks (the same
	// reservation list the unless-pay and cast flows keep) so one object
	// cannot pay two parts and the settle can emit the exact events the
	// picks name.
	sacs     []state.ObjID
	exiles   []state.ObjID
	discards []state.ObjID
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
	stack := e.G.Obj(stackObj)
	if o == nil || o.Zone != state.ZBattlefield || stack == nil {
		e.finishResumption(stackObj)
		return
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: source, Counter: "AGE", Amount: 1})
	label := sa.Params["Cost"]
	action, actionOK := parseCumulativeAction(label)
	// The triggered ability's controller was captured when it was placed on
	// the stack. A response may change control of the cumulative permanent,
	// but it must not transfer the already-triggered payment decision.
	cu := &cumulativeUpkeep{stackObj: stackObj, source: source, player: stack.Controller,
		amount: scaleCost(e.parseCost(label), o.Counter("AGE")), costLabel: label,
		actionRemaining: o.Counter("AGE")}
	if actionOK {
		cu.action = action
		cu.amount = Cost{}
	}
	e.cumulative = cu
	e.cumulativePaymentAsk()
}

// triggerBodyNeedsCostWindow reports whether a trigger effect (or an accepted
// optional trigger's body) found through findTriggerForAbility must enter the
// triggered-cost pay/decline window (startTriggeredEffectCost) BEFORE its
// body runs, rather than executing for free. Forge's `Cost$ <cost>` on a
// trigger body is the "you may pay <cost>. If you do, ..." idiom, so ANY
// non-Mandatory Cost$ arms the window -- not just the Untap / ImmediateTrigger /
// Draw / dyn-tap shapes the original allowlist served. The window itself keeps
// the split: a Priceable cost (plain mana, fixed PayLife<N>) offers a real
// "pay"; a cost whose choice-bearing components are exactly Sac/Exile/Discard
// parts and whose draws and mana half resolve offers a real pay and settles
// the components for real (trigcost2, triggeredCostComponentsPayable);
// everything else lands decline-only (the ParseUnlessCost hard-decline
// convention -- never a free execution, never a zero-amount payment).
//
// Carve-outs, each deliberate:
//   - API Mana: mana abilities have their own activation path.
//   - API CopySpellAbility: its window arming needs the trigger context's
//     event role (TriggerAbility/TriggerCard -- a context-less synthetic push
//     keeps the free-executor semantics), so the call sites arm it separately.
//   - a `Mandatory` cost prefix: the mandatory family (Cost$ Mandatory
//     Sac<1/CARDNAME>, PayLife<X>, Exile<...>) is NOT the pay idiom -- its
//     payment is a real non-mana settle rules does not yet run -- and the
//     window could only offer decline-only, which for a mandatory payment
//     would be the WORSE regression (the body would never run AND the payment
//     would never happen). Those bodies keep the established free-executor
//     semantics (follow-up ticket), EXCEPT the shapes the existing gate
//     already served: the Untap/ImmediateTrigger APIs and the dynamic
//     tapXType election (yotia_declares_war's "Mandatory tapXType<X/Artifact>")
//     -- the tap election is the payment there, so the window stays right.
func (e *Engine) triggerBodyNeedsCostWindow(sa *cards.SA) bool {
	if sa == nil || sa.Params["Cost"] == "" || sa.API == "Mana" || sa.API == "CopySpellAbility" {
		return false
	}
	if !strings.HasPrefix(sa.Params["Cost"], "Mandatory") {
		return true
	}
	if sa.API == "Untap" || sa.API == "ImmediateTrigger" {
		return true
	}
	c := e.parseCost(sa.Params["Cost"])
	// The dynamic tapXType heads (rules/mana.go's dynTapCost): the tap
	// election is the payment, the empty election the decline.
	if costCarriesDynTap(c) {
		return true
	}
	// trigmand1: the `Mandatory Sac<...>` / `Mandatory Exile<...>` shapes are
	// now settled for real by the mandatory path in this window (a pick where
	// a choice exists, the exact Sacrifice/Exile event otherwise), so they
	// enter it. Any other mandatory component keeps the free-executor
	// carve-out: a Mandatory PayLife<X>/PayEnergy/replacement body has no
	// settle here (see the AGENTS.md approximation row), and arming it would
	// only land it decline-only -- the WORSE regression.
	return mandatorySettleShape(c)
}

// mandatorySettleShape reports whether a parsed `Mandatory` cost is one the
// mandatory window can fully settle: its non-mana components are exactly the
// choice-bearing Sac/Exile parts (settled with a real pick where a choice
// exists) and everything else it carries is chargeable by payManaConv (plain
// mana / fixed PayLife<N>). Any other component -- PayEnergy, PayLife<X>,
// Discard, Reveal, Behold, SubCounter, a dynamic tap, ... -- returns false so
// the body keeps today's free execution rather than a decline-only ask.
func mandatorySettleShape(c Cost) bool {
	if len(c.Sac) == 0 && len(c.Exile) == 0 {
		return false
	}
	stripped := c
	stripped.Sac = nil
	stripped.Exile = nil
	return stripped.Priceable()
}

// startTriggeredEffectCost parks Mana Vault's triggered Untap before it runs
// and opens the same mana-ability-only payment window used by mana cumulative
// upkeep. Callers gate this helper on triggerBodyNeedsCostWindow.
func (e *Engine) startTriggeredEffectCost(rp *resumePoint, source state.ObjID) {
	o := e.G.Obj(rp.obj)
	if o == nil || o.Zone != state.ZStack || rp.sa == nil {
		return
	}
	label := rp.sa.Params["Cost"]
	e.triggerCost = &triggeredEffectCost{resume: rp, source: source,
		player: o.Controller, amount: e.parseCost(label), costLabel: label,
		mandatory: strings.HasPrefix(label, "Mandatory")}
	if e.triggerCost.mandatory {
		e.advanceTriggeredMandatory(e.triggerCost)
		return
	}
	e.triggeredCostPaymentAsk()
}

// paymentWindowAsk is the continuation mana activation calls after it adds
// mana. Only one resolution-time payment can be live on the single stack.
func (e *Engine) paymentWindowAsk() {
	if e.triggerCost != nil {
		e.triggeredCostPaymentAsk()
		return
	}
	if e.echo != nil {
		// kw:Echo's mana window re-opens after each activated source (the
		// same continuation every other window uses).
		e.echoElectionAsk()
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
		// The canonical coin-flip result Note effects.FlipCoinNote emits —
		// the ONE encoding the FlippedCoin trigger matcher
		// (rules/trigger_match.go's flippedCoinMatches) reads, so a
		// cost-side flip fires "whenever you win/lose a coin flip" exactly
		// like an effect-side one (Karplusan Minotaur).
		for i := 0; i < total; i++ {
			e.emit(effects.FlipCoinNote(cu.source, cu.player, e.Rand(2) == 0))
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
				Player: p, Label: seatFacingName(e.G, p)})
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
				Player: p, Label: seatFacingName(e.G, p) + "'s graveyard"})
		}
		e.choosing = chooseCumulative
		e.ask(d)
	}
}

// triggeredCostDrawCounts resolves every Draw component of a trigger's Cost$
// at the window's source, so the ask can decide whether "pay" is answerable
// and the pay arm can settle the draws without re-deriving them. ok=false
// means a component is unpayable: a drawer spec with no binding in the cast
// flow, or a dynamic Draw<X/Spec> whose source SVar is absent or
// unresolvable (fail closed, the ParseUnlessCost hard-decline convention).
func (e *Engine) triggeredCostDrawCounts(tc *triggeredEffectCost) ([]int32, bool) {
	out := make([]int32, len(tc.amount.Draw))
	for i, part := range tc.amount.Draw {
		if _, ok := castFlowDrawPlayer(part.Spec, tc.player); !ok {
			return nil, false
		}
		n, ok := e.drawCostCount(tc.source, tc.player, part)
		if !ok {
			return nil, false
		}
		out[i] = n
	}
	return out, true
}

// triggeredCostComponentsPayable reports whether the window can settle a
// non-Priceable cost's choice-bearing components for real (trigcost2): every
// component is a Sac/Exile/Discard part (no Announced Sac -- the announced
// count has no channel here), no OTHER non-mana component rides the cost,
// no {X}/tap demand needs an announcement the window cannot make, every
// Draw count resolves, every component is payable at ask time, and the
// mana/life half is chargeable from the pool (pool-only: the window never
// opens a mana-activation ask for a component cost, the same shape the
// unless-pay arm keeps). A cost failing any of these keeps the decline-only
// ask -- a half-paid commitment is unreachable, and so is the silent skip
// this gate replaces.
func (e *Engine) triggeredCostComponentsPayable(tc *triggeredEffectCost) bool {
	amt := tc.amount
	if len(amt.Sac)+len(amt.Discard)+len(amt.Exile) == 0 {
		return false
	}
	if amt.Tap || amt.X != 0 {
		return false
	}
	for _, part := range amt.Sac {
		if part.Announced {
			return false
		}
	}
	// Every component outside the settleable set keeps the decline-only ask
	// (PayEnergy, SubCounter, Return, PutToLib, Behold, Reveal, Forage, the
	// dyn-tap election, a dynamic life/counter value) -- never a half-paid
	// commitment (the ParseUnlessCost hard-decline convention).
	if len(amt.SubCounter) > 0 || len(amt.Reveal) > 0 || len(amt.Behold) > 0 ||
		len(amt.TapPermanent) > 0 || len(amt.Blight) > 0 || len(amt.Energy) > 0 ||
		len(amt.Return) > 0 || len(amt.PutToLib) > 0 || len(amt.LifeX) > 0 ||
		len(amt.DamageYou) > 0 || amt.Forage {
		return false
	}
	if _, ok := e.triggeredCostDrawCounts(tc); !ok {
		return false
	}
	parts := triggeredMandatoryParts(amt)
	for i, part := range parts {
		if triggeredPartIsDiscard(amt, i) && strings.EqualFold(part.Spec, "Hand") {
			// The Hand shape pays the whole hand: payable whenever the hand
			// holds at least the token's count (a Discard<0/Hand> token pays
			// even an empty hand; a Discard<1/Hand> one needs a card).
			if part.N > 0 {
				if n := len(e.discardCandidates(tc.player, tc.source, part, false, nil)); int32(n) < part.N {
					return false
				}
			}
			continue
		}
		if part.N <= 0 {
			// A zero-count ordinary component has no payment to settle; the
			// window declines rather than walking a degenerate ask.
			return false
		}
		eligible := e.triggeredMandatoryCandidates(tc, i, part)
		if int32(len(eligible)) < part.N {
			return false
		}
	}
	mana := amt
	mana.Sac, mana.Discard, mana.Exile, mana.Draw = nil, nil, nil, nil
	if mana.hasManaPayment() || mana.Life > 0 || mana.Snow > 0 ||
		len(mana.Hybrid) > 0 || len(mana.Phyrexian) > 0 ||
		len(mana.Twobrid) > 0 || len(mana.HybridPhyrexian) > 0 {
		if !e.costPayable(tc.player, tc.source, false, mana) {
			return false
		}
	}
	return true
}

func (e *Engine) triggeredCostPaymentAsk() {
	tc := e.triggerCost
	if tc == nil {
		return
	}
	// The dynamic tapXType heads (tapXType<X/Spec>, tapXType<Any/Spec>): the
	// tap election IS their payment, so the window poses it directly instead
	// of the pay/decline ask. It opens only when the non-tap rest is an EMPTY
	// payment -- the corpus's dyn-tap trigger costs are all tap-only, and a
	// cost that also charges mana or life alongside the election would
	// otherwise risk a partially paid commitment -- anything else keeps the
	// decline-only ask below (the ParseUnlessCost hard-decline convention).
	// An empty election answer is the decline: tapping nothing does not pay
	// an Any-form cost, and an X-form cost paid with zero taps runs a body
	// scaled only by X at 0 -- the outcome a decline gives without emitting
	// the no-op body's events.
	if costCarriesDynTap(tc.amount) {
		rest := withoutDynTaps(tc.amount)
		if rest.Priceable() && !rest.hasManaPayment() && rest.Life == 0 && rest.Snow == 0 {
			if e.paymentManaAsk(tc.player, tc.source, rest, tc.windowDone,
				"Activate mana abilities to pay "+tc.costLabel, chooseTriggeredCost) {
				return
			}
			if part, ok := e.nextDynTapPart(tc); ok {
				e.triggeredTapAsk(tc, part)
			}
			return
		}
		// fall through: the unpayable-cost decline-only ask below
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
	// A Draw-bearing cost is Priceable()==false by construction (payMana
	// cannot charge the draw), but it IS payable when every Draw component's
	// count resolves and the mana half -- if any -- is covered. The dynamic
	// Draw<X/Spec> form (Champion of Wits' Cost$ Draw<X/You>) resolves its
	// count from the source's SVar table here, at the window, so an
	// unresolvable body offers decline only.
	payable := tc.amount.Priceable()
	if len(tc.amount.Sac)+len(tc.amount.Discard)+len(tc.amount.Exile) > 0 {
		// A component-bearing cost is payable ONLY through the settle gate
		// (trigcost2): the Draw arm alone would offer "pay" while silently
		// skipping the components -- the defect this gate closes.
		payable = e.triggeredCostComponentsPayable(tc)
	} else if !payable && len(tc.amount.Draw) > 0 {
		_, payable = e.triggeredCostDrawCounts(tc)
	}
	if !payable {
		// An unpriceable cost (PayLife<X>, Verrak, Warped Sengir's copy
		// trigger) is a hard decline per the ParseUnlessCost convention: the
		// ask is still posed and the decision recorded, but "pay" is not an
		// answerable option -- never a free copy through a zero-amount read.
		// Options are renumbered: an ask's option Index must equal its
		// position.
		opts = []decision.Option{{Index: 0, Kind: "trigger_cost_decline", Obj: tc.source, Label: "Do not pay"}}
	}
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
			e.emit(events.Event{Kind: events.ControlChange, Obj: option.Obj, Player: cu.player})
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
	if tc == nil {
		return
	}
	e.choosing = chooseNone
	if len(chosen) == 0 {
		// The empty answer of the Min-0 tap election (the dynamic tapXType
		// window) is the decline: tapping nothing pays nothing. A Min-0 KChoose
		// elsewhere (Dig's take-none) answers the same way, and the guard this
		// branch replaces would have dropped the answer and wedged the window.
		e.triggeredCostDecline(tc)
		return
	}
	switch chosen[0].Kind {
	case "activate":
		e.activatePaymentMana(tc.player, chosen[0].Obj)
		return
	case "done":
		tc.windowDone = true
		e.triggeredCostPaymentAsk()
		return
	case "trigger_cost_tap":
		e.triggeredTapAnswer(tc, chosen)
		return
	}
	paid := false
	if chosen[0].Kind == "trigger_cost_pay" {
		if len(tc.amount.Sac)+len(tc.amount.Discard)+len(tc.amount.Exile) > 0 {
			// The settleable-component cost (trigcost2): the election is
			// "pay"; walk the components (the mandatory walk's picks -- a
			// real KChoose where a choice exists, the Hand/Random discard
			// specials without one) and settle. The walk ends in
			// settleTriggeredMandatory, which charges the stripped mana half,
			// emits the component events, executes the draws and resumes the
			// body. The gate offered "pay" only when every part was payable,
			// and the window is synchronous -- no priority pass runs between
			// its asks -- so a part the walk finds unpayable is the decline,
			// with nothing yet moved to un-pay.
			e.advanceTriggeredMandatory(tc)
			return
		}
		if len(tc.amount.Draw) > 0 {
			// The Draw-bearing cost: resolve every count (source SVar for the
			// dynamic form), charge the mana half, then draw. The window's ask
			// offered "pay" only when this resolution succeeds, so the pay
			// answer is never a partial payment.
			draws, ok := e.triggeredCostDrawCounts(tc)
			if ok && e.payManaConv(tc.player, tc.amount, e.paymentConv(tc.player, tc.source, false)) {
				paid = true
				for i, part := range tc.amount.Draw {
					if drawer, ok := castFlowDrawPlayer(part.Spec, tc.player); ok {
						for n := int32(0); n < draws[i]; n++ {
							e.drawCostCard(drawer)
						}
					}
				}
			}
		} else {
			paid = tc.amount.Priceable() &&
				e.payManaConv(tc.player, tc.amount, e.paymentConv(tc.player, tc.source, false))
		}
	}
	rp := tc.resume
	e.triggerCost = nil
	if paid {
		rp.kind = "effect_paid"
		e.resumeResolution(rp, nil)
		return
	}
	e.triggeredCostDecline(tc)
}

// triggeredCostDecline completes a declined window: the body never runs, and
// the continuation is the outer frame when one is parked or the plain
// resolution completion (the tail the decline option's answer has always
// taken).
func (e *Engine) triggeredCostDecline(tc *triggeredEffectCost) {
	rp := tc.resume
	e.triggerCost = nil
	if rp.outer != nil {
		e.resumeResolution(rp.outer, nil)
		return
	}
	e.finishResumption(rp.obj)
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
}

// triggeredMandatoryParts is the flat, ordered list of choice-bearing
// components a settle walks: every Sac part, then every Exile part, then
// every Discard part. The order is deterministic (the cost grammar's own
// order, extended with Discard last) so a replay settles the same picks in
// the same sequence. The mandatory walk only ever carries Sac/Exile
// (mandatorySettleShape); the Discard range serves the optional window's
// pay arm, which walks the same list after the pay election (trigcost2).
func triggeredMandatoryParts(c Cost) []CostPart {
	parts := make([]CostPart, 0, len(c.Sac)+len(c.Exile)+len(c.Discard))
	parts = append(parts, c.Sac...)
	parts = append(parts, c.Exile...)
	parts = append(parts, c.Discard...)
	return parts
}

// triggeredMandatoryZone is the zone a mandatory component's candidates
// come from. A Sac part is always the battlefield; an Exile part carries its
// own Zone (ZBattlefield for the bare Exile<N/Spec> token -- Dalek
// Intensive Care's shape -- ZHand/ZGraveyard for the ExileFrom* heads), with
// the zero value defaulting to hand, exactly as exAsk reads it.
func triggeredMandatoryZone(part CostPart, isSac bool) state.Zone {
	if isSac {
		return state.ZBattlefield
	}
	if part.Zone == 0 {
		return state.ZHand
	}
	return part.Zone
}

// triggeredMandatoryCandidates returns the still-available objects that can
// pay one component: the battlefield permanents a Sac part names (never one
// a CantSacrifice restriction blocks, and never a source that has left the
// battlefield), the zone's cards an Exile part names, or the hand cards a
// Discard part names (through the cast flow's discardCandidates, so the
// Hand all-candidates and Random seeded-selection specials read identically
// there), deduped against every component already settled so one object
// cannot pay twice.
func (e *Engine) triggeredMandatoryCandidates(tc *triggeredEffectCost, idx int, part CostPart) []state.ObjID {
	isSac := idx < len(tc.amount.Sac)
	used := make(map[state.ObjID]bool, len(tc.sacs)+len(tc.exiles)+len(tc.discards))
	for _, id := range tc.sacs {
		used[id] = true
	}
	for _, id := range tc.exiles {
		used[id] = true
	}
	for _, id := range tc.discards {
		used[id] = true
	}
	if triggeredPartIsDiscard(tc.amount, idx) {
		return e.discardCandidates(tc.player, tc.source, part, false, used)
	}
	zone := triggeredMandatoryZone(part, isSac)
	spec := part.Spec
	if isSac {
		// NICKNAME is the same bare self-reference as CARDNAME.
		spec = sacrificeMatchSpec(spec)
	}
	var out []state.ObjID
	for _, id := range e.G.Zone(zone, tc.player) {
		if used[id] {
			continue
		}
		if isSac && e.SacrificeBlocked(id) {
			continue
		}
		if effects.MatchesSpecFrom(e.G, spec, id, tc.player, tc.source) {
			out = append(out, id)
		}
	}
	return out
}

// triggeredPartIsDiscard reports whether the component at flat index idx of
// c's triggeredMandatoryParts list is a Discard part (the list's third
// range, after Sac and Exile).
func triggeredPartIsDiscard(c Cost, idx int) bool {
	return idx >= len(c.Sac)+len(c.Exile)
}

// recordTriggeredMandatoryPick appends one settled component's picks to the
// window's reservation lists.
func (tc *triggeredEffectCost) recordMandatoryPick(idx int, ids []state.ObjID) {
	switch {
	case idx < len(tc.amount.Sac):
		tc.sacs = append(tc.sacs, ids...)
	case idx < len(tc.amount.Sac)+len(tc.amount.Exile):
		tc.exiles = append(tc.exiles, ids...)
	default:
		tc.discards = append(tc.discards, ids...)
	}
}

// advanceTriggeredMandatory settles every component of a cost in order: a
// component with an exact-candidate count records its picks without a
// decision (a decision nobody could answer differently is never emitted),
// a component with a genuine choice poses a real KChoose, and a component
// that cannot be paid at all skips the parked body. No pay/decline election
// is ever posed HERE -- for a mandatory cost there is no "may" (the window's
// pay election, where one exists, was answered before this walk started),
// and a Hand-spec Discard pays the whole hand and a Random-spec Discard pays
// seeded picks without ever asking (the cast flow's discardAsk specials).
func (e *Engine) advanceTriggeredMandatory(tc *triggeredEffectCost) {
	parts := triggeredMandatoryParts(tc.amount)
	for tc.part < len(parts) {
		part := parts[tc.part]
		isSac := tc.part < len(tc.amount.Sac)
		isDiscard := triggeredPartIsDiscard(tc.amount, tc.part)
		eligible := e.triggeredMandatoryCandidates(tc, tc.part, part)
		if isDiscard && strings.EqualFold(part.Spec, "Hand") {
			// Forge's "discard your hand" shape: every hand card pays,
			// whatever count the token spells (the corpus writes the ignored
			// count as both 0 and 1).
			tc.recordMandatoryPick(tc.part, eligible)
			tc.part++
			continue
		}
		if isDiscard && strings.EqualFold(part.Spec, "Random") {
			// A Random discard is a selection method, never a choice: the
			// seeded picks the cast flow's discardAsk takes.
			picks := make([]state.ObjID, 0, part.N)
			for i := int32(0); i < part.N && len(eligible) > 0; i++ {
				pick := e.Rand(len(eligible))
				picks = append(picks, eligible[pick])
				eligible = append(eligible[:pick], eligible[pick+1:]...)
			}
			tc.recordMandatoryPick(tc.part, picks)
			tc.part++
			continue
		}
		if int32(len(eligible)) < part.N {
			// A component that cannot be fully paid skips the body; nothing
			// has moved yet (the settle emits every event only at the end),
			// so the skip leaves the board untouched.
			e.triggeredCostDecline(tc)
			return
		}
		if int32(len(eligible)) == part.N {
			tc.recordMandatoryPick(tc.part, eligible)
			tc.part++
			continue
		}
		name := "triggered ability"
		if o := e.G.Obj(tc.source); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		kind, noun := "sacrifice", "permanent(s)"
		switch {
		case isDiscard:
			kind, noun = "discard", "card(s)"
		case !isSac:
			kind, noun = "exile_cost", "card(s)"
		}
		d := &decision.Decision{Player: tc.player, Kind: decision.KChoose,
			Min: int(part.N), Max: int(part.N), Source: tc.source,
			Prompt: name + " — choose " + strconv.FormatInt(int64(part.N), 10) + " " + noun + " to " + kind}
		for _, id := range eligible {
			label := e.targetName(id)
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: kind, Obj: id, Label: label})
		}
		e.choosing = chooseTriggeredMandatory
		e.ask(d)
		return
	}
	e.settleTriggeredMandatory(tc)
}

// triggeredMandatoryAnswer validates one answered pick against the current
// board and advances the walk (the unless-pay re-validation discipline).
func (e *Engine) triggeredMandatoryAnswer(chosen []decision.Option) {
	tc := e.triggerCost
	if tc == nil {
		return
	}
	e.choosing = chooseNone
	parts := triggeredMandatoryParts(tc.amount)
	if tc.part >= len(parts) {
		return
	}
	part := parts[tc.part]
	ids := make([]state.ObjID, 0, len(chosen))
	for _, o := range chosen {
		if o.Obj != 0 {
			ids = append(ids, o.Obj)
		}
	}
	eligible := e.triggeredMandatoryCandidates(tc, tc.part, part)
	allowed := make(map[state.ObjID]bool, len(eligible))
	for _, id := range eligible {
		allowed[id] = true
	}
	if int32(len(ids)) != part.N {
		e.triggeredCostDecline(tc)
		return
	}
	for _, id := range ids {
		if !allowed[id] {
			e.triggeredCostDecline(tc)
			return
		}
	}
	tc.recordMandatoryPick(tc.part, ids)
	tc.part++
	e.advanceTriggeredMandatory(tc)
}

// settleTriggeredMandatory finishes a fully settled cost: the mana half (if
// any) is charged, each picked object leaves its zone by the correct event
// (events.Sacrifice for a Sac part, a MoveZone to exile for an Exile part,
// events.DiscardCost for a Discard part -- the cast flow's provenance
// marker, so discard triggers and the CardsDiscardedThisTurn count see cost
// discards), the cost's own Draw components draw (the pay arm's draw half),
// and the parked body resumes. The mana charge is a totality guard -- the
// gates armed this window only when the mana half was chargeable, but a
// board that changed under the walk could in principle leave it uncovered,
// in which case the body is skipped rather than half paid.
func (e *Engine) settleTriggeredMandatory(tc *triggeredEffectCost) {
	stripped := tc.amount
	stripped.Sac, stripped.Discard, stripped.Exile, stripped.Draw = nil, nil, nil, nil
	if (stripped.hasManaPayment() || stripped.Life > 0) &&
		!e.payManaConv(tc.player, stripped, e.paymentConv(tc.player, tc.source, false)) {
		e.triggeredCostDecline(tc)
		return
	}
	rp := tc.resume
	e.triggerCost = nil
	e.choosing = chooseNone
	for _, id := range tc.sacs {
		if o := e.G.Obj(id); o != nil && o.Zone == state.ZBattlefield {
			e.emit(events.Sacrifice(id))
		}
	}
	for _, id := range tc.exiles {
		if o := e.G.Obj(id); o != nil {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZExile, Text: "exiled as a cost"})
		}
	}
	for _, id := range tc.discards {
		if o := e.G.Obj(id); o != nil {
			e.emit(events.DiscardCost(id))
		}
	}
	// The cost's own Draw components (the pay arm's draw half -- Ambergris'
	// "discard your hand. If you do, draw two cards"). The pay gate resolved
	// every count before offering "pay", so the belt-and-braces re-resolution
	// here failing is the decline: nothing to un-pay, the component events
	// above are the payment and the body never runs.
	if len(tc.amount.Draw) > 0 {
		draws, ok := e.triggeredCostDrawCounts(tc)
		if ok {
			for i, part := range tc.amount.Draw {
				if drawer, hasDrawer := castFlowDrawPlayer(part.Spec, tc.player); hasDrawer {
					for n := int32(0); n < draws[i]; n++ {
						e.drawCostCard(drawer)
					}
				}
			}
		}
		if !ok {
			e.triggeredCostDecline(tc)
			return
		}
	}
	rp.kind = "effect_paid"
	e.resumeResolution(rp, nil)
}

// nextDynTapPart returns the next unsettled dynamic tapXType part of the
// window's cost.
func (e *Engine) nextDynTapPart(tc *triggeredEffectCost) (CostPart, bool) {
	dyn := dynTapParts(tc.amount)
	if tc.tapIdx >= len(dyn) {
		return CostPart{}, false
	}
	return dyn[tc.tapIdx], true
}

// triggeredTapAsk poses one dynamic tapXType part's tap election: a KChoose
// over the untapped permanents matching the part's spec, Min 0 (the empty
// answer is the decline), Max the candidate count. An X-form part's answered
// count is the cost's announced {X} (rules/mana.go's dynTapCost doc); the
// body the window then runs reads it through the resume point (rp.tapPaidX).
func (e *Engine) triggeredTapAsk(tc *triggeredEffectCost, part CostPart) {
	candidates := e.costCandidates(tc.player, tc.source, state.ZBattlefield, part.Spec, false, true)
	if len(candidates) == 0 {
		// No eligible permanent: an X-form election could only announce 0 and
		// an Any-form part is not payable at all -- both are the decline, and
		// a decision nobody could answer differently is never emitted (the
		// strict-supersets convention).
		e.triggeredCostDecline(tc)
		return
	}
	name := "triggered ability"
	if o := e.G.Obj(tc.source); o != nil && o.Face() != nil {
		name = o.Face().Name
	}
	d := &decision.Decision{Player: tc.player, Kind: decision.KChoose, Min: 0, Max: len(candidates),
		Prompt: name + " — " + tc.costLabel, Source: tc.source}
	for _, id := range candidates {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "trigger_cost_tap", Obj: id, Label: e.targetName(id)})
	}
	e.choosing = chooseTriggeredCost
	e.ask(d)
}

// triggeredTapAnswer settles one tap election: the chosen permanents tap
// (the Tap events every other cost tap takes), and when every dynamic part
// of the cost has settled the window's payment is complete and the parked
// body resumes with the X-form count bound. The window is synchronous -- no
// priority pass runs between its asks -- so the chosen objects cannot have
// moved; the zone guard is the same read the cumulative actions take.
func (e *Engine) triggeredTapAnswer(tc *triggeredEffectCost, chosen []decision.Option) {
	dyn := dynTapParts(tc.amount)
	idx := tc.tapIdx
	if idx >= len(dyn) {
		return
	}
	part := dyn[idx]
	for _, o := range chosen {
		if ob := e.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZBattlefield {
			e.emit(events.Event{Kind: events.Tap, Obj: o.Obj, Text: "tapped as a cost"})
		}
	}
	rp := tc.resume
	paidX := int32(0)
	if part.Dyn == "X" {
		paidX = int32(len(chosen))
	}
	tc.tapIdx++
	if tc.tapIdx < len(dyn) {
		e.triggeredCostPaymentAsk()
		return
	}
	e.triggerCost = nil
	if paidX != 0 {
		rp.tapPaidX = paidX
	}
	rp.kind = "effect_paid"
	e.resumeResolution(rp, nil)
}

func init() {
	effects.RegisterNonAPI("kw:Cumulative upkeep", "stat:UntapOtherPlayer")
}
