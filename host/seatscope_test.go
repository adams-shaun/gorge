package host

// Per-seat projection and event redaction — Task M2b-4 (FL-73).
//
// ViewAtSeat/EventsSeat are the library's answer to "what does player k
// see": a view built for one seat and an event stream redacted for it, both
// via primitives that already exist (view.ProjectFor, view.RedactEventsFor
// with Seat visibility) — this task threads a viewer through, it writes no
// new redaction. The assertions pin the three properties the brief names:
// (a) the asked seat's own hand — and, while the engine is parked on that
// seat, its decision — is visible, and no other seat's hand is; (b) the
// projection follows the viewer, so a different seat's view is that seat's
// game, not the table's; (c) the event stream surfaces the seat's own
// secret draws and a card that entered another seat's hand never surfaces.
//
// (c)'s redaction is state-at-head (view.RedactEvents rule 2: a hidden-to-
// hidden move whose object is not visible to the viewer keeps its shape,
// not its Obj), so its fixture must have cards still sitting in hands at
// head — the shared finished match does not (every seat plays its hand out,
// hand sizes are 0), which is exactly why (c) runs against a live match
// parked on its first decision, where the openers are still in hand.

import (
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// inHandAt is the set of object ids in seat p's hand at the state the
// caller passes in — "currently sitting in a hidden zone", the same reading
// view.RedactEvents rule 2 redacts against. A lookup set only, never
// iterated, so no map order can reach an assertion.
func inHandAt(g *state.Game, p state.PlayerID) map[state.ObjID]bool {
	set := map[state.ObjID]bool{}
	for _, id := range g.Zone(state.ZHand, p) {
		set[id] = true
	}
	return set
}

// lastDrawInHand scans the log backwards for seat p's latest Draw whose
// card is still in p's hand at head: a (c) fixture event, one a seat
// viewer must keep and every other seat viewer must redact.
func lastDrawInHand(l *events.Log, p state.PlayerID, hand map[state.ObjID]bool) (events.Event, bool) {
	for i := len(l.Events) - 1; i >= 0; i-- {
		ev := l.Events[i]
		if ev.Kind == events.Draw && hand[ev.Obj] {
			return ev, true
		}
	}
	return events.Event{}, false
}

// TestViewAtSeatProjectsForTheViewer pins (a) and (b) on the shared
// finished match: every seat's own hand is visible to itself and to nobody
// else; the view follows the viewer — seat 1's view is seat 1's game, seat
// 0's hand hidden, not the table's game; the projected hand contents are
// exactly the engine's own ZHand for that seat, in order; and a finished
// match asks nobody, so no seat's view carries a decision.
func TestViewAtSeatProjectsForTheViewer(t *testing.T) {
	t.Parallel()
	r, m := finishedTable(t)
	head := uint64(len(m.e.L.Events) - 1)
	seats := len(m.cfg.Names)
	g := m.e.G

	views := make([]view.View, seats)
	for p := 0; p < seats; p++ {
		id := state.PlayerID(p)
		v, err := r.ViewAtSeat("t1", 1, head, id)
		if err != nil {
			t.Fatalf("ViewAtSeat(seat %d): %v", p, err)
		}
		views[p] = v
		if v.Viewer != id {
			t.Fatalf("seat %d: View.Viewer is %d", p, v.Viewer)
		}
		if v.Visibility != view.Seat.String() {
			t.Fatalf("seat %d: visibility %q, want %q", p, v.Visibility, view.Seat.String())
		}
		for i := range v.Players {
			pv := v.Players[i]
			if pv.ID == id && pv.Hand == nil {
				t.Fatalf("seat %d's own hand is hidden", p)
			}
			if pv.ID != id && pv.Hand != nil {
				t.Fatalf("viewer %d sees seat %d's hand: %v", p, pv.ID, pv.Hand)
			}
			if pv.ID == id {
				want := g.Zone(state.ZHand, id)
				if len(pv.Hand) != len(want) {
					t.Fatalf("seat %d: view shows %d hand cards, the zone has %d", p, len(pv.Hand), len(want))
				}
				for j, cv := range pv.Hand {
					if cv.ID != want[j] {
						t.Fatalf("seat %d: hand card %d is %d, the zone has %d", p, j, cv.ID, want[j])
					}
				}
			}
		}
		if v.Decision != nil {
			t.Fatalf("viewer %d sees a decision on a finished match: %+v", p, v.Decision)
		}
	}

	// (b) the projection follows the viewer: the same board, two views.
	if views[0].Players[1].Hand != nil {
		t.Fatal("viewer 0 sees seat 1's hand")
	}
	if views[1].Players[1].Hand == nil {
		t.Fatal("viewer 1 cannot see its own hand")
	}
	if views[1].Players[0].Hand != nil {
		t.Fatal("viewer 1 sees seat 0's hand")
	}
	if len(views[1].Players[1].Hand) != views[1].Players[1].HandSize {
		t.Fatal("viewer 1's hand length disagrees with its HandSize")
	}
}

// TestViewAtSeatAndEventsSeatDuringALiveMatch pins the two properties that
// need a game still in progress. On a LIVE match parked on its first
// decision, with the openers still in hand:
//
//   - the decision half of (a): the one seat the engine is waiting on sees
//     its own pending decision (seq — the very event that asked it) and its
//     hand; every other seat sees neither the decision nor anyone's hand.
//     A finished match's views (above) all carry a nil Decision, the "no
//     seat is being asked" side of the same property.
//
//   - (c): view.RedactEvents rule 2 redacts against the state at head, so
//     "a card that entered another seat's hand never surfaces" can only be
//     observed while that card is still there. The viewer's own draws keep
//     their card (Obj and name); another seat's draws keep only their
//     shape ("draws a card"), Obj 0 and no name; the mirror direction —
//     that seat's own stream surfaces the same draw — is checked too.
func TestViewAtSeatAndEventsSeatDuringALiveMatch(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	o := testOptions(t)
	o.Seats = func(names []string, seed uint64) []seat.Seat {
		out := make([]seat.Seat, len(names))
		for i := range out {
			out[i] = blockingSeat{entered: entered}
		}
		return out
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the match never parked on a decision")
	}
	_, m, err := r.lookup("t1", 1)
	if err != nil {
		t.Fatal(err)
	}
	m.mu.RLock()
	head := uint64(len(m.e.L.Events) - 1)
	d := m.e.Pending()
	var asked state.PlayerID
	if d == nil {
		m.mu.RUnlock()
		t.Fatal("a live, parked match has no pending decision")
	}
	asked = d.Player
	seats := uint8(len(m.cfg.Names))
	hands := make([]map[state.ObjID]bool, seats)
	for s := uint8(0); s < seats; s++ {
		hands[s] = inHandAt(m.e.G, state.PlayerID(s))
	}
	m.mu.RUnlock()

	// (a)'s decision half.
	av, err := r.ViewAtSeat("t1", 1, head, asked)
	if err != nil {
		t.Fatal(err)
	}
	if av.Decision == nil {
		t.Fatal("the seat being asked does not see its own decision")
	}
	if av.Decision.Player != asked {
		t.Fatalf("the asked seat %d sees someone else's decision (%d)", asked, av.Decision.Player)
	}
	if av.Decision.Seq != head {
		t.Fatalf("the attached decision is seq %d, but the head view is seq %d", av.Decision.Seq, head)
	}
	if av.Players[asked].Hand == nil {
		t.Fatal("the asked seat's own hand is hidden from it")
	}
	for i := range av.Players {
		if pv := av.Players[i]; pv.ID != asked && pv.Hand != nil {
			t.Fatalf("the asked seat %d sees seat %d's hand", asked, pv.ID)
		}
	}
	other := state.PlayerID((uint8(asked) + 1) % seats)
	ov, err := r.ViewAtSeat("t1", 1, head, other)
	if err != nil {
		t.Fatal(err)
	}
	if ov.Decision != nil {
		t.Fatal("a seat that is not being asked sees a decision")
	}
	if ov.Players[other].Hand == nil {
		t.Fatal("the other seat's own hand is hidden from it")
	}
	if ov.Players[asked].Hand != nil {
		t.Fatal("the other seat sees the asked seat's hand")
	}

	// (c): the viewer's draws surface, another seat's do not. The parked
	// match's openers are all drawn and still in hand, so a qualifying
	// draw exists for both seats.
	own, ok := lastDrawInHand(m.e.L, asked, hands[asked])
	if !ok {
		t.Fatalf("the asked seat %d has no logged draw still in hand at head", asked)
	}
	otherDraw, ok := lastDrawInHand(m.e.L, other, hands[other])
	if !ok {
		t.Fatalf("the other seat %d has no logged draw still in hand at head", other)
	}

	bodies, err := r.EventsSeat("t1", 1, 0, asked)
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != int(head)+1 || bodies[0].Event.Seq != 0 || bodies[len(bodies)-1].Event.Seq != head {
		t.Fatalf("EventsSeat(0): %d bodies from seq %d to %d, want the whole log", len(bodies), bodies[0].Event.Seq, bodies[len(bodies)-1].Event.Seq)
	}
	bySeq := make(map[uint64]protocol.EventBody, len(bodies))
	for _, b := range bodies {
		if _, dup := bySeq[b.Event.Seq]; dup {
			t.Fatalf("events stream repeats seq %d", b.Event.Seq)
		}
		bySeq[b.Event.Seq] = b
	}
	// The viewer's own draw keeps its card, and the transcript names it.
	b, ok := bySeq[own.Seq]
	if !ok {
		t.Fatalf("viewer %d's own draw at seq %d is missing", asked, own.Seq)
	}
	if b.Event.Obj != uint32(own.Obj) {
		t.Fatalf("viewer %d's own draw at seq %d lost its card: obj %d, want %d", asked, own.Seq, b.Event.Obj, own.Obj)
	}
	if name := m.e.G.Obj(own.Obj).Face().Name; !strings.Contains(b.Line, name) {
		t.Fatalf("viewer %d's own draw does not name the card %q: %q", asked, name, b.Line)
	}
	// Another seat's draw keeps its shape, never its card.
	c, ok := bySeq[otherDraw.Seq]
	if !ok {
		t.Fatalf("seat %d's draw at seq %d is missing from viewer %d's stream", other, otherDraw.Seq, asked)
	}
	if c.Event.Obj != 0 {
		t.Fatalf("viewer %d sees the card seat %d drew (seq %d, obj %d)", asked, other, otherDraw.Seq, c.Event.Obj)
	}
	if name := m.e.G.Obj(otherDraw.Obj).Face().Name; strings.Contains(c.Line, name) {
		t.Fatalf("viewer %d's transcript names seat %d's hidden card %q: %q", asked, other, name, c.Line)
	}
	// The mirror: that other seat's own stream surfaces the same draw.
	obodies, err := r.EventsSeat("t1", 1, 0, other)
	if err != nil {
		t.Fatal(err)
	}
	var cb protocol.EventBody
	for _, bd := range obodies {
		if bd.Event.Seq == otherDraw.Seq {
			cb = bd
			break
		}
	}
	if cb.Event.Seq != otherDraw.Seq {
		t.Fatalf("seat %d's own draw at seq %d is missing from its own stream", other, otherDraw.Seq)
	}
	if cb.Event.Obj != uint32(otherDraw.Obj) {
		t.Fatalf("seat %d's own draw at seq %d lost its card: obj %d, want %d", other, otherDraw.Seq, cb.Event.Obj, otherDraw.Obj)
	}

	r.Close()
	r.Wait("t1")
}
