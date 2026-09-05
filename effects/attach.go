package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("Attach", effAttach)
	RegisterNonAPI("kw:Equip", "kw:Enchant", "kw:Living Weapon")
}

// Attachable reports whether obj may legally be attached to target. Task 14
// leaves this always-true: the full check includes "the target is not
// protected from the attachment's colours" (CR 702.16e for being attached
// despite protection), which Task 15 adds. The hook exists now so effAttach
// never hard-codes "always attach" -- the one decision the attachment SBA
// does not own on its own.
func Attachable(g *state.Game, obj state.ObjID, target state.ObjID) bool {
	_ = g
	return true
}

// effAttach implements "Attach": it fastens obj (Object$ Self by default --
// Aura's SP$ Attach is cast with Object$ Self so the STILL-ON-THE-STACK aura
// is the object being attached, Living Weapon's SVar is also Object$ Self
// with Defined$ Remembered naming the germ) onto the first Defined$ target
// that is a legal point of attachment: an object currently on the
// battlefield, not obj itself, and one Attachable accepts. The very first
// legal target wins, which is what makes the living-weapon shape work (the
// freshly minted germ is Remembered[0]).
//
// When no Defined$ target qualifies -- a player target (a player is never an
// attachment point), a non-battlefield object, obj itself, or nothing legal
// at all -- it refuses with a Note rather than emitting an Attach. The
// refusal is how the effect stays deterministic and observable while it has
// nothing legal to do.
func effAttach(h Host, c *Ctx, sa *cards.SA) {
	obj := c.Source
	switch sa.Params["Object"] {
	case "Remembered":
		if ts := objectsOf(c.Remembered); len(ts) > 0 {
			obj = ts[0].Obj
		}
	default: // "Self" (and the empty default) keep obj = c.Source.
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		target := t.Obj
		if target == obj {
			continue
		}
		tg := h.Game().Obj(target)
		if tg == nil || tg.Zone != state.ZBattlefield {
			continue
		}
		if !Attachable(h.Game(), obj, target) {
			continue
		}
		h.Emit(events.Event{Kind: events.Attach, Obj: obj, IDs: []state.ObjID{target}})
		return
	}
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "cannot attach: no legal target"})
}
