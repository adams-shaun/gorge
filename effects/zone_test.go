package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestParseZone characterises the new split contract: ParseZone retains its
// single-zone destination behaviour, while ParseZones gives Origin$ its set
// semantics and rejects unknown tokens instead of treating them as graveyard.
func TestParseZone(t *testing.T) {
	for _, c := range []struct {
		in   string
		want state.Zone
	}{
		{"Hand", state.ZHand},
		{"Battlefield", state.ZBattlefield},
		{"Library", state.ZLibrary},
		{"Exile", state.ZExile},
		{"Stack", state.ZStack},
		{"Command", state.ZCommand},
		{"Ceased", state.ZCeased},
		{"Graveyard", state.ZGraveyard},
	} {
		if got := ParseZone(c.in); got != c.want {
			t.Errorf("ParseZone(%q) = %v, want %v", c.in, got, c.want)
		}
	}

	for _, c := range []struct {
		in        string
		want      []state.Zone
		wantAll   bool
		wantValid bool
	}{
		{"Battlefield", []state.Zone{state.ZBattlefield}, false, true},
		{"Any", nil, true, true},
		{"All", nil, true, true},
		{"Battlefield, Graveyard", []state.Zone{state.ZBattlefield, state.ZGraveyard}, false, true},
		{"Battlefield,Nowhere", []state.Zone{state.ZBattlefield}, false, false},
	} {
		got, all, valid := ParseZones(c.in)
		if !slices.Equal(got, c.want) || all != c.wantAll || valid != c.wantValid {
			t.Errorf("ParseZones(%q) = (%v, %v, %v), want (%v, %v, %v)",
				c.in, got, all, valid, c.want, c.wantAll, c.wantValid)
		}
	}
}

// TestChangeZoneOriginAnyMovesANonGraveyardObject characterises Origin$ set
// semantics for the latent Any spelling and the live All and comma-list
// spellings. Each must admit a battlefield object and emit exactly one move.
func TestChangeZoneOriginAnyMovesANonGraveyardObject(t *testing.T) {
	for _, origin := range []string{"Any", "All", "Battlefield,Graveyard"} {
		t.Run(origin, func(t *testing.T) {
			g, ids := board(t)
			h := &fakeHost{g: g}
			id := ids["myBear"]
			if g.Obj(id).Zone != state.ZBattlefield {
				t.Fatalf("fixture: myBear is %v, want battlefield", g.Obj(id).Zone)
			}
			Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: id}}},
				sa(t, "SP$ ChangeZone | ValidTgts$ Permanent | Origin$ "+origin+" | Destination$ Hand"))

			if g.Obj(id).Zone != state.ZHand {
				t.Fatalf("zone = %v, want Hand: Origin$ %s must admit a battlefield permanent", g.Obj(id).Zone, origin)
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
		})
	}
}

func TestChangeZoneUnknownOriginFailsClosedAndNotes(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	id := ids["myBear"]
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: id}}},
		sa(t, "SP$ ChangeZone | ValidTgts$ Permanent | Origin$ Nowhere | Destination$ Hand"))

	if g.Obj(id).Zone != state.ZBattlefield {
		t.Fatalf("zone = %v, want unchanged Battlefield", g.Obj(id).Zone)
	}
	if len(h.log) != 1 || h.log[0].Kind != events.Note {
		t.Fatalf("events = %+v, want one Note and no MoveZone", h.log)
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
