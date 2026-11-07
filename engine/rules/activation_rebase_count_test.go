package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A copy effect that ends while another survives is settled by a clear and a
// re-base onto the survivor; when the survivor copies the same source the
// permanent's "activate only once each turn" count carries over, and when it
// copies another source the abilities are new ones.
func TestActivationCountSurvivesSameSourceRebase(t *testing.T) {
	const id, src, other = state.ObjID(7), state.ObjID(31), state.ObjID(32)
	clone := func(s state.ObjID) events.Event {
		return events.Event{Kind: events.ClonePermanent, Obj: id, IDs: []state.ObjID{s}}
	}
	clear := events.Event{Kind: events.ClonePermanent, Obj: id}
	push := events.Event{Kind: events.AbilityPush, Obj: id, Amount: 2}
	cases := []struct {
		name string
		evs  []events.Event
		want int
	}{
		{"activated under the basis", []events.Event{clone(src), push}, 1},
		{"same-source re-base keeps the count", []events.Event{clone(src), push, clear, clone(src)}, 1},
		{"another source starts afresh", []events.Event{clone(src), push, clear, clone(other)}, 0},
		{"a fresh copy of the same source starts afresh", []events.Event{clone(src), push, clone(src)}, 0},
		{"a clear alone returns the printed count", []events.Event{clone(src), push, clear}, 0},
	}
	for _, c := range cases {
		if got := activationUsedSince(c.evs, 0, id, 2, ""); got != c.want {
			t.Errorf("%s: count %d, want %d", c.name, got, c.want)
		}
	}
}
