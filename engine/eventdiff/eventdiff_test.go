package eventdiff

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

func TestFirst(t *testing.T) {
	want := []events.Event{{Kind: events.GameStart}, {Kind: events.TurnChange}}
	if d := First(want, want); d != nil {
		t.Errorf("identical streams: %v", d)
	}
	for _, tc := range []struct {
		name    string
		got     []events.Event
		seq     uint64
		missing bool
		short   bool
		message string
	}{
		{name: "differs", got: []events.Event{{Kind: events.GameStart}, {Kind: events.PlayerLost}}, seq: 1, message: "recorded"},
		{name: "runs past the recording", got: []events.Event{{Kind: events.GameStart}, {Kind: events.TurnChange}, {Kind: events.PlayerLost}}, seq: 2, missing: true, message: "recorded log ends"},
		{name: "ends early", got: []events.Event{{Kind: events.GameStart}}, seq: 1, short: true, message: "replay ended"},
	} {
		d := First(want, tc.got)
		if d == nil || d.Seq != tc.seq || d.Missing != tc.missing || d.Short != tc.short {
			t.Errorf("%s: %+v", tc.name, d)
			continue
		}
		if !strings.Contains(d.Error(), tc.message) {
			t.Errorf("%s: %q does not say %q", tc.name, d.Error(), tc.message)
		}
	}
}
