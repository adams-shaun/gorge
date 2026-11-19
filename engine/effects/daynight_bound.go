package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// dayNightBound reports whether id has daybound or nightbound as a derived
// keyword. CR 702.145b/e: such a permanent can't transform except by its own
// daybound/nightbound ability, so SetState Mode$ Transform leaves it alone.
func dayNightBound(h Host, id state.ObjID) bool {
	return h.HasKeyword(id, "Daybound") || h.HasKeyword(id, "Nightbound")
}

func init() { Register("DayTime", effDayTime) }

// effDayTime is DB$ DayTime (CR 731.1): Value$ Day or Night makes it that
// designation, Value$ Switch turns day into night and night into day (and
// leaves neither alone). An unchanged designation emits nothing, the same
// once-per-change discipline rules' setDayNight keeps; the CR 702.145c/f
// face sync follows at the next state-based re-check
// (rules' dayNightStateCheck re-checks the faces under a designation).
func effDayTime(h Host, c *Ctx, sa *cards.SA) {
	cur := h.Game().DayNight
	to := cur
	v := strings.TrimSpace(sa.ParamStr(cards.PKValue))
	switch cards.DayTimeOf(v) {
	case cards.DayTimeDay:
		to = state.DayNightDay
	case cards.DayTimeNight:
		to = state.DayNightNight
	case cards.DayTimeSwitch:
		switch cur {
		case state.DayNightDay:
			to = state.DayNightNight
		case state.DayNightNight:
			to = state.DayNightDay
		}
	default:
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "unmodelled DayTime Value$ " + v})
		return
	}
	if to == cur {
		return
	}
	h.Emit(events.Event{Kind: events.DayNightChange, Amount: int32(to), Text: to.String()})
}

// transformBlocked reports whether id can't transform right now: a daybound
// or nightbound permanent transforms only by its own ability (CR 702.145b/e),
// and a live S:Mode$ CantTransform static forbids it outright (CR 701.28f,
// Host.TransformBlocked). The one gate SetState's transform emit and its
// would-change probe share.
func transformBlocked(h Host, id state.ObjID) bool {
	return dayNightBound(h, id) || h.TransformBlocked(id)
}
