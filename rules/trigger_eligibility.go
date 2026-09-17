package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
)

// triggerEventMask is only an over-approximation: an eligible trigger still
// runs every existing zone, phase, condition, batch and firing-limit gate.
// Event ordinals are unchanged. Future kinds beyond the mask go through the
// full matcher rather than being silently truncated by a shift.
type triggerEventMask uint64

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
		for _, t := range f.Triggers {
			// Phase diagnostics are event-visible and run on unrelated events
			// and in hidden zones too. Keep ALL Phase-bearing faces on the
			// original path, without caching whether a diagnostic was emitted.
			if strings.TrimSpace(t.Params["Phase"]) != "" {
				m = allTriggerEvents
				break
			}
			m |= triggerModeEvents(t.Mode)
		}
		if e.triggerEventMasks == nil {
			e.triggerEventMasks = make(map[*cards.Face]triggerEventMask)
		}
		e.triggerEventMasks[f] = m
	}
	return m.allows(kind)
}
