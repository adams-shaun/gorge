package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 731 day/night designation. The state is Game.DayNight, folded from
// DayNightChange events; this file holds the three places the rules change it.

var kwhDaybound, kwhNightbound = newKWHead("Daybound"), newKWHead("Nightbound")

func init() {
	// The coverage census: kw:Daybound / kw:Nightbound are read rules-side off
	// the K: line (syncDayNightFaces, dayNightStateCheck, the night-entry fold
	// in engine/events/daynight_entry.go and the can't-transform gate in
	// engine/effects/daynight_bound.go), so they are registered here exactly as
	// mayflashsac.go registers kw:MayFlashSac.
	effects.RegisterNonAPI("kw:Daybound", "kw:Nightbound")
}

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
	// CR 702.145c/f: "as it becomes night/day" the permanents transform.
	e.syncDayNightFaces()
}

// syncDayNightFaces is CR 702.145c / 702.145f: while it is night every
// phased-in, face-up transforming DFC wearing its front face with daybound
// transforms to its back face, and while it is day every such permanent
// wearing its back face with nightbound transforms to its front face. Each is
// a CR 701.27 transform (one FlipFace, Text "Transformed"), emitted so the
// CR 616 "as this transforms" pass and "whenever this transforms" triggers see
// it. A permanent ENTERING at night is not transformed here: it enters back
// face up through the fold in engine/events/daynight_entry.go (CR 702.145b).
func (e *Engine) syncDayNightFaces() {
	for _, ev := range e.dayNightFlips() {
		e.emit(ev)
	}
}

// dayNightFlips lists the transforms syncDayNightFaces performs.
func (e *Engine) dayNightFlips() []events.Event {
	dn := e.G.DayNight
	if dn == state.DayNightNeither {
		return nil
	}
	var flips []events.Event
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil || o.PhasedOut || o.FaceDown || o.Card == nil || !o.Card.IsTransformingDFC() {
				continue
			}
			to := int32(-1)
			switch {
			case dn == state.DayNightNight && o.FaceIdx == 0 && e.hasKeywordH(id, kwhDaybound):
				to = 1
			case dn == state.DayNightDay && o.FaceIdx != 0 && e.hasKeywordH(id, kwhNightbound):
				to = 0
			}
			if to >= 0 {
				flips = append(flips, events.Event{Kind: events.FlipFace, Obj: id, Amount: to, Text: "Transformed"})
			}
		}
	}
	return flips
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
// phased-in nightbound permanent makes it night. With a designation it is
// CR 702.145c/f's "any time" re-check of the faces. A designation it queues
// transforms the faces on the next pass's re-check.
func (e *Engine) dayNightStateCheck(b *sbaBatch) {
	if e.G.DayNight != state.DayNightNeither {
		b.acts = append(b.acts, e.dayNightFlips()...)
		return
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
		b.add(events.Event{Kind: events.DayNightChange, Amount: int32(state.DayNightDay), Text: state.DayNightDay.String()})
	case night:
		b.add(events.Event{Kind: events.DayNightChange, Amount: int32(state.DayNightNight), Text: state.DayNightNight.String()})
	}
}
