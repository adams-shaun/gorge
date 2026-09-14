package rules

import (
	"strconv"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// kw:Cumulative upkeep (CR 702.46): "At the beginning of your upkeep, put an
// age counter on this permanent, then sacrifice it unless you pay its upkeep
// cost for each age counter on it." The upkeep cost is the keyword's
// parameter ("1", a colour pip, a PayLife<N>, a hybrid pip -- the corpus's
// 80 K: lines carry all of these plus two snow {S} costs, which ParseCost
// degrades to one generic; see the Issues note in this ticket's report).
//
// Forge scripts it as a K: keyword, not a trigger, so this engine implements
// it as the upkeep's turn-based action, exactly as it implements the
// cleanup step's discard: the age counter is placed and the pay-or-sacrifice
// choice is asked BEFORE the upkeep's triggered abilities are placed on the
// stack (those land at the next priority round's putTriggersOnStack). The
// CR-faithful shape -- one synthetic trigger per permanent, resolving in
// APNAP order with the payment at resolution -- is a refinement this build
// does not model; with a single cumulative-upkeep permanent in play the two
// orders coincide, which is every corpus card's real shape (none of the 80
// corpus cards carries two).
//
// The choice rides the ordinary KChoose machinery with a dedicated
// chooseFor, the mirror of chooseCleanup: "pay" is offered first and ONLY
// when the pool (under the live ManaConvert conversion) can pay the whole
// per-age total, so option 0 -- the deterministic bot/no-host answer -- is
// always a legal payment; when paying is impossible only "sacrifice" is
// offered. Payment is from the floating pool only (CR 702.46b allows
// activating mana abilities while paying; this engine's payment API has no
// tap-to-pay, the same boundary its UnlessCost$ payment already documents).
// Paying does not remove age counters (CR 702.46a: they persist and the
// amount grows every turn); declining sacrifices the permanent.

// chooseCumulative is the chooseFor for the pay-or-sacrifice choice. It
// extends the enum one past the last value in rules/mana_activation.go
// (chooseManaDiscard), the same pattern chooseCleanup/chooseDamageDivision
// established; the exact value only needs to differ, never to be adjacent.
const chooseCumulative chooseFor = chooseManaDiscard + 1

// cumulativeUpkeep is the in-flight pay-or-sacrifice flow: the permanents
// still to process this upkeep, in the deterministic zone order they were
// collected in, and the per-age facts of the one currently asked. Plain
// value data so Clone copies it (the *cards.SA-style Cost fields are plain).
type cumulativeUpkeep struct {
	player state.PlayerID
	// queue is the permanents whose age counter and choice are still owed,
	// collected once at the upkeep's start and never rebuilt -- a permanent
	// that leaves the battlefield mid-queue is skipped by its ID.
	queue []state.ObjID
	// current is the permanent being asked; amount its scaled per-age cost.
	current    state.ObjID
	amount     Cost
	windowDone bool
}

// scaleCost returns c with every component multiplied by n: colours, generic
// and life multiply, and each hybrid/Phyrexian pip is repeated n times. This
// is the "upkeep cost for each age counter" composition (CR 702.46a) and the
// only place the per-age cost becomes the turn's actual demand. {X} is not
// multiplied (no corpus cumulative upkeep names X; a Cost with X set would
// be unpriceable and the ask would offer sacrifice only).
func scaleCost(c Cost, n int32) Cost {
	if n <= 1 {
		return c
	}
	out := Cost{Generic: 0, Life: 0}
	for i := range c.Colored {
		out.Colored[i] = c.Colored[i] * n
	}
	out.Generic = c.Generic * n
	out.Life = c.Life * n
	for i := int32(0); i < n; i++ {
		out.Hybrid = append(out.Hybrid, c.Hybrid...)
		out.Phyrexian = append(out.Phyrexian, c.Phyrexian...)
	}
	out.Tap = c.Tap
	out.Sac = c.Sac
	out.Discard = c.Discard
	out.SubCounter = c.SubCounter
	out.AddCounter = c.AddCounter
	out.X = c.X
	return out
}

// startCumulativeUpkeep begins the upkeep's cumulative-upkeep turn-based
// action for the ACTIVE player's permanents, in battlefield zone order, and
// drives the queue. Called from beginTurn between StepUpkeep's entry and the
// turn's Priority event: when there is nothing to do the caller's ordinary
// Priority emit runs here instead, byte-identical to the old beginTurn tail,
// and when the flow suspends on its first choice the Priority emit happens
// only after the last answer (cumulativeAnswer's own tail).
func (e *Engine) startCumulativeUpkeep() {
	p := e.G.Active
	var queue []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o == nil {
			continue
		}
		if _, ok := o.Face().KeywordParam("Cumulative upkeep"); ok {
			queue = append(queue, id)
		}
	}
	e.cumulative = &cumulativeUpkeep{player: p, queue: queue}
	e.continueCumulative()
}

// continueCumulative places the next permanent's age counter and asks its
// pay-or-sacrifice choice, or finishes the queue with the turn's Priority
// event. A queued permanent that has left the battlefield (sacrificed as a
// cost of something else, bounced) is skipped: there is nothing to age.
func (e *Engine) continueCumulative() {
	cu := e.cumulative
	if cu == nil {
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
		return
	}
	for len(cu.queue) > 0 {
		id := cu.queue[0]
		cu.queue = cu.queue[1:]
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		cu.current = id
		e.placeAgeCounterAndAsk(id)
		return
	}
	e.cumulative = nil
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
}

// placeAgeCounterAndAsk increments the age counter and poses the
// pay-or-sacrifice choice for the permanent id. A per-age cost this build
// cannot price ({X}: an unfolded X is unpriceable by the payment API) offers
// sacrifice only, the conservative direction every unpriceable cost here
// takes.
func (e *Engine) placeAgeCounterAndAsk(id state.ObjID) {
	cu := e.cumulative
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		e.continueCumulative()
		return
	}
	f := o.Face()
	param, ok := f.KeywordParam("Cumulative upkeep")
	if !ok {
		e.continueCumulative()
		return
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "AGE", Amount: 1})
	ages := o.Counter("AGE") // after the increment
	cu.amount = scaleCost(ParseCost(param), ages)
	cu.windowDone = false
	e.cumulativePaymentAsk()
}

// cumulativePaymentAsk is CR 702.46b's mana-ability payment window followed
// by the pay-or-sacrifice answer. It shares the cast payment window's source
// gate, but does not grant priority: only mana abilities and Done are legal.
func (e *Engine) cumulativePaymentAsk() {
	cu := e.cumulative
	if cu == nil {
		return
	}
	id := cu.current
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		e.continueCumulative()
		return
	}
	if !cu.windowDone && cu.amount.Priceable() && !e.costPayable(cu.player, id, false, cu.amount) {
		var sources []state.ObjID
		for _, source := range e.G.Zone(state.ZBattlefield, cu.player) {
			if e.untappedManaSource(cu.player, source) {
				sources = append(sources, source)
			}
		}
		if len(sources) > 0 {
			d := &decision.Decision{Player: cu.player, Kind: decision.KChoose, Min: 1, Max: 1,
				Prompt: "Activate mana abilities to pay cumulative upkeep", Source: id}
			for _, source := range sources {
				d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "activate", Obj: source,
					Label: "Tap " + e.G.Obj(source).Face().Name + " for mana"})
			}
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "done", Label: "Done"})
			e.choosing = chooseCumulative
			e.ask(d)
			return
		}
	}
	param, _ := o.Face().KeywordParam("Cumulative upkeep")
	costLabel := strconv.FormatInt(int64(o.Counter("AGE")), 10) + " age counter(s)"
	var opts []decision.Option
	if cu.amount.Priceable() && e.costPayable(cu.player, id, false, cu.amount) {
		opts = append(opts, decision.Option{Index: len(opts), Kind: "cumulative_pay", Label: "Pay " + param + " per age (" + costLabel + ")", Obj: id})
	}
	opts = append(opts, decision.Option{Index: len(opts), Kind: "cumulative_sac", Label: "Sacrifice " + o.Face().Name, Obj: id})
	e.choosing = chooseCumulative
	e.ask(&decision.Decision{Player: cu.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: o.Face().Name + " — cumulative upkeep: pay " + param + " for each age counter, or sacrifice", Source: id, Options: opts})
}

// cumulativeAnswer applies a mana-window answer or pays/sacrifices the
// current permanent and drives the queue on.
func (e *Engine) cumulativeAnswer(chosen []decision.Option) {
	cu := e.cumulative
	if cu == nil || len(chosen) == 0 {
		return
	}
	e.choosing = chooseNone
	switch chosen[0].Kind {
	case "activate":
		e.activateCumulativeMana(cu.player, chosen[0].Obj)
		return
	case "done":
		cu.windowDone = true
		e.cumulativePaymentAsk()
		return
	}
	id := cu.current
	paid := chosen[0].Kind == "cumulative_pay" && e.G.Obj(id) != nil &&
		e.costPayable(cu.player, id, false, cu.amount) &&
		e.payManaConv(cu.player, cu.amount, e.paymentConv(cu.player, id, false))
	if !paid {
		if o := e.G.Obj(id); o != nil && o.Zone == state.ZBattlefield {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed for cumulative upkeep"})
		}
	}
	e.continueCumulative()
}

func init() {
	effects.RegisterNonAPI("kw:Cumulative upkeep", "stat:UntapOtherPlayer")
}
