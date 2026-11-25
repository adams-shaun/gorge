package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// siegeDefeatCounter is the KeywordTriggerPush payload of CR 310.12b's
// intrinsic Siege ability: "When the last defense counter is removed from this
// permanent, exile it, then you may cast it transformed without paying its
// mana cost." cards' builtinSVars names its body (DB$ SiegeDefeat), which
// askKeywordCast routes to askSiegeDefeat when the stack object resolves.
//
// The defeat is a TRIGGERED ABILITY, not a state-based action: it queues with
// the other triggers the same event made (CR 603.3b APNAP and the owner's
// ordering), is respondable, and does its exile and its optional cast only as
// it resolves. A Siege at 0 defense that is the source of such an ability
// that has not left the stack is spared by the CR 704.5v SBA
// (battleZeroDefense).
const siegeDefeatCounter = "__kwSiegeDefeat"

// siegeDefeatAPI is the body's API name (cards' builtinSVars).
const siegeDefeatAPI = "SiegeDefeat"

// The counter kind a Siege's defense is, and the two options of its cast ask.
const (
	defenseCounter = "DEFENSE"
	defeatCastYes  = "defeat_cast_yes"
	defeatCastNo   = "defeat_cast_no"
)

// resolveTimeKeywordPush names the KeywordTriggerPush a mandatory keyword
// trigger whose choice is made as its stack object resolves mints: Madness's
// cast-or-graveyard, the Siege's exile-then-cast, and suspend's two real
// triggered abilities (CR 702.62a/b — the upkeep tick rules resolves and the
// last-counter may-cast rules asks, engine/rules/suspend_trigger.go).
func (pt pendingTrigger) resolveTimeKeywordPush() (counter, text string, ok bool) {
	switch {
	case pt.Madness:
		return "__kwMadnessCast", "madness cast", true
	case pt.Siege:
		return siegeDefeatCounter, "siege defeated", true
	case pt.SuspendTick:
		return "__kwSuspendTick", "suspend tick", true
	case pt.SuspendLast:
		return "__kwSuspendCast", "suspend last-counter", true
	}
	return "", "", false
}

// siegeDefenseBefore is the defense count the Siege battle ev would hurt
// holds before ev folds, or -1 when ev cannot take a Siege's last defense
// counter: a negative DEFENSE CounterChange (a removal) or a positive Damage
// event (CR 120.3h: damage to a battle removes that many counters, folded in
// events.Apply) aimed at a face-up Siege on the battlefield. The pre-fold read
// is what makes "the LAST counter" precise (a Siege already at 0 does not
// trigger again).
func siegeDefenseBefore(g *state.Game, ev events.Event) int32 {
	switch ev.Kind {
	case events.CounterChange:
		if ev.Amount >= 0 || ev.Counter != defenseCounter {
			return -1
		}
	case events.Damage:
		if ev.Amount <= 0 || ev.Obj == 0 {
			return -1
		}
	default:
		return -1
	}
	o := g.Obj(ev.Obj)
	if o == nil || !isFaceUpSiege(o) {
		return -1
	}
	return o.Counter(defenseCounter)
}

// isFaceUpSiege reports whether o is a phased-in, face-up, front-face Siege
// battle on the battlefield (the only permanent CR 310.12b's ability belongs
// to: a battle cast transformed is a nonbattle permanent).
func isFaceUpSiege(o *state.Object) bool {
	if o.Zone != state.ZBattlefield || o.PhasedOut || o.FaceDown || o.FaceIdx != 0 {
		return false
	}
	f := o.Face()
	return f != nil && f.IsBattle() && f.HasTypeWord("Siege")
}

// queueSiegeDefeat queues CR 310.12b's trigger for the Siege ev just took
// its last defense counter from. before is siegeDefenseBefore's pre-fold
// read; it runs after the event's own triggers were matched, so the Siege's
// trigger follows the ones the same event made.
func queueSiegeDefeat(g *state.Game, pending *[]pendingTrigger, ev events.Event, before int32) {
	if before <= 0 {
		return
	}
	o := g.Obj(ev.Obj)
	if o == nil || !isFaceUpSiege(o) || o.Counter(defenseCounter) != 0 {
		return
	}
	*pending = append(*pending, pendingTrigger{
		Source:     o.ID,
		Controller: o.Controller,
		Siege:      true,
		Ctx:        effects.NewCtx(o.ID, o.Controller, effects.CtxInit{}),
	})
}

// siegeDefeatPending reports whether id is the source of a CR 310.12b
// ability that has triggered but not yet left the stack (CR 704.5v): queued
// and waiting to be put on the stack, or on it.
func siegeDefeatPending(g *state.Game, pending []pendingTrigger, id state.ObjID) bool {
	for i := range pending {
		if pt := &pending[i]; pt.Siege && pt.Source == id {
			return true
		}
	}
	for _, sid := range g.Stack {
		if so := g.Obj(sid); so != nil && so.Ability != nil && so.Source == id && so.Ability.API == siegeDefeatAPI {
			return true
		}
	}
	return false
}

// askSiegeDefeat resolves the Siege's defeat ability (CR 310.12b): exile the
// permanent, then ask its controller whether to cast it transformed without
// paying its mana cost. handled by askKeywordCast; asked reports that the
// choice was posed and answered (the ability is spent), false that it ends
// with no choice (the Siege is gone, or the cast is impossible, which leaves
// it in exile).
//
// The pre-checks are the old defeat queue's "if able": a card a CantBeCast
// static forbids or whose back face has no legal target is not offered.
func (e *Engine) askSiegeDefeat(ability *state.Object) bool {
	if ability == nil {
		return false
	}
	card := e.G.Obj(ability.Source)
	if card == nil || !isFaceUpSiege(card) {
		return false
	}
	id := card.ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile, Text: "defeated"})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZExile || o.Face() == nil || !o.Face().IsBattle() ||
		o.FaceIdx != 0 || o.Card == nil || len(o.Card.Faces) < 2 {
		return false
	}
	ctl := ability.Controller
	if int(ctl) >= len(e.G.Players) || e.G.Players[ctl].Lost {
		return false
	}
	back := o.Card.Faces[1]
	if e.castRestrictedFree(ctl, id) {
		return false
	}
	if back != nil && back.SpellAbility() != nil && !e.targetsAvailable(ctl, id, id, back.SpellAbility(), false) {
		return false
	}
	name := "it"
	if back != nil && back.Name != "" {
		name = back.Name
	}
	d := &decision.Decision{Player: ctl, Kind: decision.KChoose, Min: 1, Max: 1, Source: id,
		ResumeKind: "siege_defeat", Prompt: "Cast " + name + " (transformed) without paying its mana cost?",
		Options: []decision.Option{
			{Index: 0, Kind: defeatCastYes, Obj: id, Label: "Cast it"},
			{Index: 1, Kind: defeatCastNo, Obj: id, Label: "Leave it in exile"}}}
	tapeSiegeDefeatAnswer(e, d, tapeServe(e, d), ability.ID)
	return true
}

// tapeSiegeDefeatAnswer applies the tape-served cast-or-not answer in line,
// as tapeMadnessAnswer does: the ability leaves the stack, CR 117.3b's
// priority reset precedes the cast, and a cast that asks something (a target,
// a payment window) is a legacy ask that aborts the run. A decline leaves the
// card in exile, front face up.
func tapeSiegeDefeatAnswer(e *Engine, d *decision.Decision, in decision.Intent, ability state.ObjID) {
	chosen := d.Chosen(in)
	yes := len(chosen) == 1 && chosen[0].Kind == defeatCastYes
	a := e.G.Obj(ability)
	if a == nil {
		return
	}
	owner, card := a.Controller, d.Source
	e.finishResumption(ability)
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
	if o := e.G.Obj(card); yes && o != nil && o.Zone == state.ZExile && o.FaceIdx == 0 && o.Card != nil && len(o.Card.Faces) > 1 {
		e.beginCast(owner, decision.Option{Kind: "cast", Obj: card, Mode: "defeat_cast"})
	}
	if e.cast != nil || e.pending != nil || e.Suspended() {
		tapeUnservable(e, "siege defeat cast")
	}
	e.tapeGranted = true
}
