package httpapi

// Task c7, wire half: the same three visibilities asserted on the JSON a
// client actually receives. view/ pins the Go structs; this file pins that a
// redaction which is correct in view is also correct once marshalled and read
// back off the wire -- a redaction "correct in view but lost in marshalling"
// is still a leak, and is exactly what encoding/json re-introducing a field, a
// field tag being dropped, or a silent decode-time transformation would show.
//
// Two shapes are tested here:
//
//  1. A controlled board projected for the three visibilities and encoded
//     through the identical encoding/json path the HTTP handler uses
//     (writeJSON marshals a view.View; so does this test), then decoded back
//     into a raw map tree and asserted structurally -- key present/absent, null
//     vs array -- on the actual wire, not on the Go struct. The controlled
//     board gives every player a non-empty, per-player-distinct
//     hand/graveyard/exile/library (see the view/ fixture this mirrors), so no
//     assertion can pass on a zero or a shared value.
//
//  2. A real live table driven over the actual network (parkedSeatServer, the
//     same fixture seat_test.go uses), so a truly-served endpoint's JSON is
//     under test: an opponent's graveyard/exile really are present on the
//     wire of a seat view, an opponent's hand really is null, and no seat is
//     ever served a library list.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// visibilityWireBoard is the httpapi package's copy of the controlled board
// from view/visibility_pin_test.go (the two packages cannot share a private
// helper). Every player has a non-empty, per-player-distinct number of cards
// in the library {6,5,7,4}, hand {3,2,4,1}, graveyard {2,4,1,3} and exile
// {1,3,2,4}, plus a distinct pool of 1+seat green.
func visibilityWireBoard(t *testing.T) *state.Game {
	t.Helper()
	g := state.NewGame([]string{"alice", "bob", "carol", "dave"})
	c, _ := cards.ParseBytes("b.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nK:Trample\nOracle:x\n"))
	c.Link()
	for _, zc := range []struct {
		z state.Zone
		n [4]int
	}{
		{state.ZLibrary, [4]int{6, 5, 7, 4}},
		{state.ZHand, [4]int{3, 2, 4, 1}},
		{state.ZGraveyard, [4]int{2, 4, 1, 3}},
		{state.ZExile, [4]int{1, 3, 2, 4}},
	} {
		for p := state.PlayerID(0); p < 4; p++ {
			ids := make([]state.ObjID, 0, zc.n[p])
			for i := 0; i < zc.n[p]; i++ {
				o := g.AddObject(c, p)
				o.Zone = zc.z
				ids = append(ids, o.ID)
			}
			g.SetZone(zc.z, p, ids)
		}
	}
	for p := state.PlayerID(0); p < 4; p++ {
		g.Players[p].Pool[state.MG] = int32(1 + int(p))
	}
	return g
}

// projectWire runs view.ProjectFor for one visibility and returns both the
// Go struct and the raw marshalled JSON decoded into a generic tree, the same
// encoding/json round-trip the handler's writeJSON does. The generic tree is
// what the wire assertions inspect: keys, null-versus-array, ids.
func projectWire(t *testing.T, g *state.Game, viewer state.PlayerID, vis view.Visibility) (view.View, []byte, map[string]any) {
	t.Helper()
	v := view.ProjectFor(g, nil, viewer, vis, nil)
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(b, &tree); err != nil {
		t.Fatal(err)
	}
	return v, b, tree
}

// wirePlayers decodes a view's JSON "players" array entry as maps. A failure
// here is a structural regression in the marshal shape, caught before any
// redaction assertion runs.
func wirePlayers(t *testing.T, tree map[string]any) []map[string]any {
	t.Helper()
	arr, ok := tree["players"].([]any)
	if !ok {
		t.Fatalf("wire view has no players array: %v", tree["players"])
	}
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		m, ok := e.(map[string]any)
		if !ok {
			t.Fatal("a players entry is not an object")
		}
		out = append(out, m)
	}
	return out
}

// wireCardIDs extracts the object ids of a CardView array key (e.g.
// "graveyard", "exile"). A missing or null key fails: the public zone is
// supposed to be a present, populated array.
func wireCardIDs(t *testing.T, pv map[string]any, key string) []state.ObjID {
	t.Helper()
	raw, ok := pv[key]
	if !ok {
		t.Fatalf("wire player is missing %q (it must be present in every mode)", key)
	}
	if raw == nil {
		t.Fatalf("wire player %q is null; public zones never marshal null", key)
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("%q is not an array: %v", key, raw)
	}
	out := make([]state.ObjID, 0, len(arr))
	for _, e := range arr {
		m, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("%q element is not an object", key)
		}
		idf, ok := m["id"].(float64)
		if !ok {
			t.Fatalf("%q element has no numeric id: %v", key, m)
		}
		out = append(out, state.ObjID(int(idf)))
	}
	return out
}

// noLibraryOnWire walks the whole decoded tree for any seat object carrying a
// "library" key. The struct half of the D12 pin lives in view/ (no Library
// field at all); here we additionally prove no library-carrying key reaches
// the wire in any seat object, which is the shape a marshalling-introduced
// leak would show.
func noLibraryOnWire(t *testing.T, players []map[string]any) {
	t.Helper()
	for _, pv := range players {
		if _, ok := pv["library"]; ok {
			t.Fatalf("wire player carries a library list: %v", pv)
		}
	}
}

func sameIDs(a, b []state.ObjID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestWireSeatVisibilityRedactsHandsKeepsOpponentsPublicZones is group 1 on
// the wire, projected for seat 0. Seat 0's own hand is an array with its pool;
// every other seat's hand is JSON null while its graveyard and exile are fully
// populated arrays on the wire, its hand_size/library_size are the real
// counts, and no seat is served a library list.
func TestWireSeatVisibilityRedactsHandsKeepsOpponentsPublicZones(t *testing.T) {
	g := visibilityWireBoard(t)
	const viewer = state.PlayerID(0)
	_, _, tree := projectWire(t, g, viewer, view.Seat)
	players := wirePlayers(t, tree)

	// The viewer's own hand is an array, its pool an object.
	own := players[viewer]
	if own["hand"] == nil {
		t.Fatal("the viewer's own hand is null on the wire")
	}
	if hand := own["hand"].([]any); len(hand) != len(g.Zone(state.ZHand, viewer)) {
		t.Fatalf("the viewer's own wire hand has %d cards, want %d", len(hand), len(g.Zone(state.ZHand, viewer)))
	}
	if own["pool"] == nil {
		t.Fatal("the viewer's own pool is absent on the wire")
	}

	for other := state.PlayerID(1); other < 4; other++ {
		pv := players[other]
		if pv["hand"] != nil {
			t.Fatalf("viewer %d is served seat %d's hand on the wire", viewer, other)
		}
		if pv["pool"] != nil {
			t.Fatalf("viewer %d is served seat %d's pool on the wire", viewer, other)
		}
		if hs, ok := pv["hand_size"].(float64); !ok || int(hs) != len(g.Zone(state.ZHand, other)) {
			t.Fatalf("seat %d hand_size on wire = %v, want %d", other, pv["hand_size"], len(g.Zone(state.ZHand, other)))
		}
		if ls, ok := pv["library_size"].(float64); !ok || int(ls) != len(g.Zone(state.ZLibrary, other)) {
			t.Fatalf("seat %d library_size on wire = %v, want %d", other, pv["library_size"], len(g.Zone(state.ZLibrary, other)))
		}
		if got := wireCardIDs(t, pv, "graveyard"); !sameIDs(got, g.Zone(state.ZGraveyard, other)) {
			t.Fatalf("seat %d graveyard on wire = %v, want ids %v", other, got, g.Zone(state.ZGraveyard, other))
		}
		if got := wireCardIDs(t, pv, "exile"); !sameIDs(got, g.Zone(state.ZExile, other)) {
			t.Fatalf("seat %d exile on wire = %v, want ids %v", other, got, g.Zone(state.ZExile, other))
		}
	}
	noLibraryOnWire(t, players)
}

// TestWirePublicVisibilityShowsEveryPublicZoneNoHands is group 2 on the wire:
// a Public projection's every seat hand and pool is null, its hand_size is
// the real count, its graveyard and exile are fully populated arrays, and no
// seat carries a library list.
func TestWirePublicVisibilityShowsEveryPublicZoneNoHands(t *testing.T) {
	g := visibilityWireBoard(t)
	for viewer := state.PlayerID(0); viewer < 4; viewer++ {
		_, _, tree := projectWire(t, g, viewer, view.Public)
		players := wirePlayers(t, tree)
		for _, pv := range players {
			if pv["hand"] != nil {
				t.Fatalf("public viewer %d is served a hand on the wire: %v", viewer, pv["hand"])
			}
			if pv["pool"] != nil {
				t.Fatalf("public viewer %d is served a pool on the wire: %v", viewer, pv["pool"])
			}
		}
		for seat := state.PlayerID(0); seat < 4; seat++ {
			pv := players[seat]
			if got := wireCardIDs(t, pv, "graveyard"); !sameIDs(got, g.Zone(state.ZGraveyard, seat)) {
				t.Fatalf("public viewer %d seat %d graveyard on wire = %v, want %v", viewer, seat, got, g.Zone(state.ZGraveyard, seat))
			}
			if got := wireCardIDs(t, pv, "exile"); !sameIDs(got, g.Zone(state.ZExile, seat)) {
				t.Fatalf("public viewer %d seat %d exile on wire = %v, want %v", viewer, seat, got, g.Zone(state.ZExile, seat))
			}
		}
		noLibraryOnWire(t, players)
	}
}

// TestWireOmniscientShowsEveryHandAndPoolNoLibrary is group 3 on the wire: an
// omniscient projection serves every seat's hand as an array and every seat's
// pool as an object, keeps the public zones, and still never serves a library
// list. The positive control (hands present on the wire) is what rules out a
// "nothing at all on the wire" false pass.
func TestWireOmniscientShowsEveryHandAndPoolNoLibrary(t *testing.T) {
	g := visibilityWireBoard(t)
	for viewer := state.PlayerID(0); viewer < 4; viewer++ {
		_, _, tree := projectWire(t, g, viewer, view.Omniscient)
		players := wirePlayers(t, tree)
		for seat := state.PlayerID(0); seat < 4; seat++ {
			pv := players[seat]
			if pv["hand"] == nil {
				t.Fatalf("omniscient viewer %d is not served seat %d's hand on the wire", viewer, seat)
			}
			if got := wireCardIDs(t, pv, "hand"); !sameIDs(got, g.Zone(state.ZHand, seat)) {
				t.Fatalf("omniscient viewer %d seat %d hand on wire = %v, want %v", viewer, seat, got, g.Zone(state.ZHand, seat))
			}
			if pv["pool"] == nil {
				t.Fatalf("omniscient viewer %d is not served seat %d's pool on the wire", viewer, seat)
			}
			if got := wireCardIDs(t, pv, "graveyard"); !sameIDs(got, g.Zone(state.ZGraveyard, seat)) {
				t.Fatalf("omniscient viewer %d seat %d graveyard on wire = %v, want %v", viewer, seat, got, g.Zone(state.ZGraveyard, seat))
			}
			if got := wireCardIDs(t, pv, "exile"); !sameIDs(got, g.Zone(state.ZExile, seat)) {
				t.Fatalf("omniscient viewer %d seat %d exile on wire = %v, want %v", viewer, seat, got, g.Zone(state.ZExile, seat))
			}
		}
		noLibraryOnWire(t, players)
	}
}

// TestWireSeatEndpointServesOpponentsPublicZonesAndNoLibrary is shape 2 from
// the file doc: a real table served over the actual network, the same
// parkedSeatServer fixture seat_test.go uses. It reaches the HTTP endpoint a
// client talks to and asserts on the raw bytes (so this is not only the
// marshalling of a hand-built struct): the parked seat's own hand is an array
// on the wire, the other human seat's hand is null while its graveyard and
// exile keys are present, and no seat on the wire carries a library list. The
// omniscient spectator endpoint on the same table serves both human hands and
// still no library list.
//
// Note: this fixture is parked at the first human decision of turn 1, before
// either human player has played a card, so the opponent graveyards it serves
// are genuinely empty -- the CONTENT of opponent public zones (the real D12
// pin) is covered by the controlled-board wire tests above; here the shape
// (keys present as arrays, null-versus-array hand, never a library key) is
// what travels the actual network.
func TestWireSeatEndpointServesOpponentsPublicZonesAndNoLibrary(t *testing.T) {
	srv, r := parkedSeatServer(t, seatClaims)
	parked, other := parkedSeat(t, r)
	ps := "s" + strconv.Itoa(int(parked))
	head := liveHead(t, r)

	var tree map[string]any
	if code, e, raw := seatReq(t, http.MethodGet,
		fmt.Sprintf("%s/api/tables/t1/matches/1/view?seat=%d&seq=%d", srv.URL, parked, head), ps, nil); code != http.StatusOK {
		t.Fatalf("parked seat view over the network: %d %+v", code, e)
	} else if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	players := wirePlayers(t, tree)
	if players[parked]["hand"] == nil {
		t.Fatal("the parked seat's own hand is null on the served wire")
	}
	if players[other]["hand"] != nil {
		t.Fatalf("the parked seat is served the other seat's hand over the network")
	}
	if players[other]["graveyard"] == nil {
		t.Fatal("an opponent's graveyard key is absent on the served wire")
	}
	if players[other]["exile"] == nil {
		t.Fatal("an opponent's exile key is absent on the served wire")
	}
	noLibraryOnWire(t, players)

	// The omniscient spectator path (no ?seat=) on the same table: it serves
	// a library list to nobody either.
	var spec map[string]any
	if code, e, raw := seatReq(t, http.MethodGet, srv.URL+"/api/tables/t1/matches/1/view", "", nil); code != http.StatusOK {
		t.Fatalf("spectator view over the network: %d %+v", code, e)
	} else if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	specPlayers := wirePlayers(t, spec)
	if specPlayers[parked]["hand"] == nil || specPlayers[other]["hand"] == nil {
		t.Fatal("the omniscient spectator endpoint fails to serve one of the human hands over the network")
	}
	noLibraryOnWire(t, specPlayers)
}
