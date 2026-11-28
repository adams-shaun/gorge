package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestEffectAdjustLandPlaysChosenNumber pins the frozen-number spelling of
// the Effect-delivered AdjustLandPlays$ grant (Nahiri's Lithoforming): a
// value naming an SVar whose body is Count$ChosenNumber registers the
// Effect's SetChosenNumber$ as the number of additional land plays, and a
// frozen zero registers a zero grant (no extra land), never the unimplemented
// Note.
func TestEffectAdjustLandPlaysChosenNumber(t *testing.T) {
	for _, tc := range []struct {
		chosen string
		want   int32
	}{{"3", 3}, {"0", 0}} {
		h := newHost(t, 2)
		c := &Ctx{Controller: 0, Source: 1, SVars: map[string]string{
			"MoreLands": "Mode$ Continuous | Affected$ You | AdjustLandPlays$ Z | Description$ Play X more lands this turn.",
			"Z":         "Count$ChosenNumber",
		}}
		Resolve(h, c, sa(t, "DB$ Effect | SetChosenNumber$ "+tc.chosen+" | StaticAbilities$ MoreLands"))
		if len(h.continuous) != 1 {
			t.Fatalf("chosen %s: continuous = %+v, want one AdjustLandPlays registration", tc.chosen, h.continuous)
		}
		ce := h.continuous[0]
		if ce.AdjustLandPlays != tc.want || ce.Affects != "You" || ce.ChosenNumber != tc.want {
			t.Fatalf("chosen %s: registration = %+v, want %d land drops for You", tc.chosen, ce, tc.want)
		}
		for _, ev := range h.log {
			if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented") {
				t.Fatalf("chosen %s: the frozen-number grant reports unimplemented: %q", tc.chosen, ev.Text)
			}
		}
	}
}
