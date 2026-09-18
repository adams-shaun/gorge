package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// triggerEventMask is only an over-approximation: an eligible trigger still
// runs every existing zone, phase, condition, batch and firing-limit gate.
// Event ordinals are unchanged. Future kinds beyond the mask go through the
// full matcher rather than being silently truncated by a shift.
type triggerEventMask uint64

type objectTriggerEventMasks struct {
	faces [2]*cards.Face
	masks [2]triggerEventMask
}

const allTriggerEvents triggerEventMask = ^triggerEventMask(0)

func (m triggerEventMask) allows(kind events.Kind) bool {
	return kind >= 64 || m&(1<<kind) != 0
}

// Keep this aligned with triggerMatches' actual dispatch, not with a wider
// interpretation of Forge mode names. Unknown modes retain the old path so
// adding a matcher cannot silently lose triggers before this table catches up.
func triggerModeEvents(mode string) triggerEventMask {
	switch mode {
	case "ChangesZone":
		return 1<<events.MoveZone | 1<<events.Draw | 1<<events.PutOnStack
	case "SpellCast":
		return 1 << events.PutOnStack
	case "AbilityCast", "SpellAbilityCast":
		return 1 << events.AbilityPush
	case "Attacks", "AttackersDeclaredOneTarget", "AttackersDeclared":
		return 1 << events.DeclareAttackers
	case "AttackerBlocked":
		return 1 << events.DeclareBlockers
	case "Sacrificed", "Discarded", "LandPlayed":
		return 1 << events.MoveZone
	case "Cycled":
		return 1 << events.MoveZone
	case "CommitCrime", "BecomesTarget":
		return 1 << events.TargetsChosen
	case "Taps", "TapsForMana":
		return 1 << events.Tap
	case "DamageDone", "DamageDealtOnce", "DamageDoneOnce":
		return 1 << events.Damage
	case "CounterAdded":
		return 1 << events.CounterChange
	case "Drawn":
		return 1 << events.Draw
	case "LifeLost":
		return 1<<events.Damage | 1<<events.LifeChange
	case "Phase":
		return 1 << events.StepChange
	default:
		// Always reads state on every event. LifeLostAll also has a batch-
		// finishing entry point; leave its existing gates authoritative.
		return allTriggerEvents
	}
}

func grantedKeywordTriggerEvent(kind events.Kind) bool {
	return kind == events.TargetsChosen || kind == events.DeclareAttackers
}

func triggerMaskForFace(f *cards.Face) triggerEventMask {
	if f == nil || len(f.Triggers) == 0 {
		return 0
	}
	var m triggerEventMask
	for _, t := range f.Triggers {
		// Phase diagnostics are event-visible and run on unrelated events
		// and in hidden zones too. Keep ALL Phase-bearing faces on the
		// original path, without caching whether a diagnostic was emitted.
		if strings.TrimSpace(t.Params["Phase"]) != "" {
			return allTriggerEvents
		}
		m |= triggerModeEvents(t.Mode)
	}
	return m
}

// faceMayTrigger caches immutable printed eligibility, never object/zone
// membership or a dynamic match. New fixture objects, token faces, transforms
// and Room unlocks therefore need no invalidation. The LIVE queue owner owns
// the map even when matching against a scratch look-back observer; no snapshot
// cache is shared or mutated. Clones start with an independent, empty map.
func (e *Engine) faceMayTrigger(f *cards.Face, kind events.Kind) bool {
	if f == nil || len(f.Triggers) == 0 {
		return false
	}
	m, ok := e.triggerEventMasks[f]
	if !ok {
		m = triggerMaskForFace(f)
		if e.triggerEventMasks == nil {
			e.triggerEventMasks = make(map[*cards.Face]triggerEventMask)
		}
		e.triggerEventMasks[f] = m
	}
	return m.allows(kind)
}

// objectFaceMayTrigger is the object-walk fast path. Object IDs are dense, and
// the two face slots stay stable for the immutable lifetime of a Card, so the
// repeated event scan can avoid hashing a face pointer. The pointer check keeps
// synthetic face replacement and transforms safe; uncommon faces beyond the
// two-face card model use the conservative face cache above.
func (e *Engine) objectFaceMayTrigger(id state.ObjID, faceIdx uint8, f *cards.Face, kind events.Kind) bool {
	if f == nil {
		return false
	}
	if id == 0 || faceIdx >= 2 {
		return e.faceMayTrigger(f, kind)
	}
	i := int(id) - 1
	if i >= len(e.triggerObjectMasks) {
		e.triggerObjectMasks = append(e.triggerObjectMasks, make([]objectTriggerEventMasks, i+1-len(e.triggerObjectMasks))...)
	}
	entry := &e.triggerObjectMasks[i]
	if entry.faces[faceIdx] != f {
		entry.faces[faceIdx] = f
		entry.masks[faceIdx] = triggerMaskForFace(f)
	}
	return entry.masks[faceIdx].allows(kind)
}
