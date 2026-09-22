package rules

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// convertInfectDamage places the counters CR 702.90b deals an infect
// source's damage in, called from Engine.emit immediately after an
// infect-marked Damage event has folded. The Damage event itself keeps every
// semantic it always had -- prevention/replacement rewrote it before it
// folded, DamageDone triggers and lifelink read the amount that landed, the
// combat-damage ledger and the walker loyalty exchange (a separate, fold
// conversion) are unaffected -- and the FORM the rule rewrites arrives as a
// real event through THIS same emit, so the repl:AddCounter class (a Winding
// Constrictor doubler, a CantPutCounter lock) and trig:CounterAdded see the
// placement exactly like any other counter placement, and rules/sba.go's CR
// 704.5b ten-poison loss reads a real PlayerCounterChange fold.
//
// The form is read HERE, off the event that actually landed and the
// recipient's live layer state: a creature recipient (the compound
// "infect+creature" marker, the emitter's layer-accurate classification the
// fold reuses) takes -1/-1 counters, a player takes poison counters, and any
// other object -- an artifact, a Battle, a printed planeswalker, a hit a
// redirect moved to a non-creature -- takes the damage in its ordinary form,
// which the fold already applied. A recipient that left the battlefield
// before the fold places nothing: damage dealt to an object that is no
// longer there marks nothing and places no counters either.
func (e *Engine) convertInfectDamage(ev events.Event) {
	if ev.Obj != 0 {
		o := e.G.Obj(ev.Obj)
		if o == nil || o.Zone != state.ZBattlefield || !e.IsCreature(ev.Obj) {
			return
		}
		e.emitInfectCounters(events.Event{Kind: events.CounterChange, Obj: ev.Obj,
			Counter: "M1M1", Amount: ev.Amount})
		return
	}
	if int(ev.Player) >= len(e.G.Players) {
		return
	}
	e.emitInfectCounters(events.Event{Kind: events.PlayerCounterChange,
		Player: ev.Player, Counter: "POISON", Amount: ev.Amount})
}

// emitInfectCounters emits one conversion placement, publishing the damage
// source's controller as the placement's adder while the event is in flight
// (the repl:AddCounter class's ValidSource$ role -- an opponent's infect
// creature is who is putting the counters on your board, so a
// ValidSource$-scoped line reads that player, never the recipient). A hit
// with no recorded damage source publishes nothing, and the AddCounter
// matcher then fails its ValidSource$ line closed -- the standing
// no-provenance convention -- rather than guessing.
func (e *Engine) emitInfectCounters(ev events.Event) {
	var saved state.PlayerID
	published := false
	if src := e.inFlightDamageSource(); src != 0 {
		if c := e.controllerOf(src); int(c) >= 0 && int(c) < len(e.G.Players) {
			saved = e.SetCounterAdder(c)
			published = true
		}
	}
	e.emit(ev)
	if published {
		e.SetCounterAdder(saved)
	}
}
