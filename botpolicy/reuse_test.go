package botpolicy

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// reuseChars is a Chars stand-in that reports distinct derived facts per
// object, so a test can tell which game a Board's creatures came from — the
// concrete check for whether a refill cleared the previous game's leftovers.
type reuseChars struct {
	pw map[state.ObjID]int32
	kw map[state.ObjID][]string
}

func (c reuseChars) Power(id state.ObjID) int32       { return c.pw[id] }
func (c reuseChars) Toughness(id state.ObjID) int32   { return c.pw[id] }
func (c reuseChars) Keywords(id state.ObjID) []string { return c.kw[id] }

// reuseGame builds a 2-seat game ("alice" seats 0, "bob" seat 1) with one
// creature per listed (kind, controller, pt): kind "a" names it "A" and kind
// "b" names it "B" so the two games in the ownership tests have disjoint,
// identifiable censuses. Returns the game and the Chars that reports the
// same pw for each object.
func reuseGame(t *testing.T, spec []struct {
	kind string
	ctrl state.PlayerID
	pt   int32
}) (*state.Game, reuseChars) {
	t.Helper()
	g := state.NewGame([]string{"alice", "bob"})
	pw := map[state.ObjID]int32{}
	for _, s := range spec {
		o := g.AddObject(cardFace(t, s.kind, "Creature "+s.kind, s.pt, s.pt), s.ctrl)
		g.SetZone(state.ZBattlefield, s.ctrl, []state.ObjID{o.ID})
		pw[o.ID] = s.pt
	}
	return g, reuseChars{pw: pw, kw: map[state.ObjID][]string{}}
}

// TestBoardFromGameIntoOwnership is the named test for Task d2's reuse
// contract: a Board built by BoardFromGameInto is the host's ONE board per
// match, refilled for every decision, so its maps are shared across fills.
// The contract has two halves, both asserted here:
//
//  1. REFILL CLEARS: filling a second, different game into the same Board must
//     leave ONLY the second game's creatures — no leftovers from the first.
//     Removing the clear() in BoardFromGameInto fails this test by name.
//  2. MAPS ARE REUSED: the refill keeps the SAME map headers (clear keeps
//     buckets), so it actually saves the allocation it claims to. Reallocating
//     instead of clearing fails this too.
//
// And the reason a seat must never retain a Board: the Board value returned
// for the first decision shares its maps with the second fill, so a policy
// that held it and read it after the second decision would observe that
// decision's contents — which is why botpolicy.Decide never retains the Board
// (TestDecideDoesNotRetainBoard). This is precisely the hazard Task d2 warns
// about.
func TestBoardFromGameIntoOwnership(t *testing.T) {
	gA, cA := reuseGame(t, []struct {
		kind string
		ctrl state.PlayerID
		pt   int32
	}{
		{"A", 0, 3}, {"A", 1, 4},
	})
	// The first fill, exactly what host/projectNext does for decision A.
	boardA := BoardFromGame(gA, cA, 0)
	if len(boardA.Creatures) != 2 {
		t.Fatalf("board A census = %d creatures, want 2", len(boardA.Creatures))
	}
	// Reuse: capture the A map headers (each is a *Board*'s map reference a
	// retaining seat could have held) before refilling.
	creatA := boardA.Creatures
	lifeA := boardA.Life
	cardsA := boardA.Cards

	gB, cB := reuseGame(t, []struct {
		kind string
		ctrl state.PlayerID
		pt   int32
	}{{"B", 0, 1}})
	boardB := BoardFromGameInto(gB, cB, 0, &boardA)

	// (2) Same buckets: clearing-and-refilling must not reallocate the maps.
	// Maps can't be compared with ==, so compare the header pointer each map
	// resolves to: refill keeps the SAME map, so the pointers are equal.
	if mapPtr(boardB.Creatures) != mapPtr(creatA) || mapPtr(boardB.Life) != mapPtr(lifeA) || mapPtr(boardB.Cards) != mapPtr(cardsA) {
		t.Fatalf("BoardFromGameInto reallocated the maps: reuse is not happening")
	}

	// (1) Refill cleared: only B's single creature remains, none of A's.
	if len(boardB.Creatures) != 1 {
		t.Fatalf("refilled census = %d creatures, want 1 (A's %d creatures failed to clear)", len(boardB.Creatures), len(boardA.Creatures))
	}
	for id, c := range boardB.Creatures {
		if c.Power != 1 {
			t.Errorf("refilled creature %d power = %d, want 1 (a stale A creature leaked into B)", id, c.Power)
		}
	}
	if len(boardA.Creatures) != 1 {
		t.Errorf("the reused Board A still reports %d creatures after refill, want 1", len(boardA.Creatures))
	}
	// Life is repopulated identically on both fills.
	if boardB.Life[0] != boardA.Life[0] || boardB.Life[0] != 20 {
		t.Errorf("life after refill = %v, want 20", boardA.Life)
	}

	// The retained-value hazard, stated as an assertion rather than a
	// footnote: boardA (the value a careless seat might keep) now reads B's
	// contents through its shared maps — which is exactly why a seat must
	// hand each Board to Decide and never read it again.
	if len(boardA.Creatures) != len(boardB.Creatures) {
		t.Errorf("retained boardA sees %d creatures, boardB sees %d — retained boards are not readable after refill (ownership contract)", len(boardA.Creatures), len(boardB.Creatures))
	}
}

// TestDecideDoesNotRetainBoard pins the second half of the ownership
// contract: botpolicy.Decide never retains a Board (or its maps) beyond the
// call, so the host's build-under-lock → Decide → reuse-next loop is safe. It
// refills one Board with a second game and asserts Decide returns the SAME
// intent it would on a freshly built Board of that second game — a policy
// that had retained the first Board would, on the shared maps, drift from
// the fresh answer.
func TestDecideDoesNotRetainBoard(t *testing.T) {
	// Game B: a 2/2 blocker on seat 0 and a 5/5 attacker on seat 1. The 2/2
	// does not kill the 5/5 and seat 0 is at 20, so both BR1 and BR2 decline
	// — Decide returns no block. That is the exact answer a board must give
	// whether it is freshly built or refilled over game A's.
	gB, cB := reuseGame(t, []struct {
		kind string
		ctrl state.PlayerID
		pt   int32
	}{
		{"B", 0, 2}, {"B", 1, 5},
	})
	var aID, bID state.ObjID
	for id, c := range BoardFromGame(gB, cB, 0).Creatures {
		if c.Controller == 1 {
			bID = id
		} else {
			aID = id
		}
	}
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KBlockers, Min: 0, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "blocker", Obj: aID, Player: 0, Attacker: bID}}}

	// Refill a Board first with a different game A (which, if not cleared,
	// or if Decide retained it, would change the answer), then with B.
	gA, cA := reuseGame(t, []struct {
		kind string
		ctrl state.PlayerID
		pt   int32
	}{{"A", 0, 9}})
	board := BoardFromGame(gA, cA, 0)
	board = BoardFromGameInto(gB, cB, 0, &board)

	fresh := BoardFromGame(gB, cB, 0)
	got := Decide(board, &d, rng(1)).Choices
	want := Decide(fresh, &d, rng(2)).Choices
	if !equalChoices(got, want) {
		t.Errorf("Decide on refilled board = %v, but freshly-built board = %v — Decide retained the prior board across the refill", got, want)
	}
}

func equalChoices(a, b []int) bool {
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

// mapPtr returns the underlying hash-table pointer of a map (its header's
// first word, what reflect.Value.Pointer returns for a map), so a test can
// assert two map values are the SAME map — the reuse invariant — since Go
// allows only nil comparisons on maps. A map kept across clear() has the same
// pointer; a reallocated map has a different one.
func mapPtr(m any) uintptr {
	return reflect.ValueOf(m).Pointer()
}

// TestBoardFromGameIntoConcurrentOwnership runs the reuse protocol the host
// match loop actually obeys — fill the one shared Board (build under the
// match lock), hand it to Decide, let the next fill start only after the
// decision is done — under the race detector. This is the concurrent form
// Task d2 demands for a sharing invariant: the Board is one struct whose maps
// are shared across every decision, so a seat that RETAINED it and read it
// while the next decision refilled the shared maps would data-race with the
// producer. Two goroutines (producer = the host loop's next projectNext, and
// the reader = a Decide) alternate via channels: the producer fills the shared
// Board, signals filled; the reader reads the latest fill, signals consumed;
// the next produce waits on consumed. Because the read completes before the
// next write, the race detector must come back clean — and any transition
// that let a retained read overlap a refill is exactly what -race pins as a
// bug, which is why the contract forbids it.
func TestBoardFromGameIntoConcurrentOwnership(t *testing.T) {
	gA, cA := reuseGame(t, []struct {
		kind string
		ctrl state.PlayerID
		pt   int32
	}{{"A", 0, 3}, {"A", 1, 4}})
	gB, cB := reuseGame(t, []struct {
		kind string
		ctrl state.PlayerID
		pt   int32
	}{{"B", 0, 1}})

	brd := NewBoard(2)
	// Two rendezvous channels, created once and reused: the producer forbids
	// the next fill until the reader has consumed the previous one, so no read
	// ever overlaps a write — the contract — and the race detector proves it.
	filled := make(chan struct{})
	consumed := make(chan struct{})
	const cycles = 200

	producerDone := make(chan struct{})
	go func() { // producer: the host loop's build-under-lock, refilling one Board
		defer close(producerDone)
		for i := 0; i < cycles; i++ {
			// Two arbitrary fills per cycle to exercise clear-between-fills.
			BoardFromGameInto(gA, cA, 0, &brd)
			BoardFromGameInto(gB, cB, 0, &brd)
			filled <- struct{}{} // hand the latest fill to the reader
			<-consumed           // wait until the reader is done with it
		}
	}()

	consumerDone := make(chan struct{})
	go func() { // reader: a seat's Decide reading the board it was handed
		defer close(consumerDone)
		for i := 0; i < cycles; i++ {
			<-filled
			// Read the latest fill fully before signalling the next produce.
			if n := len(brd.Creatures); n != 1 {
				t.Errorf("cycle %d: census = %d, want 1", i, n)
			}
			if brd.Life[0] != 20 || brd.Life[1] != 20 {
				t.Errorf("cycle %d: life = %v, want 20/20", i, brd.Life)
			}
			consumed <- struct{}{} // the read is done; the next fill may start
		}
	}()

	<-producerDone
	<-consumerDone
}
