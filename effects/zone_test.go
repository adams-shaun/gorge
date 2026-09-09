package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestParseZone confirms the mechanism behind Task fx26 before the
// behavioural leaf below. effects/zone.go's ParseZone maps Forge zone names to
// a single state.Zone, and the names it does NOT handle ("Any", a
// comma-separated list like "Battlefield,Graveyard") fall through the switch
// to `return state.ZGraveyard`.
//
// This test ASSERTS THE CURRENT (DEFECTIVE) BEHAVIOUR on those unhandled
// names so the mechanism is pinned and documented; it is not an oracle for
// what ParseZone should return. The behavioural consequence is exercised by
// TestChangeZoneOriginAnyMovesANonGraveyardObject, and any fix to ParseZone
// (or to the way effChangeZone special-cases Origin$ Any / a list) will need
// this test revisited alongside it.
func TestParseZone(t *testing.T) {
	cases := []struct {
		in   string
		want state.Zone
	}{
		// Names ParseZone handles correctly.
		{"Hand", state.ZHand},
		{"Battlefield", state.ZBattlefield},
		{"Library", state.ZLibrary},
		{"Exile", state.ZExile},
		{"Stack", state.ZStack},
		{"Command", state.ZCommand},
		{"Ceased", state.ZCeased},
		{"Graveyard", state.ZGraveyard},
		// Names ParseZone does NOT handle: both fall through to ZGraveyard.
		{"Any", state.ZGraveyard},
		{"Battlefield,Graveyard", state.ZGraveyard},
	}
	for _, c := range cases {
		if got := ParseZone(c.in); got != c.want {
			t.Errorf("ParseZone(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestChangeZoneOriginAnyMovesANonGraveyardObject is the Task fx26
// deliverable: a ChangeZone effect whose Origin$ says "Any" must move an
// object that is NOT in a graveyard. A permanent on the battlefield is the
// clearest case. Because ParseZone("Any") resolves to ZGraveyard,
// effChangeZone's precondition `o.Zone != ParseZone("Any")` reads as "skip
// unless the object is in the GRAVEYARD", so a battlefield permanent is
// silently skipped -- no MoveZone event, no Note, no log entry.
//
// EXPECTED TO FAIL on main. That is the point.
func TestChangeZoneOriginAnyMovesANonGraveyardObject(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	id := ids["myBear"]
	if g.Obj(id).Zone != state.ZBattlefield {
		t.Fatalf("fixture: myBear is %v, want battlefield", g.Obj(id).Zone)
	}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: id}}},
		sa(t, "SP$ ChangeZone | ValidTgts$ Permanent | Origin$ Any | Destination$ Hand"))

	if g.Obj(id).Zone != state.ZHand {
		t.Fatalf("zone = %v, want Hand: Origin$ Any must move a permanent that is not in the graveyard", g.Obj(id).Zone)
	}
	var moves []events.Event
	for _, e := range h.log {
		if e.Kind == events.MoveZone {
			moves = append(moves, e)
		}
	}
	if len(moves) != 1 {
		t.Fatalf("MoveZone events = %d, want exactly 1: %+v", len(moves), h.log)
	}
}

// TestChangeZoneOriginBattlefieldMovesTheTarget is the control for the
// failing leaf above: it proves THIS fixture's ChangeZone actually moves an
// object when Origin$ is spelled explicitly as "Battlefield" -- so the Any
// failure is caused by Origin$ Any being mis-parsed, not by a ChangeZone
// fixture that never worked. It must pass.
func TestChangeZoneOriginBattlefieldMovesTheTarget(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	id := ids["myBear"]
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: id}}},
		sa(t, "SP$ ChangeZone | ValidTgts$ Permanent | Origin$ Battlefield | Destination$ Hand"))
	if g.Obj(id).Zone != state.ZHand {
		t.Fatalf("zone = %v, want Hand: Origin$ Battlefield on a battlefield permanent must move it", g.Obj(id).Zone)
	}
	var moves int
	for _, e := range h.log {
		if e.Kind == events.MoveZone {
			moves++
		}
	}
	if moves != 1 {
		t.Fatalf("MoveZone events = %d, want 1", moves)
	}
}
