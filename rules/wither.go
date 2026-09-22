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
