package rules

// cr-g96: DB$ Mill | Optional$ True is a real may-mill election. effMill used
// to ignore Optional$ and always milled; this drives an inline "you may mill
// two cards" spell through the resolution kernel and pins both answers.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestMillOptionalAsks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		accept bool
		want   int
	}{
		{"decline mills nothing", false, 0},
		{"accept mills N", true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := kr2Engine(t, 2)
			spell := kr2Put(t, e, 0, kr2Sorcery(t, "Optional Mill",
				"A:SP$ Mill | NumCards$ 2 | Optional$ True"), state.ZHand, false)
			libBefore := len(e.G.Zone(state.ZLibrary, 0))
			from := len(e.L.Events)
			if libBefore < 2 {
				t.Fatalf("precondition: seat 0 library holds %d cards, want at least 2", libBefore)
			}
			d := kr2Cast(t, e, 0, spell)
			if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "mill_optional" || d.Player != 0 {
				t.Fatalf("pending = %+v, want seat 0's mill_optional KChoose", d)
			}
			kind := "no"
			if tc.accept {
				kind = "yes"
			}
			kr2Answer(t, e, d, kr2Kind(t, d, kind))
			milled := 0
			for _, ev := range e.L.Events[from:] {
				if events.IsMill(ev) {
					milled++
				}
			}
			if milled != tc.want {
				t.Errorf("milled %d cards, want %d", milled, tc.want)
			}
			if got := len(e.G.Zone(state.ZLibrary, 0)); got != libBefore-tc.want {
				t.Errorf("library holds %d cards after the mill, want %d", got, libBefore-tc.want)
			}
		})
	}
}
