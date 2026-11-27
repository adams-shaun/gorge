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

// The one identifier for each Assist decision vocabulary. decision owns the
// strings so the ask, the castAnswer dispatch and bots/botpolicy's arms cannot
// drift; these aliases keep the rules call sites readable.
const (
	assistAnswerKind = decision.OptAssist

	assistWindowActivateKind = decision.OptAssistActivate
	assistWindowDoneKind     = decision.OptAssistDone
)

// assistPayment is one seat's promised contribution, recorded while the cast
// is still provisional and drained only when the caster's payment commits (see
// drainAssist). A promise is priced from the seat's floating pool when it is
// answered; nothing can change that pool before the commit, so the drain pays
// exactly the promised amount.
type assistPayment struct {
	player state.PlayerID
	amount int32
}

// assist.go is CR 702.132 Assist: "Other players may pay any amount of that
// cost up to that cost, then reduce the total cost by the amount paid this
// way." The keyword is not an ability the controller activates: it is a
// payment step. Once the total cost is settled (every announcement made, the
// 601.2e recheck passed) and before the caster's mana window opens, each other
// seat in APNAP order from the caster is offered its contribution.
//
// CR 702.132a gives the chosen player a chance to activate mana abilities
// before the caster does. This engine's brief extends that to every other
// seat, so each seat gets its own window: while its floating pool cannot fund
// the remaining generic part and it controls an untapped mana source, the seat
// is asked to activate sources (or press Done), and only then is it offered
// the amounts its pool can now pay. A seat with no reachable mana is skipped
// (a player who cannot pay pays 0) and a seat that presses Done is offered the
// amounts its pool already holds.
//
// The cast is OFFERED against a possible assist rather than only the caster's
// own pool (assistOfferCredit): a caster who can pay only the coloured part is
// a legal caster once another seat may fund the generic part. That is a
// deliberate over-approximation -- the offer prices what the other seats could
// pay, and a table where every seat declines can still rewind the proposal
// (CR 733.1) -- but withholding the cast would make the keyword inert for the
// exact case it exists for.
//
// The legality rule has one home: pay.AssistCap (engine/rules/pay/assist.go)
// prices what a seat's pool can actually pay. assistAsk builds each seat's
// option list from it, and the commit drain goes through pay.AssistPay, which
// never exceeds it -- so the offer, the answer Decision.Validate admits and the
// mana that leaves a foreign pool are the same read. The decision itself is an
// ordinary Min 1 / Max 1 KChoose over options 0..cap (the Chorus/Gift shape),
// so no parallel legality rule exists to drift; botpolicy's assist arm takes
// the 0 option and its test runs that answer back through Decision.Validate.

// assistAsk poses the CR 702.132a contributions of the pending cast, one seat
// at a time in APNAP order from the caster (state.G.AliveFrom). It is the
// Assist payment step: called from payCast after the 601.2e recheck and before
// the CR 601.2g mana window, so the seats' contributions are out of the way
// before the caster floats mana for what is left. Returns true while the cast
// is suspended on a window or an amount election.
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
	seats := e.assistOtherSeats(pc.player)
	for {
		if pc.assistActive {
			if e.assistSeatAsk(pc, pc.assistPayer, amount) {
				return true
			}
			// The seat cannot contribute anything and has no mana window
			// left: it pays 0 and the next seat is offered its election.
			assistSeatDone(pc)
			continue
		}
		if pc.assistAsked >= len(seats) {
			pc.assistDone = true
			return false
		}
		pc.assistActive = true
		pc.assistPayer = seats[pc.assistAsked]
		pc.assistWindowDone = false
		pc.assistWindow = false
	}
}

// assistSeatAsk poses the current seat's next step and reports whether a
// decision was asked. While the seat's pool cannot pay the whole remaining
// amount and it still has an untapped mana source, its mana window comes
// first; once it presses Done (assistWindowDone) or has nothing left to
// activate, it is offered 0..cap out of the pool. A cap of 0 with no window
// left is not asked at all: the single-legal-answer election the engine never
// poses.
func (e *Engine) assistSeatAsk(pc *pendingCast, p state.PlayerID, amount int32) bool {
	payer := asPayer(e)
	cap := pay.AssistCap(payer, p, amount)
	if cap >= amount || pc.assistWindowDone || !e.hasUntappedManaSourceFor(p, assistPaymentDescriptor(amount)) {
		if cap <= 0 {
			return false
		}
		pc.assistWindow = false
		e.choosing = chooseCast
		e.ask(assistDecision(e, pc, p, cap))
		return true
	}
	if !e.assistWindowAsk(pc, p, amount) {
		// No source the window could usefully offer (it exists but yields
		// nothing, or every restriction refuses this payment): settle for
		// what the pool already holds.
		if cap <= 0 {
			return false
		}
		pc.assistWindow = false
		e.choosing = chooseCast
		e.ask(assistDecision(e, pc, p, cap))
		return true
	}
	return true
}

// assistPaymentDescriptor is the descriptor both the window's source filter and
// pay.AssistCap price against (PurposeOther, bare generic): one payment shape,
// so a source the window offers is a source the drain can spend.
func assistPaymentDescriptor(want int32) paymentDescriptor {
	c := Cost{Generic: want}
	return paymentDescriptor{Class: paymentOther, Cost: &c}
}

// assistWindowAsk poses the contributing seat's CR 702.132a mana-ability
// window: one "assist_activate" option per untapped source whose mana this
// payment may spend, then "assist_done". It reports false when no source is
// worth offering.
func (e *Engine) assistWindowAsk(pc *pendingCast, p state.PlayerID, want int32) bool {
	desc := assistPaymentDescriptor(want)
	var sources []state.ObjID
	for _, id := range pay.ManaSourceIDs(e.G, p) {
		if e.untappedManaSourceFor(p, id, desc) && e.windowSourceYields(p, id) {
			sources = append(sources, id)
		}
	}
	if len(sources) == 0 {
		return false
	}
	d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Activate mana abilities to assist " + e.targetName(pc.card) + "?", Source: pc.card}
	for _, id := range sources {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: assistWindowActivateKind,
			Obj: id, Label: "Tap " + o.Face().Name + " for mana",
			Cost: e.manaActivationCostMarker(e.availableManaAbilities(p, id))})
	}
	if len(d.Options) == 0 {
		return false
	}
	d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: assistWindowDoneKind, Label: "Done"})
	pc.assistWindow = true
	e.choosing = chooseCast
	e.ask(d)
	return true
}

// answerAssistWindow settles the contributing seat's mana-window answer:
// activating a source runs that activation for the SEAT (the caster's own
// window would otherwise spend the caster's source), and Done flips the seat to
// its amount election on the next assistAsk re-entry.
func (e *Engine) answerAssistWindow(pc *pendingCast, kind string, chosen []decision.Option) {
	if pc == nil || !pc.assistWindow {
		return
	}
	pc.assistWindow = false
	switch kind {
	case assistWindowActivateKind:
		if len(chosen) > 0 {
			e.activateManaPayment(pc.assistPayer, chosen[0].Obj, true)
		}
	case assistWindowDoneKind:
		pc.assistWindowDone = true
	}
}

// assistOtherSeats is the APNAP order of the seats offered a contribution: the
// caster's own seat is excluded.
func (e *Engine) assistOtherSeats(caster state.PlayerID) []state.PlayerID {
	var seats []state.PlayerID
	for _, p := range e.G.AliveFrom(caster) {
		if p != caster {
			seats = append(seats, p)
		}
	}
	return seats
}

// assistSeatDone retires the current seat: the next assistAsk call advances the
// APNAP cursor to the next one.
func assistSeatDone(pc *pendingCast) {
	pc.assistActive = false
	pc.assistWindow = false
	pc.assistWindowDone = false
	pc.assistAsked++
}

// assistOf reads the cast's derived Assist line (printed or granted: Yume,
// Chronicler of Valor's static puts the keyword on the stack object the same
// way). It reports the printed amount and whether the line carried one: forge
// prints the bare "Assist" for "up to the generic part", and a card whose
// printed amount differs from its generic cost carries the number.
func (e *Engine) assistOf(pc *pendingCast) (amount int32, printed, ok bool) {
	return e.assistOfCard(pc.card)
}

// assistOfCard is assistOf for an object that may not be on the stack yet: the
// offer gate and the target ask price a proposal against the same derived
// keyword line the payment step reads.
func (e *Engine) assistOfCard(id state.ObjID) (amount int32, printed, ok bool) {
	for _, keyword := range e.derivedWith(id, state.ZStack).Keywords {
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

// assistPotential is the total generic amount the other seats could produce
// right now: their floating pools plus every untapped mana source, capped by a
// printed Assist amount (a bare line has no cap of its own -- it can pay the
// whole generic part). It is an over-bound by construction; callers re-price
// the real pool after the seat's window.
func (e *Engine) assistPotential(caster state.PlayerID, id state.ObjID) int32 {
	amount, printed, ok := e.assistOfCard(id)
	if !ok {
		return 0
	}
	total := int64(0)
	for _, p := range e.G.AliveFrom(caster) {
		if p == caster {
			continue
		}
		total += int64(e.PotentialMana(p).Total())
		if total >= int64(potentialUnbounded) {
			total = int64(potentialUnbounded)
			break
		}
	}
	if printed && total > int64(amount) {
		total = int64(amount)
	}
	return int32(total)
}

// assistOfferCredit is the generic amount the offer, target-affordability and
// target-menu gates may leave to the other seats: assistPotential capped by the
// spell's generic part. It is exactly the case CR 702.132 exists for: a caster
// who can pay the coloured part alone is a legal caster while another seat may
// fund the generic part. The proposal is still re-checked after the seats
// answer, so a table that declines every contribution rewinds it (CR 733.1).
func (e *Engine) assistOfferCredit(caster state.PlayerID, id state.ObjID, generic int32) int32 {
	if generic <= 0 {
		return 0
	}
	credit := e.assistPotential(caster, id)
	if credit > generic {
		credit = generic
	}
	return credit
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
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: assistAnswerKind,
			Label: label, Player: p, Amount: int(n)})
	}
	return d
}

// answerAssist settles one seat's answer into a PROMISE. The mana leaves the
// seat's pool only when the caster's payment commits (drainAssist), so a cast
// the caster later cancels or cannot finish never takes a contributor's mana.
// Crediting the promised amount rather than a drained one keeps paymentMana --
// and so the caster's mana window and bill -- equal to what the seats will
// actually pay.
//
// The guards are the decision's own identity, not just an amount: the round
// must still be live (assistDone false), the seat must be the one currently
// deciding (assistPayer), no window may be pending, and the answer must be the
// amount election's Kind. A stale or duplicated answer after the round closed
// therefore moves nothing.
func (e *Engine) answerAssist(pc *pendingCast, d *decision.Decision, chosen []decision.Option) {
	if pc == nil || pc.assistDone || !pc.assistActive || pc.assistWindow || pc.assistPaid >= pc.assistCap {
		return
	}
	if d == nil || d.Player != pc.assistPayer || len(chosen) != 1 || chosen[0].Kind != assistAnswerKind {
		return
	}
	amount := int32(chosen[0].Amount)
	if amount > 0 {
		// Price the promise against the same remaining bill the option list
		// was built from. The answer is already constrained to an offered
		// option by Decision.Validate; these clamps only keep a hand-built
		// answer from over-promising.
		if owed := e.castPaymentMana(pc).Generic; amount > owed {
			amount = owed
		}
		if cap := pay.AssistCap(asPayer(e), pc.assistPayer, amount); amount > cap {
			amount = cap
		}
	}
	if amount > 0 {
		pc.assistPaid += amount
		pc.assistPays = append(pc.assistPays, assistPayment{player: pc.assistPayer, amount: amount})
	}
	assistSeatDone(pc)
}

// assistDrainable is the pure precheck drainAssist needs before the caster
// pays anything: every promise must still be payable out of its seat's pool.
// Nothing can change those pools between the answer and the commit, so a false
// answer is a programme error the caller turns into a clean abort rather than
// a half-paid cast.
func (e *Engine) assistDrainable(pc *pendingCast) bool {
	if pc == nil {
		return true
	}
	payer := asPayer(e)
	for _, c := range pc.assistPays {
		if pay.AssistCap(payer, c.player, c.amount) < c.amount {
			return false
		}
	}
	return true
}

// drainAssist pays every promised contribution out of its OWN pool AFTER the
// caster's reduced payment has succeeded, so a cast that fails before the
// commit (including a caster who cannot finish the remaining bill) never
// takes a contributor's mana. pay.AssistPay emits the negative ManaAdd events
// that make a foreign seat's spend replayable.
func (e *Engine) drainAssist(pc *pendingCast) bool {
	if pc == nil || len(pc.assistPays) == 0 {
		return true
	}
	payer := asPayer(e)
	total := int32(0)
	for _, c := range pc.assistPays {
		paid := pay.AssistPay(payer, c.player, c.amount)
		if paid != c.amount {
			return false
		}
		total += paid
	}
	pc.assistPays = nil
	pc.assistPaid = total
	return true
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
