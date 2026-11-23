package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
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
	return replaceUmbraArmor(h, id, h.UmbraArmorAura(id))
}

// replaceUmbraArmor is ReplaceUmbraArmor with the Aura chosen by the caller.
// A simultaneous destroy batch reads the Aura before its first move, so the
// bearer is saved even when the Aura is itself a victim; the zone check below
// keeps that Aura from being moved twice (CR 702.89a: the same as destroying it
// once).
func replaceUmbraArmor(h Host, id, aura state.ObjID) bool {
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
	// same "destroyed" text every destruction path emits. An Aura that an
	// earlier batch member already moved is not moved again.
	if a := h.Game().Obj(aura); a != nil && a.Zone == state.ZBattlefield {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: aura,
			From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	}
	return true
}

// umbraBatchAuras reads every victim's umbra Aura before a simultaneous
// destroy batch's first move. A destroy batch destroys the Aura and the
// creature it enchants at the same time; CR 702.89a still saves the creature,
// and the Aura is destroyed once rather than twice. Without the pre-read, the
// answer would depend on whether the Aura came first in the victim order (the
// Relic Crush repro).
func umbraBatchAuras(h Host, victims []state.ObjID) map[state.ObjID]state.ObjID {
	if len(victims) < 2 {
		return nil
	}
	var out map[state.ObjID]state.ObjID
	for _, id := range victims {
		if aura := h.UmbraArmorAura(id); aura != 0 {
			if out == nil {
				out = make(map[state.ObjID]state.ObjID, len(victims))
			}
			out[id] = aura
		}
	}
	return out
}

// ReplaceShieldCounter applies CR 122.1c's destruction half: "If this
// permanent would be destroyed as the result of an effect, instead remove a
// shield counter from it." Consulted by the Destroy effects and by the
// lethal-damage SBA (engine/rules/sba.go), after the Indestructible skip and
// the regeneration shield and before umbra armor --
// the deterministic stand-in for CR 616.1's controller choice: the
// this-turn regeneration shield is spent before the lasting counter, and the
// counter before the Aura. NoRegen$ does not gate it: it is not
// regeneration. The counter kind is "SHIELD", never the engine's "Shield"
// regeneration marker.
func ReplaceShieldCounter(h Host, id state.ObjID) bool {
	o := h.Game().Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Counter("SHIELD") <= 0 {
		return false
	}
	h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "SHIELD", Amount: -1})
	return true
}

// ReplaceDestroyLine offers a destruction to the R:Event$ Destroy lines, before
// the shield-like replacements below: the same deterministic stand-in for CR
// 616.1's controller choice they use among themselves. regenerable is false for
// a destruction that can't be regenerated from, which a Regeneration$ line
// doesn't apply to.
func ReplaceDestroyLine(h Host, id state.ObjID, regenerable bool) bool {
	ev := events.Event{Kind: events.DestroyProposal, Obj: id}
	if regenerable {
		ev.Amount = 1
	}
	return h.ActionReplaced(ev)
}

// effRegeneration is the body of a "would be destroyed, regenerate it" line: the
// regeneration itself (CR 701.19a), with no shield involved.
func effRegeneration(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		if o := h.Game().Obj(t.Obj); !t.IsPlayer && o != nil && o.Zone == state.ZBattlefield {
			regenerate(h, o)
		}
	}
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
	// An Effect-registered CantRegenerate restriction (Incinerate)
	// forbids regeneration outright, so even a shield in place is never
	// consumed. This is the single choke point every destruction path
	// (effDestroy, effDestroyAll and engine/rules/sba.go's lethal-damage sweep) funnels
	// through, so one check covers all of them rather than a bespoke per-caller
	// guard that a future destructor could quietly miss.
	if h.RestrictionApplies("CantRegenerate", id) {
		return false
	}
	h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "Shield", Amount: -1})
	regenerate(h, o)
	return true
}

// regenerate heals o, taps it and removes it from combat (CR 701.19a).
func regenerate(h Host, o *state.Object) {
	id := o.ID
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
	h.Emit(events.Event{Kind: events.EndCombatReset, Obj: id, Text: events.RegeneratedNotice})
}
