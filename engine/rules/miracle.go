// miracle.go implements the Miracle keyword (CR 702.94).
//
// Miracle is not a triggered ability in the T: sense -- it has no trigger
// line to fire -- but it shares the trigger machinery's queue. When a card
// with Miracle is the FIRST draw of its controller's turn, the draw queues a
// reveal offer. CR 702.94a: the owner may reveal the card as they draw it, and
// doing so creates a triggered ability; they may cast the card only as that
// trigger resolves. The sequence is reveal ask (placement, in the drain right
// after the Draw, before anyone has priority) -> a MiracleCast trigger on the
// stack -> priority, so every player may respond -> cast ask at resolution
// (askMiracleCast) -> the ordinary cast flow and payment.
//
// The offer is queued as a pendingTrigger with Miracle set and no Idx/SA (no
// T: line), so the drain (putTriggersOnStack, trigger_queue.go) treats it as
// an optional trigger whose decider is the owner: optionalDecider and
// triggerLabel special-case Miracle, askTriggerOptional asks the yes/no
// reveal, and a yes routes pushTrigger into revealMiracle below. A no drops
// the offer silently -- declining costs nothing and reveals nothing.

package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// offerMiracle is called from checkTriggers (trigger_match.go) after a Draw
// event. It queues a miracle offer for the drawn card iff the card carries
// the Miracle keyword and this is the first draw of its controller's turn.
//
// "First draw of the turn" is derived from the log (drawsThisTurn, below) --
// never from a field that only lives in memory -- so a log-only replay
// reaches exactly the same conclusion. The card is owned by ev.Player (the
// draw moved it from that player's library to their hand, so the object's
// Controller at this instant is also that player; ev.Player is the source of
// truth the interface names).
//
// The offer is queued as a trivial Miracle pendingTrigger. Idx/SA are zero/
// nil (there is no T: line); the drain and decider read the Miracle flag
// instead. The card at this point is guaranteed to be in the owner's hand
// (the draw just put it there); revealMiracle re-verifies before it reveals, in
// case the offer is still in the queue when something else moves the card.
func (e *Engine) offerMiracle(ev events.Event) {
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Face() == nil {
		return
	}
	if _, ok := e.derivedKeywordParamH(ev.Obj, kwhMiracle); !ok {
		return
	}
	if e.G.Turn == 0 || e.drawsThisTurn(ev.Player) != 1 {
		return // the opening deal, or not the first draw of the turn: no miracle.
	}
	e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
		Source:     ev.Obj,
		Controller: o.Controller,
		Miracle:    true,
		Ctx:        effects.NewCtx(ev.Obj, o.Controller, effects.CtxInit{}),
	})
}

// miracleCardCost is the granted spelling that names the card's own mana cost.
// "Miracle:ManaCost:4" is that cost less {4} (Aminatou, Veil Piercer).
const miracleCardCost = "ManaCost"

// miracleCost is card id's miracle cost, printed or granted in hand (Lorehold, the
// Historian's "miracle {2}").
func (e *Engine) miracleCost(id state.ObjID) (Cost, bool) {
	raw, ok := e.derivedKeywordParamH(id, kwhMiracle)
	if !ok {
		return Cost{}, false
	}
	less, cardCost := miracleCardCostLess(raw)
	if !cardCost {
		return ParseCost(cardsKeywordHead(raw)), true
	}
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return Cost{}, false
	}
	cost := ParseCost(o.Face().ManaCost)
	cost.Generic = max(0, cost.Generic-ParseCost(less).Generic)
	return cost, true
}

// miracleCardCostLess reads the card-cost spelling: the mana it takes off, and
// whether raw is that spelling at all.
func miracleCardCostLess(raw string) (string, bool) {
	if cardsKeywordHead(raw) != miracleCardCost {
		return "", false
	}
	if len(raw) <= len(miracleCardCost)+1 {
		return "", true
	}
	return raw[len(miracleCardCost)+1:], true
}

// drawsThisTurn counts Draw events for player p since the last TurnChange in
// the log (or since the start of the log, on turn 1). It is the "first card
// drawn this turn" oracle for offerMiracle's ==1 gate, and mirrors
// spellsCastThisTurn (cast.go) exactly -- a log walk, so a replay derives
// the same number. The opening deal is logged before the first TurnChange, so
// offerMiracle rules it out by turn number (CR 103.5: no turn has begun).
func (e *Engine) drawsThisTurn(p state.PlayerID) int {
	n := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.Draw && ev.Player == p {
			n++
		}
	}
	return n
}

// revealMiracle is called from pushTrigger (trigger_queue.go) when the owner
// answered the miracle reveal ask yes. It is the "place" step for a Miracle
// pendingTrigger: the card is revealed and the miracle triggered ability is
// minted as a real stack object (CR 702.94a), the way Madness mints its own.
//
// The card must still be in the owner's hand -- the offer was queued at draw
// time, and something (another effect, a state-based action, the owner being
// eliminated) may have moved it since; if it is gone the offer is dropped
// silently. Otherwise a Note carries the reveal (the text is parsed by
// bots/searchprobe/known.go, keep it byte-identical) and KeywordTriggerPush
// puts the MiracleCast ability on the stack. Nothing is cast here: the cast
// is asked when that ability resolves (askMiracleCast).
func (e *Engine) revealMiracle(pt pendingTrigger) {
	o := e.G.Obj(pt.Source)
	if o == nil || o.Controller != pt.Controller || o.Zone != state.ZHand {
		return
	}
	name := "it"
	if f := o.Face(); f != nil && f.Name != "" {
		name = f.Name
	}
	e.emit(events.Event{Kind: events.Note, Player: pt.Controller, IDs: []state.ObjID{pt.Source},
		Text: "reveals " + name + " (miracle)"})
	e.emit(events.Event{Kind: events.KeywordTriggerPush, Player: pt.Controller,
		Obj: pt.Source, Counter: "__kwMiracleCast", Text: "miracle cast"})
	e.drainAwaitsTarget = e.Pending() != nil
}

// askKeywordCast is resolveTop's hook for the two respondable keyword cast
// triggers (Madness, Miracle). handled reports that o is one of them; asked
// that a cast-or-not decision was posed and the resolution is spent.
func (e *Engine) askKeywordCast(o *state.Object) (handled, asked bool) {
	switch o.Ability.API {
	case "MadnessCast":
		return true, e.askMadnessCast(o)
	case "MiracleCast":
		return true, e.askMiracleCast(o)
	}
	return false, false
}

// miracleCastable reports whether id is still the card in p's hand that a
// miracle trigger may cast.
func miracleCastable(g *state.Game, p state.PlayerID, id state.ObjID) bool {
	o := g.Obj(id)
	return o != nil && o.Zone == state.ZHand && o.Owner == p
}

// miracleCostText is the cost as the reveal and cast asks print it.
func (e *Engine) miracleCostText(id state.ObjID) string {
	cost, _ := e.derivedKeywordParamH(id, kwhMiracle)
	if less, ok := miracleCardCostLess(cost); ok {
		cost = "its mana cost"
		if less != "" {
			cost += " less " + less
		}
	}
	return cost
}

// askMiracleCast is the miracle trigger's resolution (CR 702.94a): its owner
// may cast the revealed card, which must still be in their hand, for the
// miracle cost. Stifle reaches the ability before this runs; a card that left
// the hand meanwhile ends the ability with no choice (false).
func (e *Engine) askMiracleCast(ability *state.Object) bool {
	if ability == nil {
		return false
	}
	card := e.G.Obj(ability.Source)
	if card == nil || card.Face() == nil || card.Zone != state.ZHand || card.Owner != ability.Controller ||
		int(card.Owner) >= len(e.G.Players) || e.G.Players[card.Owner].Lost {
		return false
	}
	cost, ok := e.miracleCost(card.ID)
	if !ok {
		return false
	}
	name := card.Face().Name
	d := &decision.Decision{Player: card.Owner, Kind: decision.KTriggerOptional,
		Min: 1, Max: 1, Source: card.ID, ResumeKind: "miracle",
		Prompt: "Cast " + name + " for its miracle cost?"}
	if e.offerCastablePotential(card.Owner, card.ID, cost, spellScope("miracle"), false) &&
		e.castTargetsAvailable(card.Owner, card.ID, card.Face().SpellAbility()) {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "yes",
			Label: "Cast " + name + " for " + e.miracleCostText(card.ID), Obj: card.ID})
	}
	d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "no",
		Label: "Keep " + name + " in hand", Obj: card.ID})
	tapeMiracleAnswer(e, d, tapeServe(e, d), ability.ID)
	return true
}

// tapeMiracleAnswer runs the miracle answer (resolveMiracleChoice) in line for
// a tape-served cast-or-not choice, as tapeMadnessAnswer does for madness:
// both arms log their own CR 117.3b priority reset, and a cast that asks
// something (a target, a payment window) is a legacy ask that aborts the run.
func tapeMiracleAnswer(e *Engine, d *decision.Decision, in decision.Intent, ability state.ObjID) {
	chosen := d.Chosen(in)
	e.resolveMiracleChoice(&resumePoint{kind: "miracle", obj: ability}, len(chosen) == 1 && chosen[0].Kind == "yes")
	if e.cast != nil || e.pending != nil || e.Suspended() {
		tapeUnservable(e, "miracle cast")
	}
	e.tapeGranted = true
}

func (e *Engine) resolveMiracleChoice(rp *resumePoint, yes bool) {
	ability := e.G.Obj(rp.obj)
	if ability == nil {
		return
	}
	card, owner := ability.Source, ability.Controller
	e.finishResumption(rp.obj)
	// CR 117.3b: the trigger's resolution ends with the active player
	// receiving priority and a fresh pass count; the reset precedes the cast.
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
	if yes && miracleCastable(e.G, owner, card) {
		e.beginCast(owner, decision.Option{Kind: "cast", Obj: card, Mode: "miracle"})
	}
}

func init() {
	effects.RegisterNonAPI("kw:Miracle")
}
