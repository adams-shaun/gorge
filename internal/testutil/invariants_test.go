package testutil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// fatalSignal is what fakeTB.Fatalf panics with, so runCheck's recover can
// tell "CheckInvariants called Fatalf" apart from "CheckInvariants itself
// panicked" (a real bug this test must not swallow).
type fatalSignal struct{}

// fakeTB is a recording testing.TB. Embedding the (nil) interface satisfies
// testing.TB's unexported method -- only the standard library may implement
// that directly -- without this package needing to; Helper and Fatalf, the
// only two methods CheckInvariants calls, are overridden to record rather
// than touch the embedded nil. Fatalf panics with fatalSignal to mirror a
// real *testing.T's Fatalf-then-Goexit: CheckInvariants must stop at the
// first violation exactly as it would inside a real test, and every other
// method it might call besides Helper/Fatalf would panic on the embedded
// nil if reached, which is the point -- CheckInvariants is documented to
// use only these two.
type fakeTB struct {
	testing.TB
	msg string
}

func (f *fakeTB) Helper() {}

func (f *fakeTB) Fatalf(format string, args ...any) {
	f.msg = fmt.Sprintf(format, args...)
	panic(fatalSignal{})
}

// runCheck calls CheckInvariants against a fake TB and reports whether it
// failed and, if so, what it said -- without invoking a real
// testing.T.Fatalf, which would abort this test too rather than the
// scenario under test.
func runCheck(g *state.Game, d *decision.Decision, where string) (failed bool, msg string) {
	f := &fakeTB{}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fatalSignal); !ok {
				panic(r) // Not ours: CheckInvariants itself misbehaved.
			}
			failed, msg = true, f.msg
		}
	}()
	CheckInvariants(f, g, d, "test")
	return false, ""
}

// blankCard is a minimal, valid *cards.Card: no faces, so Face() returns nil
// and every zone-walk field CheckInvariants reads (Zone, Damage, Counters)
// comes from the state.Object wrapper, not the card, which is all these
// tests need.
func blankCard() *cards.Card { return &cards.Card{} }

// TestCheckInvariantsCatchesAnObjectInTwoZones feeds it a game where the
// same object id has been (wrongly) appended to two zone lists without ever
// being removed from the first -- the exact shape a MoveZone bug that
// forgets to call remove() would produce.
func TestCheckInvariantsCatchesAnObjectInTwoZones(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	o := g.AddObject(blankCard(), 0)
	g.SetZone(state.ZLibrary, 0, []state.ObjID{o.ID})
	g.SetZone(state.ZHand, 0, []state.ObjID{o.ID}) // Bug: still in ZLibrary too.

	failed, msg := runCheck(g, nil, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on an object present in two zone lists")
	}
	if !strings.Contains(msg, fmt.Sprint(o.ID)) {
		t.Errorf("failure message %q does not name the culprit object %d", msg, o.ID)
	}
}

// TestCheckInvariantsCatchesADecisionForALostPlayer is invariant 3.
func TestCheckInvariantsCatchesADecisionForALostPlayer(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	g.Players[1].Lost = true
	d := &decision.Decision{Player: 1, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "pass"}}}

	failed, msg := runCheck(g, d, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on a decision pending for an eliminated player")
	}
	if !strings.Contains(msg, "1") || !strings.Contains(msg, "lost") {
		t.Errorf("failure message %q does not name the culprit player as eliminated", msg)
	}
}

// TestCheckInvariantsCatchesADecisionForAZeroLifePlayer is invariant 7
// (Ruling P10): Task 22's measured pin, encoded permanently.
func TestCheckInvariantsCatchesADecisionForAZeroLifePlayer(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	g.Players[0].Life = 0
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "pass"}}}

	failed, msg := runCheck(g, d, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on a decision pending for a player at 0 life")
	}
	if !strings.Contains(msg, "0") || !strings.Contains(msg, "life") {
		t.Errorf("failure message %q does not name the culprit player's life total", msg)
	}
}

// TestCheckInvariantsCatchesDuplicateOptionIndices is invariant 5.
func TestCheckInvariantsCatchesDuplicateOptionIndices(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	d := &decision.Decision{Player: 0, Kind: decision.KTarget, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "player"}, {Index: 0, Kind: "player"}}}

	failed, msg := runCheck(g, d, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on a decision with a duplicate option index")
	}
	if !strings.Contains(msg, "0") {
		t.Errorf("failure message %q does not name the culprit index", msg)
	}
}

// TestCheckInvariantsAllowsAHealthyGame is the control: a well-formed game
// and a well-formed pending decision must not trip anything above, so the
// fuzz gate's constant CheckInvariants calls are not silently always
// failing (or always passing) regardless of state.
func TestCheckInvariantsAllowsAHealthyGame(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	// AddObject defaults Object.Zone to ZLibrary and adds the object to no
	// zone list at all -- both need setting explicitly to place one in the
	// hand and the other on the battlefield.
	hand := g.AddObject(blankCard(), 0)
	hand.Zone = state.ZHand
	g.SetZone(state.ZHand, 0, []state.ObjID{hand.ID})
	bf := g.AddObject(blankCard(), 1)
	bf.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 1, []state.ObjID{bf.ID})

	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "pass"}}}

	if failed, msg := runCheck(g, d, "test"); failed {
		t.Fatalf("CheckInvariants failed on a healthy game: %s", msg)
	}
}
