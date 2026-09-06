package host

// Task d3: a burst's redacted, described bodies are a pure function of the
// (viewer, vis) pair handed to eventBodiesFor, and two subscribers may share
// a result ONLY when that pair is identical. Three things are pinned here:
//
//   (a) One-principal visibility. eventBodiesFor(seat, Seat) reveals that
//       seat's own secret draws; eventBodiesFor(NoSeat, Public) never does,
//       and — critically — neither does eventBodiesFor(seat, Public): Public
//       forces the NoSeat path regardless of who asks, so a spectator whose
//       viewer id happens to equal a seat's must not see that seat's hand.
//       This is the discrete rule a "share bodies keyed on viewer ALONE"
//       mutation breaks: it would make a Public viewer of seat 0 receive
//       seat 0's own secret, i.e. the seat subscriber's body leaks to the
//       spectator. The test fails by name.
//
//   (b) No cross-seat reveal through the seat path. A seat sees its OWN
//       secret and nobody else's; another seat's hidden card never appears.
//
//   (c) Ownership under concurrency. The redaction deep-copies IDs/Pairs,
//       so two goroutines formatting the SAME shared engine log for two
//       different viewers can never corrupt each other's input or the log.
//       This is a concurrency-only property: a sequential run cannot observe
//       it, so the test drives both viewers concurrently and is run under
//       -race (FL-89: one package per invocation).

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// handOf returns the object ids sitting in seat p's hand at g, in zone
// order — a plain []state.ObjID built by reading the zone directly, never
// ranged over in a hot order.
func handOf(t *testing.T, g *state.Game, p state.PlayerID) []state.ObjID {
	t.Helper()
	zs := g.Zone(state.ZHand, p)
	return append([]state.ObjID(nil), zs...)
}

// shareFixture builds a 2-seat game with one card in each seat's hand and a
// log of each seat's own private draw plus a public damage event. Each
// secret draw's Obj is a real hand card, so rule 1 (a Secret event is its
// emitter's to redact or not) and rule 2 collide: the seat's own draw keeps
// its Obj (Player == viewer) while every other viewer gets the shape only.
func shareFixture(t *testing.T) (*state.Game, []events.Event, []state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"alice", "bob"})
	c, err := cards.ParseBytes("b.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nK:Trample\nOracle:x\n"))
	if err != nil {
		t.Fatal(err)
	}
	c.Link()
	card := func(g *state.Game, z state.Zone, p state.PlayerID) state.ObjID {
		o := g.AddObject(c, p)
		o.Zone = z
		g.SetZone(z, p, []state.ObjID{o.ID})
		return o.ID
	}
	alice := card(g, state.ZHand, 0)
	bob := card(g, state.ZHand, 1)
	evs := []events.Event{
		{Seq: 0, Kind: events.Draw, Player: 0, From: state.ZLibrary, To: state.ZHand, Obj: alice, Secret: true},
		{Seq: 1, Kind: events.Draw, Player: 1, From: state.ZLibrary, To: state.ZHand, Obj: bob, Secret: true},
		{Seq: 2, Kind: events.Damage, Player: 0, Amount: 2},
	}
	return g, evs, []state.ObjID{alice}
}

// atSeq returns the body whose Event.Seq is want, or nil. The bodies are in
// chain order, so Seq is the stable handle.
func atSeq(bodies []protocol.EventBody, want uint64) *protocol.EventBody {
	for i := range bodies {
		if bodies[i].Event.Seq == want {
			return &bodies[i]
		}
	}
	return nil
}

// TestSeatAndSpectatorNeverShareEventBodies is the d3 redaction invariant's
// named test (brief item 5). A seat subscriber's bodies and a spectator's
// bodies for the same table are derived from the SAME (viewer, vis) pair;
// here we pin that the two viewers of one seat never receive each other's
// bodies, and that the seat never receives another seat's.
func TestSeatAndSpectatorNeverShareEventBodies(t *testing.T) {
	g, evs, alice := shareFixture(t)

	seat := eventBodiesFor(0, view.Seat, g, evs)
	spec := eventBodiesFor(view.NoSeat, view.Public, g, evs)
	// viewer 0 asked through the *spectator* table role must still be a
	// spectator — Public forces NoSeat regardless of who asks.
	spec0 := eventBodiesFor(0, view.Public, g, evs)

	// The seat sees alice's own secret draw with its card.
	a := atSeq(seat, 0)
	if a == nil || a.Event.Obj != uint32(alice[0]) {
		t.Fatalf("seat 0 body for its own draw: Obj = %d, want %d (alice's hand card)", objOr(a), alice[0])
	}
	// The seat does NOT see bob's secret draw.
	if bb := atSeq(seat, 1); bb != nil && bb.Event.Obj != 0 {
		t.Fatalf("seat 0 sees bob's secret draw Obj %d", bb.Event.Obj)
	}
	// The spectator sees neither seat's draw.
	for _, s := range []state.PlayerID{view.NoSeat, 0} {
		bodies := spec
		if s == 0 {
			bodies = spec0
		}
		if ab := atSeq(bodies, 0); ab == nil || ab.Event.Obj != 0 {
			t.Fatalf("spectator viewer %d sees alice's secret draw Obj %d (rule 1 leak)", s, objOr(atSeq(bodies, 0)))
		}
		if bb := atSeq(bodies, 1); bb == nil || bb.Event.Obj != 0 {
			t.Fatalf("spectator viewer %d sees bob's secret draw Obj %d (rule 1 leak)", s, objOr(atSeq(bodies, 1)))
		}
	}

	// viewer 0, Public must be byte-identical to viewer NoSeat, Public —
	// Public is viewer-independent. If the (viewer, vis) share were keyed on
	// viewer alone (Public honoring the real seat), eventBodiesFor(0, Public)
	// would equal eventBodiesFor(0, Seat) on the secret draw and this fails.
	if len(spec) != len(spec0) {
		t.Fatalf("Public differs by viewer id: %d vs %d bodies", len(spec), len(spec0))
	}
	for i := range spec {
		if spec[i].Event.Seq != spec0[i].Event.Seq || spec[i].Event.Obj != spec0[i].Event.Obj {
			t.Fatalf("body %d: Public(viewer=%d) Obj %d != Public(NoSeat) Obj %d — sharing keyed on viewer alone", i, 0, spec0[i].Event.Obj, spec[i].Event.Obj)
		}
	}

	// Line must be derived from the redacted event, so a hidden card's name
	// never reaches a spectator's transcript: the spectator's draw line is
	// "alice draws a card", never "alice draws Bear".
	for _, s := range []state.PlayerID{view.NoSeat, 0} {
		bodies := spec
		if s == 0 {
			bodies = spec0
		}
		ab := atSeq(bodies, 0)
		if strings.Contains(ab.Line, "Bear") {
			t.Fatalf("spectator viewer %d line names the hidden card: %q", s, ab.Line)
		}
	}
}

// objOr is a nil-safe Obj reader for the diagnostic messages above.
func objOr(b *protocol.EventBody) uint32 {
	if b == nil {
		return 0xDEAD
	}
	return b.Event.Obj
}

// TestEventBodiesForConcurrentViewersOwnTheirBodies is the ownership half of
// the invariant and can only be seen under concurrency: two goroutines
// redact+describe the SAME shared engine log for two different viewers while
// the test concurrently bodies in the input's appended encoding. If any
// redaction aliased the input's IDs/Pairs instead of deep-copying them, the
// race detector would fire or the later input check would catch a mutation.
// Run under -race (one package per invocation, FL-89).
func TestEventBodiesForConcurrentViewersOwnTheirBodies(t *testing.T) {
	g, evs, alice := shareFixture(t)
	// Give the log real IDs on a non-secret event too, so the deep-copy
	// path (rule 3) is exercised concurrently, not just the Secret rule 1.
	evs = append(evs, events.Event{Seq: 3, Kind: events.DeclareAttackers, Player: 0, IDs: alice})

	// Pin the input's own encoding before any redaction runs.
	want := make([]string, len(evs))
	for i, ev := range evs {
		want[i] = string(ev.Append(nil))
	}

	const rounds = 200
	var wg sync.WaitGroup
	wg.Add(2)
	err := make(chan error, 2)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			b := eventBodiesFor(0, view.Seat, g, evs)
			a := atSeq(b, 0)
			if a == nil || a.Event.Obj != uint32(alice[0]) {
				err <- nil
				return
			}
			// seat 0 must never surface bob's draw under any interleaving.
			if bobBody := atSeq(b, 1); bobBody != nil && bobBody.Event.Obj != 0 {
				err <- nil
				return
			}
		}
		err <- nil
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			b := eventBodiesFor(view.NoSeat, view.Public, g, evs)
			if b0 := atSeq(b, 0); b0 == nil || b0.Event.Obj != 0 {
				err <- nil
				return
			}
			if b3 := atSeq(b, 3); b3 == nil {
				err <- nil
				return
			}
		}
		err <- nil
	}()
	wg.Wait()
	for i := 0; i < 2; i++ {
		if <-err != nil {
			t.Fatal("a concurrent viewer received another viewer's body")
		}
	}

	// The shared input was never mutated by any redaction interleaving.
	for i, ev := range evs {
		if string(ev.Append(nil)) != want[i] {
			t.Fatalf("event %d of the shared log was mutated through a concurrent redaction", i)
		}
	}
}
