package effects

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// ReplaceUmbraArmor applies umbra armor (CR 702.90): if the permanent that
// would be destroyed wears an Aura with umbra armor, instead remove all
// damage from it and destroy that Aura. Consulted at every destruction
// choke point AFTER ReplaceDestruction — a bearer with both a regeneration
// shield and an umbra Aura spends the shield first (the deterministic
// stand-in for CR 616.1's controller choice). Umbra armor is NOT
// regeneration: it is never blocked by CantRegenerate/
// RegenerationDisallowed, and it applies even when the destroyer carries
// NoRegen$ True. Unlike the shield it is one-shot: the Aura itself is the
// charge, and once it is in the graveyard the next destruction is ordinary.
// The bearer is neither tapped nor removed from combat (CR 702.90 names only
// the two clauses), and the Aura's own destruction is part of the
// replacement, not a new replaceable one — no Indestructible or shield
// consultation for the Aura.
func ReplaceUmbraArmor(h Host, id state.ObjID) bool {
	aura := h.UmbraArmorAura(id)
	if aura == 0 {
		return false
	}
	o := h.Game().Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		return false
	}
	// (a) remove all damage from the bearer, plus the deathtouch mark — at
	// zero damage with no mark the lethal-damage SBA cannot re-kill it.
	if o.Damage > 0 {
		h.Emit(events.Event{Kind: events.Damage, Obj: id, Amount: -o.Damage})
	}
	if n := o.Counter("Deathtouched"); n > 0 {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "Deathtouched", Amount: -n})
	}
	// (b) destroy this Aura instead (CR 702.90). A plain MoveZone with the
	// same "destroyed" text every destruction path emits.
	h.Emit(events.Event{Kind: events.MoveZone, Obj: aura,
		From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	return true
}

// ReplaceDestruction consumes one regeneration shield (CR 701.16). Call only
// for regenerable destruction, after checking indestructible and NoRegen,
// never for other zone moves.
// Shield is the engine's regeneration marker, not the shield-counter mechanic.
func ReplaceDestruction(h Host, id state.ObjID) bool {
	o := h.Game().Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Counter("Shield") <= 0 {
		return false
	}
	// Task ce1: an Effect-registered CantRegenerate restriction (Incinerate)
	// forbids regeneration outright, so even a shield in place is never
	// consumed. This is the single choke point every destruction path
	// (effDestroy, effDestroyAll and rules/sba.go's lethal-damage sweep) funnels
	// through, so one check covers all of them rather than a bespoke per-caller
	// guard that a future destructor could quietly miss.
	if h.RegenerationDisallowed(id) {
		return false
	}
	h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "Shield", Amount: -1})
	if o.Damage > 0 {
		h.Emit(events.Event{Kind: events.Damage, Obj: id, Amount: -o.Damage})
	}
	if n := o.Counter("Deathtouched"); n > 0 {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "Deathtouched", Amount: -n})
	}
	if !o.Tapped {
		// The regenerated permanent's controller taps it (CR 701.19a; Forge
		// RegenerationEffect passes the card's controller as the tapper).
		h.EmitTap(id, o.Controller, false)
	}
	h.Emit(events.Event{Kind: events.EndCombatReset, Obj: id})
	return true
}
