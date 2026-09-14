package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// rooms.go implements the Enchantment Room mechanic (CR 309, Forge's
// AlternateMode:Split two-face enchantments -- 128 corpus files). A Room is
// cast as ONE half (the engine's ordinary card-cast flow casts
// Faces[FaceIdx], i.e. the printed front half); that half's door is unlocked
// from entry and the ALTERNATE half (Faces[1]) stays locked and inert. As a
// sorcery the controller may pay the locked half's mana cost (its own
// Face.ManaCost, priced through the ordinary offerCostFor/castable gate) to
// unlock it: one DoorUnlock event, whose Apply flips the room's Unlocked
// flag. The unlock trigger (T:Mode$ UnlockDoor on the unlocked face, gated by
// ValidPlayer$ and ThisDoor$ True) queues off that same event, and the
// alternate face's ongoing rules text (triggers, statics, activated
// abilities) becomes live once unlocked -- the scans below and their callers
// in staticEffects / checkFaceTriggers / legalActions consult o.Unlocked.
//
// Face liveness convention: a room's live faces are its cast face (index 0)
// always, plus its alternate face (index 1) once unlocked. The three engine
// scans that read a permanent's face walk roomFaces; everything else keeps
// reading Face() -- for rooms, the readers that matter (targeting legality,
// P/T, types) read the cast face, which is correct: a room is one permanent,
// its characteristics are both halves' combined only in the CR-613 sense this
// build does not model, and no room half carries a P/T.
//
// Deliberately out of scope (reported in the ticket's Issues): casting a room
// by its ALTERNATE half (CR 309.4b "you may cast either half" -- the engine's
// cast flow always casts the front face), so a room whose unlock trigger
// lives on the FRONT face (Spiked Corridor: cast the back half, then unlock
// the front) can never fire its trigger here.

// roomLockedFace returns the locked alternate face of a room permanent, or
// nil when the object is not a room, has no alternate face, or is already
// unlocked.
func roomLockedFace(o *state.Object) *cards.Face {
	if o == nil || o.Unlocked || o.Card == nil || o.FaceIdx != 0 || len(o.Card.Faces) < 2 {
		return nil
	}
	f := o.Card.Faces[1]
	if !isRoomFace(o.Card.Faces[0]) {
		return nil
	}
	return f
}

// isRoomFace reports whether a face is a Room half (Enchantment Room). Both
// halves of a Forge AlternateMode:Split room carry the Room type.
func isRoomFace(f *cards.Face) bool {
	return f != nil && f.IsEnchantment() && f.IsRoom()
}

// isRoom reports whether the object is a Room permanent.
func isRoom(o *state.Object) bool {
	return o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].IsRoom()
}

// checkUnlockTriggers queues the unlocked face's T:Mode$ UnlockDoor triggers
// (CR 309.5's "when you unlock this door"). Only the DoorUnlock event's own
// room is scanned; ValidPlayer$ (You) is the room's controller, and
// ThisDoor$ True holds by construction -- the trigger belongs to the face
// whose door was just unlocked. A trigger that names no ValidPlayer$ still
// fires (its room is its owner's business); one whose ValidPlayer$ is
// anything but You degrades to no fire (fail-closed, the unhandled-qualifier
// convention MatchesPlayerSpec applies). The queue entry is the delayed-shape
// pendingTrigger (the ability is an SVar body reached through Execute$, which
// TriggerPush cannot carry for an alternate face).
func (e *Engine) checkUnlockTriggers(ev events.Event) {
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Zone != state.ZBattlefield || !isRoom(o) || !o.Unlocked {
		return
	}
	if len(o.Card.Faces) < 2 {
		return
	}
	alt := o.Card.Faces[1]
	if !isRoomFace(alt) {
		return
	}
	for _, t := range alt.Triggers {
		if t.Mode != "UnlockDoor" {
			continue
		}
		if vp := t.Params["ValidPlayer"]; vp != "" && vp != "You" {
			continue
		}
		if td := t.Params["ThisDoor"]; td != "" && !strings.EqualFold(td, "True") {
			continue
		}
		exec := t.Params["Execute"]
		if exec == "" || t.Effect == nil {
			continue
		}
		e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
			Source:     ev.Obj,
			Controller: o.Controller,
			Delayed:    true,
			DelayedID:  ^uint32(0),
			Execute:    exec,
			SA:         t.Effect,
			Ctx: effects.Ctx{
				Source:     ev.Obj,
				Controller: o.Controller,
			},
		})
	}
}

// unlockRoomCost returns the locked half's mana cost, parsed, for the offer
// and payment gate.
func (e *Engine) unlockRoomCost(o *state.Object) (Cost, bool) {
	f := roomLockedFace(o)
	if f == nil {
		return Cost{}, false
	}
	return ParseCost(f.ManaCost), true
}
