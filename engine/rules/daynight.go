package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 731 day/night designation. The state is Game.DayNight, folded from
// DayNightChange events; this file holds the three places the rules change it.

var kwhDaybound, kwhNightbound = newKWHead("Daybound"), newKWHead("Nightbound")

// dayNightFace reports whether the printed face carries daybound or
// nightbound. No corpus line grants either keyword, so the printed face is a
// superset of the derived keyword set: a prefilter, never the final answer.
func dayNightFace(f *cards.Face) bool {
	return f.KeywordLinesHaveHead(kwhDaybound.S, kwhDaybound.ID) ||
		f.KeywordLinesHaveHead(kwhNightbound.S, kwhNightbound.ID)
}

// setDayNight is the single emitter of a designation change.
func (e *Engine) setDayNight(d state.DayNight) {
	if e.G.DayNight == d {
		return
	}
	e.emit(events.Event{Kind: events.DayNightChange, Amount: int32(d), Text: d.String()})
}

// dayNightUntapCheck is CR 731.2 / 502.2 / 703.4b: in the untap step, after
// phasing and before the untap action, it becomes night if it is day and the
// previous turn's active player cast no spells (731.2a), and day if it is night
// and that player cast two or more (731.2b).
func (e *Engine) dayNightUntapCheck() {
	switch e.G.DayNight {
	case state.DayNightDay:
		if e.G.PrevTurnSpells == 0 {
			e.setDayNight(state.DayNightNight)
		}
	case state.DayNightNight:
		if e.G.PrevTurnSpells >= 2 {
			e.setDayNight(state.DayNightDay)
		}
	}
}

// dayNightStateCheck is CR 702.145d / 702.145g: with neither designation, a
// phased-in daybound permanent makes it day, and with none of those a
// phased-in nightbound permanent makes it night. It reports whether it
// changed the designation.
func (e *Engine) dayNightStateCheck() bool {
	if e.G.DayNight != state.DayNightNeither {
		return false
	}
	day, night := false, false
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil || o.PhasedOut {
				continue
			}
			if e.hasKeywordH(id, kwhDaybound) {
				day = true
			} else if e.hasKeywordH(id, kwhNightbound) {
				night = true
			}
		}
	}
	switch {
	case day:
		e.setDayNight(state.DayNightDay)
		return true
	case night:
		e.setDayNight(state.DayNightNight)
		return true
	}
	return false
}
