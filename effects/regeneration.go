package effects

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// ReplaceDestruction consumes one regeneration shield (CR 701.16). Call only
// for destruction, after checking indestructible, never for other zone moves.
// Shield is the engine's regeneration marker, not the shield-counter mechanic.
func ReplaceDestruction(h Host, id state.ObjID) bool {
	o := h.Game().Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Counter("Shield") <= 0 {
		return false
	}
	h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "Shield", Amount: -1})
	if o.Damage > 0 {
		h.Emit(events.Event{Kind: events.Damage, Obj: id, Amount: -o.Damage})
	}
	if n := o.Counter("Deathtouched"); n > 0 {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "Deathtouched", Amount: -n})
	}
	h.Emit(events.Event{Kind: events.Tap, Obj: id})
	h.Emit(events.Event{Kind: events.EndCombatReset, Obj: id})
	return true
}
