package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { effects.RegisterNonAPI("kw:Assist") }

// assistAnswerKind is the Option.Kind every assist election carries: the one
// identifier the ask, the castAnswer dispatch (cast_payment.go's code table)
// and botpolicy's arm all key off, so a seat's answer can never be routed by a
// different rule than the one that offered it.
const assistAnswerKind = "assist"

// assist.go is CR 702.132 Assist: "Other players may pay any amount of that
// cost up to that cost, then reduce the total cost by the amount paid this
// way." The keyword is not an ability the controller activates: it is a
// payment step. Once the total cost is settled (every announcement made, the
// 601.2e recheck passed) and before the caster's mana window opens, each other
// seat in APNAP order from the caster is offered its contribution, pays it out
// of its OWN pool, and the generic part of the caster's bill drops by exactly
// what left that pool.
//
// The legality rule has one home: pay.AssistCap (engine/rules/pay/assist.go)
// prices what a seat's pool can actually pay. assistAsk builds each seat's
// option list from it, and answerAssist drains through pay.AssistPay, which
// never exceeds it — so the offer, the answer Decision.Validate admits and the
// mana that leaves a foreign pool are the same read. The decision itself is an
// ordinary Min 1 / Max 1 KChoose over options 0..cap (the Chorus/Gift shape),
// so no parallel legality rule exists to drift; botpolicy's assist arm takes
// the 0 option and its test runs that answer back through Decision.Validate.

// assistAsk poses the CR 702.132a contributions of the pending cast, one seat
// at a time in APNAP order from the caster (state.Game.AliveFrom). It is the
// Assist payment step: called from payCast after the 601.2e recheck and before
// the CR 601.2g mana window, so a seat's contribution is out of the way before
// the caster floats mana for what is left. Each seat is offered 0..cap where
// cap is the min of the keyword's amount and the generic part still owed,
// capped again by what that seat's pool can pay; a seat that cannot pay
// anything is not asked (the single-legal-answer election the engine never
// poses) and pays 0. Returns true while the cast is suspended on an ask.
func (e *Engine) assistAsk() bool {
	pc := e.cast
	if pc == nil || pc.assistDone || pc.isAbility() {
		return false
	}
	amount, printed, ok := e.assistOf(pc)
	if !ok {
		pc.assistDone = true
		return false
	}
	// The generic part still owed, after every reduction that has already
	// settled (Convoke's taps, Delve's exiles, cost modifiers, and the earlier
	// seats' contributions): Assist reduces the total cost, so it can never be
	// paid twice and can never drive the bill below zero.
	owed := e.castPaymentMana(pc).Generic
	if !printed {
		// A bare K:Assist line means "up to the generic part of this cost":
		// the {3} of Bring Down's {3}{W}, and the announced X of Gang Up's
		// {X}{B} (xAsk folded X into the generic part before this point).
		amount = owed
	}
	if amount > owed {
		amount = owed
	}
	if amount <= 0 {
		pc.assistDone = true
		return false
	}
	// The ceiling is the FIRST pass's amount and holds for the whole step (it
	// is 0 only before that first pass, which is when amount is 0 and the step
	// closes below): every later seat's ask re-reads the still-outstanding
	// generic part, which the earlier seats' contributions have already
	// lowered, and must never be able to raise the total above the printed
	// amount.
	if pc.assistCap == 0 {
		pc.assistCap = amount
	} else if amount > pc.assistCap {
		amount = pc.assistCap
	}
	var seats []state.PlayerID
	for _, p := range e.G.AliveFrom(pc.player) {
		if p != pc.player {
			seats = append(seats, p)
		}
	}
	for pc.assistAsked < len(seats) {
		p := seats[pc.assistAsked]
		pc.assistAsked++
		cap := pay.AssistCap(asPayer(e), p, amount)
		if cap <= 0 {
			continue // nothing this pool can contribute: it pays 0, unasked
		}
		pc.assistPayer = p
		e.choosing = chooseCast
		e.ask(assistDecision(e, pc, p, cap))
		return true
	}
	pc.assistDone = true
	return false
}

// assistOf reads the cast's derived Assist line (printed or granted: Yume,
// Chronicler of Valor's static puts the keyword on the stack object the same
// way). It reports the printed amount and whether the line carried one: forge
// prints the bare "Assist" for "up to the generic part", and a card whose
// printed amount differs from its generic cost carries the number.
func (e *Engine) assistOf(pc *pendingCast) (amount int32, printed, ok bool) {
	for _, keyword := range e.derivedWith(pc.card, state.ZStack).Keywords {
		head, param, found := strings.Cut(keyword, ":")
		if kwHeadOf(head).ID != kwhAssist.ID {
			continue
		}
		ok = true
		if !found {
			continue
		}
		if n, err := strconv.ParseInt(strings.TrimSpace(param), 10, 32); err == nil && n >= 0 {
			printed = true
			if v := int32(n); v > amount {
				amount = v
			}
		}
	}
	return amount, printed, ok
}

// assistDecision poses one seat's contribution election: a Min 1 / Max 1
// KChoose over one option per payable amount, the amount riding Option.Amount
// (never Index — the same discipline xAsk documents, so a prepended option
// cannot corrupt the value) and the payer riding Option.Player.
func assistDecision(e *Engine, pc *pendingCast, p state.PlayerID, cap int32) *decision.Decision {
	d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Pay how much toward " + e.targetName(pc.card) + "?", Source: pc.card}
	for n := int32(0); n <= cap; n++ {
		label := "Pay " + costPhrase(Cost{Generic: n})
		if n == 0 {
			label = "Pay nothing toward the cost"
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "assist",
			Label: label, Player: p, Amount: int(n)})
	}
	return d
}

// answerAssist settles one seat's answer: the mana it named leaves that seat's
// pool through pay.AssistPay (which pays no more than its pool can and emits
// the negative ManaAdd events that make a foreign spend replayable), and what
// actually left is credited against the caster's generic bill by applyAssist.
// Crediting the DRAINED amount rather than the answered one is what keeps the
// two halves equal however the pool changed under the ask: the caster is never
// charged mana nobody paid, and never over-credited.
func (e *Engine) answerAssist(pc *pendingCast, chosen []decision.Option) {
	if len(chosen) == 0 || pc.assistPaid >= pc.assistCap {
		return
	}
	paid := pay.AssistPay(asPayer(e), pc.assistPayer, int32(chosen[0].Amount))
	if paid <= 0 {
		return
	}
	pc.assistPaid += paid
}

// applyAssist is the caster's half of CR 702.132a: the total cost is reduced by
// the amount the other seats paid this way, out of the generic portion (the
// coloured pips and every non-mana part stay with the caster). Call it where
// the paid total is composed, after Convoke's contributions.
func applyAssist(pc *pendingCast, m Cost) Cost {
	if pc.assistPaid <= 0 {
		return m
	}
	m.Generic -= pc.assistPaid
	if m.Generic < 0 {
		m.Generic = 0
	}
	return m
}
