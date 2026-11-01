package host

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/protocol"
)

// bodyCarriesID reports whether one served event body exposes id: the wire
// event's only id-shaped fields (protocol.EventFrom fills Obj/IDs/Pairs and
// nothing else with object ids) or the transcript line naming the card.
func bodyCarriesID(b protocol.EventBody, id state.ObjID, name string) bool {
	if uint32(id) == b.Event.Obj {
		return true
	}
	for _, x := range b.Event.IDs {
		if x == uint32(id) {
			return true
		}
	}
	for _, p := range b.Event.Pairs {
		if p[0] == uint32(id) || p[1] == uint32(id) {
			return true
		}
	}
	return name != "" && strings.Contains(b.Line, name)
}

// TestStartingChoiceEventsSeatHidesOwnDraws is the serving half of
// CR 103.5: while the CR 103.1 play/draw choice is pending, the seat's own
// EventsSeat stream (GET events?seat=, and the SSE catch-up built on it)
// redacts the seat's own Secret opening-hand draws to the opponent shape --
// no own hand card id or card name reaches the body, and the draw count
// survives (7 draw events, library -> hand). The omniscient spectator's
// Events stream is unchanged (the ids are still there). After the choice
// resolves, the same request serves the hand again: the redaction is
// serve-time state, never a change to what the log records, so the chain
// heads and cmd/repro's fixture replay are untouched.
func TestStartingChoiceEventsSeatHidesOwnDraws(t *testing.T) {
	t.Parallel()
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(starterFixtureTable("t1")); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}

	// The fixture opens on the toss winner's (seat 1's) winner-chooses ask.
	d0 := waitPending(t, r, "t1", 1, 1)
	if d0.Kind != decision.KStartingPlayer || d0.Player != 1 {
		t.Fatalf("fixture precondition: first pending = seat %d kind %v, want seat 1's CR 103.1 ask", d0.Player, d0.Kind)
	}
	handIDs, names := seatHandIDs(t, r, "t1", 1)
	if len(handIDs) != 7 {
		t.Fatalf("fixture precondition: %d cards in seat 1's hand, want 7", len(handIDs))
	}
	for _, id := range handIDs {
		if names[id] == "" {
			t.Fatalf("fixture precondition: no name for hand card %d", id)
		}
	}

	// While the choice is pending: seat 1's own event stream carries the 7
	// draws but none of their ids or names.
	blind, err := r.EventsSeat("t1", 1, 0, 1)
	if err != nil {
		t.Fatalf("EventsSeat (pending): %v", err)
	}
	draws := 0
	for _, b := range blind {
		for _, id := range handIDs {
			if bodyCarriesID(b, id, names[id]) {
				t.Errorf("pending window: served body seq %d (%s) leaks hand id %d / %q", b.Event.Seq, b.Event.Kind, id, names[id])
			}
		}
		if b.Event.Kind == "draw" && b.Event.Player == 1 {
			draws++
			if b.Event.From != "library" || b.Event.To != "hand" {
				t.Errorf("pending window: draw body seq %d moved %s->%s, want library->hand (the shape survives)", b.Event.Seq, b.Event.From, b.Event.To)
			}
		}
	}
	if draws != 7 {
		t.Errorf("pending window: served %d seat-1 draw bodies, want 7 (the count survives the redaction)", draws)
	}

	// The omniscient spectator's stream is unchanged: the same genesis draw
	// bodies still carry the hand ids for them.
	spec, err := r.Events("t1", 1, 0)
	if err != nil {
		t.Fatalf("Events (spectator, pending): %v", err)
	}
	specSeen := 0
	for _, b := range spec {
		for _, id := range handIDs {
			if uint32(id) == b.Event.Obj {
				specSeen++
				break
			}
		}
	}
	if specSeen < 7 {
		t.Errorf("omniscient spectator: only %d bodies carry a hand id, want the 7 draws unchanged", specSeen)
	}

	// Answer the choice (seat 1 names themselves, the fixture's own path),
	// and once the engine has parked the next ask, the same EventsSeat
	// request serves the hand again.
	self := -1
	for _, o := range d0.Options {
		if o.Player == 1 {
			self = o.Index
		}
	}
	if self < 0 {
		t.Fatalf("fixture precondition: the toss ask offers no option for seat 1")
	}
	if err := r.SubmitIntent("t1", 1, 1, decision.Intent{Seq: d0.Seq, Player: 1, Choices: []int{self}}); err != nil {
		t.Fatalf("SubmitIntent(toss choice): %v", err)
	}
	waitNextPending(t, r, "t1", 1, 1, d0.Seq)
	open, err := r.EventsSeat("t1", 1, 0, 1)
	if err != nil {
		t.Fatalf("EventsSeat (after the choice): %v", err)
	}
	openSeen := 0
	for _, b := range open {
		for _, id := range handIDs {
			if bodyCarriesID(b, id, names[id]) {
				openSeen++
				break
			}
		}
	}
	if openSeen < 7 {
		t.Errorf("after the choice: only %d served bodies carry a hand id, want the 7 opening draws visible again", openSeen)
	}
}

// seatHandIDs reads a seat's true opening hand and its cards' names off the
// parked engine, the ground truth the leak assertions walk the served wire
// against.
func seatHandIDs(t *testing.T, r *Registry, id TableID, seat state.PlayerID) ([]state.ObjID, map[state.ObjID]string) {
	t.Helper()
	_, m, err := r.lookup(id, 1)
	if err != nil {
		t.Fatalf("lookup %s: %v", id, err)
	}
	var ids []state.ObjID
	names := map[state.ObjID]string{}
	if err := m.locked(func() error {
		for _, oid := range m.e.G.Zone(state.ZHand, seat) {
			ids = append(ids, oid)
			names[oid] = m.e.Name(oid)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return ids, names
}
