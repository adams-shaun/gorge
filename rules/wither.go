package rules

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// convertWitherDamage places the -1/-1 counters that replace damage from a
// Wither source. The Damage event has already passed prevention and amount
// replacements, so its amount is the amount that actually landed. Keeping the
// placement as a separate emitted event preserves counter replacements and
// CounterAdded triggers.
// recomputeWitherMarker preserves the source's Wither fact while deriving
// the recipient form after DamageDone replacement effects have run. Unlike
// infect, Wither has no player form: a player hit remains ordinary damage,
// while a redirected hit onto a battlefield creature becomes counters.
// stat:WitherDamage (Everlasting Torment, CR 702.90c): while an active
// battlefield static with Mode$ WitherDamage applies, every source deals its
// damage as though it had Wither, so an otherwise ordinary hit on a creature
// enters the same "wither+creature" marker here (a player, planeswalker or
// battle hit stays ordinary, exactly like a keyword Wither source's).
func (e *Engine) recomputeWitherMarker(ev *events.Event) {
	if ev == nil || ev.Kind != events.Damage {
		return
	}
	if ev.Counter != "wither" && ev.Counter != "wither+creature" {
		// Not a keyword-Wither emitter: only the global static can convert
		// this hit. Only a creature recipient converts; only a positive
		// amount (the cleanup and regeneration negatives that CLEAR marked
		// damage must keep taking the ordinary fold branch that decrements
		// it, and a prevented hit never reaches here as a Damage event).
		if ev.Amount <= 0 || ev.Obj == 0 || !e.witherDamageStaticActive() {
			return
		}
		o := e.G.Obj(ev.Obj)
		if o == nil || o.Zone != state.ZBattlefield || !e.IsCreature(ev.Obj) {
			return
		}
		ev.Counter = "wither+creature"
		return
	}
	if ev.Obj != 0 {
		if o := e.G.Obj(ev.Obj); o != nil && o.Zone == state.ZBattlefield && e.IsCreature(ev.Obj) {
			ev.Counter = "wither+creature"
		} else {
			ev.Counter = "wither"
		}
		return
	}
	// Player damage is ordinary Wither damage.
	ev.Counter = "wither"
}

func (e *Engine) convertWitherDamage(ev events.Event) {
	if ev.Obj == 0 || ev.Amount <= 0 {
		return
	}
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Zone != state.ZBattlefield || !e.IsCreature(ev.Obj) {
		return
	}
	var saved state.PlayerID
	published := false
	if src := e.inFlightDamageSource(); src != 0 {
		if c := e.controllerOf(src); int(c) >= 0 && int(c) < len(e.G.Players) {
			saved = e.SetCounterAdder(c)
			published = true
		}
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: ev.Obj, Counter: "M1M1", Amount: ev.Amount})
	if published {
		e.SetCounterAdder(saved)
	}
}

// witherDamageStaticActive reports whether any battlefield permanent carries
// an active S:Mode$ WitherDamage static (Everlasting Torment). The static has
// no Valid$ parameters in the corpus, so any live instance applies to all
// damage; the canonical activeStatics walk keeps face-down, phased-out and
// EffectZone$ exclusions consistent with every other static consumer.
func (e *Engine) witherDamageStaticActive() bool {
	return len(e.activeStatics("WitherDamage")) > 0
}
