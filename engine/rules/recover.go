package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// Recover (kw:Recover, CR 702.59: "Recover [cost] (When a creature is put
// into your graveyard from the battlefield, you may pay [cost]. If you do,
// return this card from your graveyard to your hand. Otherwise, exile this
// card.)") is expanded by cards into an ordinary ChangesZone trigger living
// in the graveyard (engine/cards/kw_recover.go), so it is ordered and placed
// with every other trigger and can be responded to before it resolves. This
// file owns the resolution-time election, the Echo flow's shape
// (engine/rules/echo.go): pay through the shared pool-only payment window
// (resPayFund, paymentManaAsk) or decline. The decline arm and any failed
// payment exile the card — CR 702.59's "otherwise, exile this card" — and
// the pay arm returns it from the graveyard to the hand. A cost this build
// cannot price is skipped LOUDLY (one Note, the card stays in the
// graveyard), the Echo convention: a silent exile every death would be
// flatly wrong.
//
// chooseRecover sits in the chooseRiot+1.. family; the numbers only need to
// differ, per the chooseFor contract — transient engine RAM, Clone copies
// the struct, no event or wire field ever carries one.
const chooseRecover chooseFor = 45

type recoverFlow struct {
	stackObj   state.ObjID
	source     state.ObjID
	player     state.PlayerID
	amount     Cost
	costLabel  string
	windowDone bool
	// res is the pay-first funding state (respay.go): the pay/exile election
	// comes before any mana window.
	res resPay
}

// startRecover starts resolution of the already-stacked keyword trigger. The
// election's outcome is not applied before this point: responses and another
// simultaneous trigger observe the pre-resolution board, like every other
// resolving ability.
func (e *Engine) startRecover(stackObj, source state.ObjID, sa *cards.SA) {
	o := e.G.Obj(source)
	stack := e.G.Obj(stackObj)
	// A source that left the graveyard in response (returned by another
	// trigger, exiled, cast) is asked nothing and exiled by nothing.
	if o == nil || o.Zone != state.ZGraveyard || stack == nil {
		e.finishResumption(stackObj)
		return
	}
	// The triggered ability's controller was captured when it was placed on
	// the stack; a response changing the card's control must not transfer
	// the already-triggered election (the Cumulative upkeep contract,
	// CR 113.8).
	rf := &recoverFlow{stackObj: stackObj, source: source, player: stack.Controller}
	label := sa.ParamStr(cards.PKCost)
	// The strict unless parser is the one that accepts the corpus's
	// non-mana token (Garza's Assassin's LifeTotalHalfUp); a token it
	// rejects falls back to the ordinary cost parser (a bare mana cost).
	if c, ok := costvocab.ParseUnlessCost(label); ok {
		rf.amount = c
	} else {
		rf.amount = e.parseCost(label)
	}
	// Garza's Assassin's life spelling folds here: the LifeTotalHalfUp token
	// prices as half the payer's current life, rounded up, unpayable at zero
	// life (pay.UnlessFoldDynamic, the Temporal Extortion read).
	folded, ok := pay.UnlessFoldDynamic(asPayer(e), rf.player, rf.amount, nil)
	if !ok || !folded.Priceable() {
		e.emit(events.Event{Kind: events.Note, Player: rf.player, Obj: source,
			Text: "recover cost unresolvable: " + label})
		e.finishResumption(stackObj)
		e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
		return
	}
	rf.amount = folded
	rf.costLabel = capitaliseFirst(label)
	e.recover = rf
	e.recoverElectionAsk()
}

// recoverElectionAsk poses the pay-or-exile election. The pay option is
// offered FIRST when payable (a bot's clamp fallback takes option 0 — pay is
// the sane default) and the exile option is always present, so nobody is ever
// wedged. A pool that cannot yet pay opens the shared mana-ability funding
// window first (the cumulativePaymentAsk shape).
func (e *Engine) recoverElectionAsk() {
	rf := e.recover
	if rf == nil {
		return
	}
	o := e.G.Obj(rf.source)
	if o == nil || o.Zone != state.ZGraveyard {
		e.finishRecover()
		return
	}
	if rf.res.chosen {
		// The payer already elected to pay (respay.go): fund what the pool
		// lacks, then charge.
		if e.resPayFund(&rf.res, rf.player, rf.source, rf.amount, rf.windowDone,
			"Activate mana abilities to pay recover", chooseRecover, paymentOther) {
			return
		}
		e.recoverAnswer([]decision.Option{{Kind: "recover_pay", Obj: rf.source}})
		return
	}
	payable := pay.CostPayableOther(asPayer(e), rf.player, rf.source, rf.amount)
	var opts, manual []decision.Option
	if e.resPayNeedsMana(rf.player, rf.source, rf.amount, paymentOther) {
		routes := e.resPayOptions(rf.player, rf.source, rf.amount, paymentOther, "recover_pay", "recover_pay_manual", rf.costLabel)
		opts, manual = []decision.Option{routes[0]}, routes[1:]
	} else if payable {
		opts = append(opts, decision.Option{Kind: "recover_pay", Obj: rf.source,
			Label: rf.costLabel})
	}
	opts = append(opts, decision.Option{Kind: "recover_exile", Obj: rf.source,
		Label: "Exile " + o.Face().Name})
	opts = append(opts, manual...)
	for i := range opts {
		opts[i].Index = i
	}
	windowAsk(e, &decision.Decision{Player: rf.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: o.Face().Name + " — recover: " + rf.costLabel + " or exile",
		Source: rf.source, Options: opts}, chooseRecover)
}

// recoverAnswer handles the election (and the mana window feeding it). The
// decline arm and any failed payment fall through to the exile — CR 702.59's
// "otherwise, exile this card".
func (e *Engine) recoverAnswer(chosen []decision.Option) {
	rf := e.recover
	if rf == nil || len(chosen) == 0 {
		return
	}
	e.choosing = chooseNone
	switch recoverAnswerCodes.Code(string(chosen[0].Kind)) {
	case recoverAnswerActivate:
		e.activatePaymentMana(rf.player, chosen[0].Obj)
		return
	case recoverAnswerDone:
		rf.windowDone = true
		e.recoverElectionAsk()
		return
	case recoverAnswerPay:
		if !rf.res.chosen && e.resPayNeedsMana(rf.player, rf.source, rf.amount, paymentOther) {
			rf.res.elect(chosen[0].Kind == "recover_pay_manual")
			e.recoverElectionAsk()
			return
		}
		if pay.PayManaConv(asPayer(e), rf.player, rf.amount, asEval(e).Conv(rf.player, rf.source, pay.SAOther)) {
			e.recoverReturn()
			return
		}
	}
	e.recoverExile()
}

// recoverReturn applies the pay arm: the card returns from the graveyard to
// its owner's hand. A source that left the graveyard in response is not
// moved again.
func (e *Engine) recoverReturn() {
	rf := e.recover
	if o := e.G.Obj(rf.source); o != nil && o.Zone == state.ZGraveyard {
		e.emit(events.Event{Kind: events.MoveZone, Obj: rf.source,
			From: state.ZGraveyard, To: state.ZHand, Text: "recovered"})
	}
	e.finishRecover()
}

// recoverExile applies the decline arm: the card leaves the graveyard for
// exile. A source that left the graveyard in response is not moved again.
func (e *Engine) recoverExile() {
	rf := e.recover
	if o := e.G.Obj(rf.source); o != nil && o.Zone == state.ZGraveyard {
		e.emit(events.Event{Kind: events.MoveZone, Obj: rf.source,
			From: state.ZGraveyard, To: state.ZExile, Text: "recover declined"})
	}
	e.finishRecover()
}

func (e *Engine) finishRecover() {
	rf := e.recover
	e.recover = nil
	e.choosing = chooseNone
	if rf == nil {
		return
	}
	e.finishResumption(rf.stackObj)
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
}

func init() {
	effects.RegisterNonAPI("kw:Recover")
}

type recoverAnswerCode uint16

const (
	recoverAnswerActivate recoverAnswerCode = iota + 1
	recoverAnswerDone
	recoverAnswerPay
)

var recoverAnswerCodes = state.NewStrCodes(
	state.StrEntry[recoverAnswerCode]{Key: "activate", Val: recoverAnswerActivate},
	state.StrEntry[recoverAnswerCode]{Key: "done", Val: recoverAnswerDone},
	state.StrEntry[recoverAnswerCode]{Key: "recover_pay", Val: recoverAnswerPay},
	state.StrEntry[recoverAnswerCode]{Key: "recover_pay_manual", Val: recoverAnswerPay},
)

// startEchoOrRecover dispatches the keyword-expansion triggers whose DB$ body
// needs rules' payment window (CR 702.35a Echo, CR 702.59 Recover): each is an
// ordinary trigger whose body asks a pay-or-lose election (engine/rules/echo.go,
// recover.go). Echo's intervening-if was already applied at trigger time
// (triggerMatches's Echo$ branch), so everything reaching here is owed. It
// reports whether the stacked ability was one of the two and has been started.
func startEchoOrRecover(e *Engine, id state.ObjID, o *state.Object) bool {
	if o.Ability.API == "Echo" {
		e.startEcho(id, o.Source, o.Ability)
		return true
	}
	if o.Ability.API == "Recover" {
		e.startRecover(id, o.Source, o.Ability)
		return true
	}
	return false
}

// echoOrRecoverAnswer routes the answered keyword-expansion election
// (e.choosing) to its keyword: Echo's pay-or-sacrifice or Recover's
// pay-or-exile.
func echoOrRecoverAnswer(e *Engine, chosen []decision.Option) {
	if e.choosing == chooseRecover {
		e.recoverAnswer(chosen)
		return
	}
	e.echoAnswer(chosen)
}
