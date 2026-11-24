package effects

// effect_static_trigger_marker_test.go — a Static$ True body under an Effect's
// Triggers$ is continuous-effect bookkeeping (CR 611.2a), resolved inline and
// never put on the stack (Exhaustion's self-exile at the untap step, Kor
// Dirge's OutOfSight). Its DelayedRegister must carry the |ST flag so
// events.Apply decodes DelayedTrigger.Static, in the Phase arm and in both
// event arms; Static$ False must not mark it.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

func TestEffectTriggersCarryStaticMarker(t *testing.T) {
	cases := []struct {
		name, body string
		want       bool
	}{
		{"phase", "Mode$ Phase | Phase$ End of Turn | Static$ True | Execute$ Ex", true},
		{"changeszone", "Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ Card.IsRemembered | Static$ True | Execute$ Ex", true},
		{"event", "Mode$ AttackerBlocked | ValidCard$ Card.IsRemembered | Static$ True | Execute$ Ex", true},
		{"phase-false", "Mode$ Phase | Phase$ End of Turn | Static$ False | Execute$ Ex", false},
		{"phase-absent", "Mode$ Phase | Phase$ End of Turn | Execute$ Ex", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, base := fixtureHost(t)
			c := NewCtxPtr(base.Source, base.Controller, CtxInit{SVars: map[string]string{
				"T":  tc.body,
				"Ex": "DB$ ChangeZone | Defined$ Self | Origin$ Command | Destination$ Exile",
			}})
			Resolve(h, c, sa(t, "DB$ Effect | Triggers$ T | Duration$ Permanent"))
			var reg []events.Event
			for _, ev := range h.log {
				if ev.Kind == events.DelayedRegister {
					reg = append(reg, ev)
				}
			}
			if len(reg) != 1 || len(h.g.Delayed) != 1 {
				t.Fatalf("precondition: registrations = %+v, delayed = %+v, want exactly one", reg, h.g.Delayed)
			}
			if got := h.g.Delayed[0].Static; got != tc.want {
				t.Fatalf("Delayed.Static = %v for %q (text %q), want %v", got, tc.body, reg[0].Text, tc.want)
			}
		})
	}
}
