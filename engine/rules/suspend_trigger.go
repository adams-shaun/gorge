package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Suspend (CR 702.62) as real triggered abilities. CR 702.62b's "At the
// beginning of your upkeep, if this card is suspended, remove a time counter
// from it" and CR 702.62a's "When the last time counter is removed from this
// card, if it's exiled, you may play it without paying its mana cost" are
// triggered abilities: they use the stack (CR 603.3), are respondable, and
// act only as they resolve. Both are minted as respondable keyword-triggered
// abilities through resolveTimeKeywordPush's __kwSuspendTick/__kwSuspendCast
// payloads (cards' builtinSVars), Source = the suspended card in exile; the
// queue sites are turn.go's upkeep walk (the tick) and emit.go's single
// TIME-counter hook (the may-cast, the one home every remover shares).

// queueSuspendCast queues CR 702.62a's may-cast trigger for the suspended
// card ev just took its last time counter from. It runs after the event's own
// triggers were matched (the caller, like queueSiegeDefeat), so the may-cast
// trigger follows the ones the same event made.
func queueSuspendCast(g *state.Game, pending *[]pendingTrigger, ev events.Event) {
	o := g.Obj(ev.Obj)
	if o == nil || o.Zone != state.ZExile || !hasSuspendAbility(o) || o.Counter("TIME") != 0 {
		return
	}
	*pending = append(*pending, pendingTrigger{
		Source:      o.ID,
		Controller:  o.Owner,
		SuspendLast: true,
		Ctx:         effects.NewCtx(o.ID, o.Owner, effects.CtxInit{}),
	})
}

// startSuspendTick is the upkeep tick trigger's resolution (resolveTop's
// SuspendTick hook): CR 603.4's intervening-if is rechecked here — a card
// that left exile, lost its suspend provenance or its last time counter
// while the trigger waited fizzles with no effect — and the tick removes
// exactly one TIME counter through the ordinary event, which is what feeds
// emit.go's may-cast hook when it was the last one.
func (e *Engine) startSuspendTick(id state.ObjID) {
	o := e.G.Obj(id)
	if o == nil || o.Ability == nil {
		return
	}
	card := e.G.Obj(o.Source)
	if card == nil || card.Zone != state.ZExile || !hasSuspendAbility(card) ||
		card.Counter("TIME") <= 0 {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id,
			From: state.ZStack, To: state.ZExile, Text: "fizzled: intervening-if no longer holds"})
		e.ensureLeftTheStack(id, state.ZExile, "a replacement fully discarded this "+
			"ability's 'fizzled: intervening-if' move without relocating it anywhere; sent to exile "+
			"instead of re-resolving forever")
		return
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: card.ID, Counter: "TIME", Amount: -1})
	e.finishResumption(id)
}

// askSuspendCast is the may-cast trigger's resolution (CR 702.62a): its
// owner — the suspended card's controller, the ability's controller — may
// play the card without paying its mana cost, "if able": a card a CantBeCast
// static forbids or whose spell can find no legal target is never offered,
// and there is nothing to choose (the SiegeDefeat pre-checks). A card that
// left exile, changed hands or lost its suspend provenance while the trigger
// waited ends the ability with no choice either (CR 702.62a's "if it's
// exiled", rechecked at resolution). handled by askKeywordCast; asked reports
// that the cast-or-not decision was posed.
func (e *Engine) askSuspendCast(ability *state.Object) bool {
	if ability == nil {
		return false
	}
	card := e.G.Obj(ability.Source)
	if card == nil || card.Face() == nil || card.Zone != state.ZExile ||
		!hasSuspendAbility(card) {
		return false
	}
	id, ctl := card.ID, ability.Controller
	if int(ctl) >= len(e.G.Players) || e.G.Players[ctl].Lost {
		return false
	}
	// "If able" includes every restriction that makes casting illegal, not
	// merely whether the spell can find a target. An uncastable card is never
	// offered: there is nothing to choose (CR 702.62a casts "if able"), so no
	// decision is posed and the ability resolves doing nothing.
	if e.castRestrictedFree(ctl, id) || !e.castTargetsAvailable(ctl, id, card.Face().SpellAbility()) {
		return false
	}
	name := "it"
	if f := card.Face(); f != nil && f.Name != "" {
		name = f.Name
	}
	d := &decision.Decision{Player: ctl, Kind: decision.KChoose, Min: 1, Max: 1, Source: id,
		ResumeKind: "suspend_cast", Prompt: "Cast " + name + " without paying its mana cost?",
		Options: []decision.Option{
			{Index: 0, Kind: "suspend_cast_yes", Obj: id, Label: "Cast it"},
			{Index: 1, Kind: "suspend_cast_no", Obj: id, Label: "Leave it in exile"}}}
	tapeSuspendAnswer(e, d, tapeServe(e, d), ability.ID)
	return true
}

// tapeSuspendAnswer applies the tape-served cast-or-not answer in line, as
// tapeSiegeDefeatAnswer does: the ability leaves the stack, CR 117.3b's
// priority reset precedes the cast (the resolution ends with the active
// player receiving priority and a fresh pass count), and a cast that asks
// something (a target, a payment window) is a legacy ask that aborts the
// run. A decline — or a card that is no longer the exiled suspended card the
// offer named — leaves the card in exile, which is exactly what CR 702.62a
// says a card whose cast was not made does.
func tapeSuspendAnswer(e *Engine, d *decision.Decision, in decision.Intent, ability state.ObjID) {
	chosen := d.Chosen(in)
	yes := len(chosen) == 1 && chosen[0].Kind == "suspend_cast_yes"
	a := e.G.Obj(ability)
	if a == nil {
		return
	}
	owner, card := a.Controller, d.Source
	e.finishResumption(ability)
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
	if o := e.G.Obj(card); yes && o != nil && o.Zone == state.ZExile &&
		hasSuspendAbility(o) && o.Face() != nil {
		e.beginCast(owner, decision.Option{Kind: "cast", Obj: card, Mode: "suspend_cast"})
	}
	if e.cast != nil || e.pending != nil || e.Suspended() {
		tapeUnservable(e, "suspend cast")
	}
	e.tapeGranted = true
}
