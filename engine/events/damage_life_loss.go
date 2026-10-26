package events

import "github.com/adams-shaun/gorge/engine/state"

// damageLifeLossMarker tags a Damage event's Pairs entry that carries the
// life loss the event actually causes when a LifeReduced replacement effect
// (CR 616.1) changed it. The Damage event's Amount stays the damage dealt --
// lifelink (CR 702.15e) and damage triggers read that -- while the life
// total, the turn's life-lost tally and the LifeLost triggers read this
// value instead. A real object id can never carry the bit (state.NextID
// starts at 1 and the range is far below it), so generic Pairs walkers that
// do not know the tag still see no object.
//
// The tag lives in the existing Pairs field, so the frozen Event struct and
// the log encoding are untouched; a log-only replay folds the same
// replacement-adjusted loss.
const damageLifeLossMarker = state.ObjID(1) << 29

// SetDamageLifeLoss records amount as the life loss ev causes, replacing any
// previously recorded value so chained LifeReduced replacements compose on
// the running total. It is a no-op for a non-Damage event.
func SetDamageLifeLoss(ev *Event, amount int32) {
	if ev == nil || ev.Kind != Damage {
		return
	}
	for i := range ev.Pairs {
		if ev.Pairs[i][0] == damageLifeLossMarker {
			ev.Pairs[i][1] = state.ObjID(uint32(amount))
			return
		}
	}
	ev.Pairs = append(ev.Pairs, [2]state.ObjID{damageLifeLossMarker, state.ObjID(uint32(amount))})
}

// DamageLifeLoss returns the life loss recorded on a Damage event by a
// LifeReduced replacement, and whether one was recorded. Without one the
// caller uses the event's Amount, which is the ordinary damage-to-life-loss
// exchange.
func DamageLifeLoss(ev Event) (int32, bool) {
	if ev.Kind != Damage {
		return 0, false
	}
	for i := range ev.Pairs {
		if ev.Pairs[i][0] == damageLifeLossMarker {
			return int32(ev.Pairs[i][1]), true
		}
	}
	return 0, false
}
