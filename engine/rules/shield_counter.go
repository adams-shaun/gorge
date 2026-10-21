package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 122.1c: one or more shield counters on a permanent create a single
// prevention effect, "If damage would be dealt to this permanent, prevent
// that damage and remove a shield counter from it." (The destruction half
// is effects.ReplaceShieldCounter, consulted by the Destroy effects.)
//
// The prevention is one more DamageDone replacement, collected beside the
// granted Prevent keywords so it competes under CR 616.1 like any other and
// is skipped under stat:CantPreventDamage (damageReplacementPrevents reads
// its Prevent$ True). It reuses preventToAny's parameter table (no new map)
// under its own pointer, which is how its application is recognised and the
// counter spent. The engine's "Shield" regeneration marker is a different
// counter kind and is never read here.
var shieldCounterDamageRepl = &cards.Repl{Event: "DamageDone", Params: preventToAny.Params}

// shieldCounterDamageMatches returns the shield-counter prevention for a
// Damage event dealt to a battlefield permanent carrying a SHIELD counter.
func (e *Engine) shieldCounterDamageMatches(ev events.Event, out []replMatch) []replMatch {
	if ev.Kind != events.Damage || ev.Obj == 0 || ev.Amount <= 0 {
		return out
	}
	if o := e.G.Obj(ev.Obj); o == nil || o.Zone != state.ZBattlefield || o.Counter("SHIELD") <= 0 {
		return out
	}
	if !e.replacementMatches(*shieldCounterDamageRepl, ev.Obj, ev) {
		return out
	}
	return append(out, replMatch{id: ev.Obj, repl: shieldCounterDamageRepl})
}

// spendShieldCounter is the "remove a shield counter from it" half, run
// right after the prevention's Note when the applied match is the shield
// counter's.
func (e *Engine) spendShieldCounter(m replMatch) {
	if m.repl != shieldCounterDamageRepl {
		return
	}
	if o := e.G.Obj(m.id); o != nil && o.Counter("SHIELD") > 0 {
		e.emit(events.Event{Kind: events.CounterChange, Obj: m.id, Counter: "SHIELD", Amount: -1})
	}
}
