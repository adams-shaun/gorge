package view

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// pendingChars is flatChars with a pending-trigger queue.
type pendingChars struct {
	flatChars
	pts []state.PendingTrigger
}

func (c pendingChars) PendingTriggers() []state.PendingTrigger { return c.pts }

// An ability's name, source link and artwork follow what the viewer can see of
// its source: CR 401.2, 402.3 and 708.5. The monarch's draw is the shape that
// reaches a library: its source is any card at all (CR 724.2a).
func TestAbilityFromASourceTheViewerCantSee(t *testing.T) {
	const name = "Watcher"
	for _, tc := range []struct {
		zone      state.Zone
		faceDown  bool
		viewer    state.PlayerID
		vis       Visibility
		named     bool
		wantLink  bool
		wantInLog bool
	}{
		{zone: state.ZLibrary, viewer: 0, vis: Seat},
		{zone: state.ZLibrary, viewer: 1, vis: Seat},
		{zone: state.ZLibrary, viewer: NoSeat, vis: Omniscient},
		{zone: state.ZHand, viewer: 0, vis: Seat, named: true, wantLink: true},
		{zone: state.ZHand, viewer: 1, vis: Seat},
		{zone: state.ZHand, viewer: NoSeat, vis: Public},
		{zone: state.ZHand, viewer: NoSeat, vis: Omniscient, named: true, wantLink: true},
		{zone: state.ZBattlefield, faceDown: true, viewer: 0, vis: Seat, named: true, wantLink: true},
		{zone: state.ZBattlefield, faceDown: true, viewer: 1, vis: Seat, wantLink: true},
		{zone: state.ZBattlefield, faceDown: true, viewer: NoSeat, vis: Omniscient, named: true, wantLink: true},
		{zone: state.ZBattlefield, viewer: 1, vis: Seat, named: true, wantLink: true, wantInLog: true},
	} {
		g, id := twoSeatWith(t, watcherSrc)
		g.Obj(id).Zone = tc.zone
		g.SetZone(state.ZHand, 0, nil)
		g.SetZone(tc.zone, 0, []state.ObjID{id})
		g.Obj(id).FaceDown = tc.faceDown
		events.Apply(g, events.Event{Kind: events.DelayedPush, Player: 0, Obj: id, Counter: "WatcherBody"})
		ability := g.Stack[0]
		ch := pendingChars{flatChars{g}, []state.PendingTrigger{{Source: id, Controller: 0, Label: name}}}

		v := ProjectFor(g, ch, tc.viewer, tc.vis, nil)
		s := v.Stack[0]
		if got := s.Name == name; got != tc.named {
			t.Errorf("%+v: stack name %q", tc, s.Name)
		}
		if got := s.Source == id; got != tc.wantLink {
			t.Errorf("%+v: stack source %d", tc, s.Source)
		}
		if s.Card != nil && !tc.named {
			t.Errorf("%+v: stack artwork %+v", tc, s.Card)
		}
		if got := len(v.Pending) == 1; got != tc.named {
			t.Errorf("%+v: pending %+v", tc, v.Pending)
		}
		if got := strings.Contains(obj(g, ability), name); got != tc.wantInLog {
			t.Errorf("%+v: log names the ability %q", tc, obj(g, ability))
		}
		if line := Describe(g, events.Event{Kind: events.DelayedPush, Player: 0, Counter: "__monarch_draw"}); strings.Contains(line, name) {
			t.Errorf("%+v: monarch line %q", tc, line)
		}
	}
}

func TestLogNamesAFaceDownObjectOnlyWhenShown(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	o := g.Obj(id)
	o.Zone, o.FaceDown = state.ZBattlefield, true
	g.SetZone(state.ZHand, 0, nil)
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{id})
	tag := "#" + itoa(int64(id))

	if got, want := Describe(g, events.Event{Kind: events.Tap, Obj: id}), "a face-down permanent "+tag+" taps"; got != want {
		t.Errorf("tap line %q, want %q", got, want)
	}
	if got := Describe(g, events.Event{Kind: events.Note, Player: 1, IDs: []state.ObjID{id}}); !strings.Contains(got, "Watcher") {
		t.Errorf("a public reveal should name it: %q", got)
	}
	if got := Describe(g, events.Event{Kind: events.Note, Player: 1, Secret: true, IDs: []state.ObjID{id}}); !strings.Contains(got, "Watcher") {
		t.Errorf("the looker's own line should name it: %q", got)
	}
	o.Zone = state.ZStack
	g.SetZone(state.ZBattlefield, 0, nil)
	if got, want := Describe(g, events.Event{Kind: events.Tap, Obj: id}), "a face-down spell "+tag+" taps"; got != want {
		t.Errorf("stack line %q, want %q", got, want)
	}
}

// A cast and a resolution carry the spell's name in their Text, which the log
// must keep; only the seat's copy can lose it.
func TestRedactionTakesAFaceDownSpellsNameOutOfItsEventText(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	o := g.Obj(id)
	o.Zone, o.FaceDown = state.ZBattlefield, true
	ev := events.Event{Kind: events.Resolve, Obj: id, Text: "Watcher"}
	if got := RedactEvent(g, ev, 1).Text; got != "" {
		t.Errorf("opponent reads %q", got)
	}
	if got := RedactEvent(g, ev, 0).Text; got != "Watcher" {
		t.Errorf("controller reads %q", got)
	}
	if got := RedactEvent(g, ev, NoSeat).Text; got != "" {
		t.Errorf("spectator reads %q", got)
	}
	o.FaceDown = false
	if got := RedactEvent(g, ev, 1).Text; got != "Watcher" {
		t.Errorf("face up, opponent reads %q", got)
	}
}

// A sacrificed token still names the ability it paid for: it has ceased to exist,
// and its Zone field's zero value is the library (CR 111.8).
func TestAbilityFromASacrificedTokenKeepsItsName(t *testing.T) {
	g, id := twoSeatWith(t, watcherSrc)
	src := g.Obj(id)
	src.IsToken, src.Zone = true, state.ZLibrary
	g.SetZone(state.ZHand, 0, nil)
	events.Apply(g, events.Event{Kind: events.DelayedPush, Player: 0, Obj: id, Counter: "WatcherBody"})
	if got := Project(g, flatChars{g}, 1, nil).Stack[0].Name; got != "Watcher" {
		t.Errorf("stack name %q", got)
	}
	if got := obj(g, g.Stack[0]); !strings.Contains(got, "Watcher") {
		t.Errorf("log names the ability %q", got)
	}
}
