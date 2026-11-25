package effects

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestTriggerCountScryNum pins TriggerCount$ScryNum (Celeborn the Wise,
// Elrond, Master of Healing, Elvish Mariner, Elminster): the number of cards
// a completed scry LOOKED at (CR 701.22a), read from Ctx.TriggerScryNum and
// independent of ScryBottom (the bottom count in TriggerAmount). The
// completed-scry marker carries both, and the no-host stand-in records the
// window it actually looked at (clamped to the library).
func TestTriggerCountScryNum(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{}
	c.TriggerScryNum = 3
	c.TriggerAmount = 1
	for _, tt := range []struct {
		expr string
		want int32
	}{
		{"TriggerCount$ScryNum", 3},
		{"TriggerCount$ScryNum/Twice", 6},
		{"TriggerCount$ScryBottom", 1},
	} {
		if got := EvalCount(h, c, tt.expr); got != tt.want {
			t.Errorf("%s = %d, want %d", tt.expr, got, tt.want)
		}
	}
	if got := EvalCount(h, &Ctx{}, "TriggerCount$ScryNum"); got != 0 {
		t.Errorf("TriggerCount$ScryNum outside a scry trigger = %d, want 0", got)
	}

	ev := ScryRecord(0, 7, 4, 2)
	if ev.Kind != events.Scry || ev.Amount != 2 || ScryRecordLooked(ev) != 4 {
		t.Fatalf("ScryRecord(looked 4, bottom 2) = %+v (looked %d)", ev, ScryRecordLooked(ev))
	}

	card := mkCard(t, "Name:C\nTypes:Creature\nPT:1/1\nOracle:x\n")
	for _, tt := range []struct {
		lib        int
		scry, want int32
	}{{5, 3, 3}, {2, 3, 2}} {
		h := newHost(t, 2)
		fillLibrary(h.g, 0, card, tt.lib)
		Resolve(h, &Ctx{Source: 1, Controller: 0}, sa(t, "SP$ Scry | Defined$ You | ScryNum$ "+strconv.Itoa(int(tt.scry))))
		var marks []events.Event
		for _, ev := range h.log {
			if ev.Kind == events.Scry {
				marks = append(marks, ev)
			}
		}
		if len(marks) != 1 {
			t.Fatalf("scry %d over %d cards: %d Scry markers, want 1", tt.scry, tt.lib, len(marks))
		}
		if got := ScryRecordLooked(marks[0]); got != tt.want || marks[0].Amount != 0 {
			t.Errorf("scry %d over %d cards: marker looked %d bottom %d, want looked %d bottom 0",
				tt.scry, tt.lib, got, marks[0].Amount, tt.want)
		}
	}
}
