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

// place is AddObject plus the two things it does NOT do (Ruling T25-e, fix
// round 1): set Object.Zone and append the id to the matching zone list. A
// well-formed object needs both, or checkZones' own invariant-1 exactly-
// one-zone check fires before whatever the caller actually meant to test
// ever gets a chance to.
func place(g *state.Game, owner state.PlayerID, z state.Zone) *state.Object {
	o := g.AddObject(blankCard(), owner)
	o.Zone = z
	g.SetZone(z, owner, append(g.Zone(z, owner), o.ID))
	return o
}

// TestCheckInvariantsCatchesAZoneAgreementMismatch is invariant 1's
// zone-agreement half, independently of its exactly-one-zone half: the
// object is in exactly one list (so the count check alone would pass it),
// but Object.Zone disagrees with that list's own zone.
func TestCheckInvariantsCatchesAZoneAgreementMismatch(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	o := place(g, 0, state.ZHand)
	o.Zone = state.ZBattlefield // Bug: still only in the hand list.

	failed, msg := runCheck(g, nil, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on Object.Zone disagreeing with its list")
	}
	if !strings.Contains(msg, fmt.Sprint(o.ID)) || !strings.Contains(msg, "hand") {
		t.Errorf("failure message %q does not name the culprit object and its list", msg)
	}
}

// TestCheckInvariantsCatchesAnObjectInTwoZones is invariant 1's
// exactly-one-zone half, independently of the zone-agreement half: the same
// id is appended to two DIFFERENT PLAYERS' hand lists (same zone kind both
// times, so Object.Zone == ZHand agrees with either list on its own) --
// the only shape the count check, and nothing else, catches.
func TestCheckInvariantsCatchesAnObjectInTwoZones(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	o := place(g, 0, state.ZHand)
	g.SetZone(state.ZHand, 1, append(g.Zone(state.ZHand, 1), o.ID)) // Bug: also in seat 1's hand.

	failed, msg := runCheck(g, nil, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on an object present in two zone lists")
	}
	if !strings.Contains(msg, fmt.Sprint(o.ID)) || !strings.Contains(msg, "2 zone lists") {
		t.Errorf("failure message %q does not name the culprit object and the count", msg)
	}
}

// TestCheckInvariantsCatchesObjID0InAZoneList is invariant 1's ObjID-0 half.
func TestCheckInvariantsCatchesObjID0InAZoneList(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	g.SetZone(state.ZHand, 0, []state.ObjID{0})

	failed, msg := runCheck(g, nil, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on ObjID 0 appearing in a zone list")
	}
	if !strings.Contains(msg, "ObjID 0") {
		t.Errorf("failure message %q does not name ObjID 0 as the culprit", msg)
	}
}

// TestCheckInvariantsCatchesHiddenAndBattlefieldOverlap is invariant 6, now
// reachable after I-3/Ruling T25-d hoisted it above the zone-agreement
// check: the same id sits in a hidden zone's list (library) and the
// battlefield's, so invariant 6 must fire on its own, before invariant 1's
// zone-agreement check ever gets a look at it.
func TestCheckInvariantsCatchesHiddenAndBattlefieldOverlap(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	o := place(g, 0, state.ZLibrary)
	g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), o.ID)) // Bug: also on the battlefield.

	failed, msg := runCheck(g, nil, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on an object in both a hidden zone's list and the battlefield's")
	}
	if !strings.Contains(msg, fmt.Sprint(o.ID)) || !strings.Contains(msg, "hidden") {
		t.Errorf("failure message %q does not name the culprit object as the invariant-6 violation", msg)
	}
}

// TestCheckInvariantsCatchesTwoSurvivorsInAFinishedGame is invariant 2.
func TestCheckInvariantsCatchesTwoSurvivorsInAFinishedGame(t *testing.T) {
	g := state.NewGame([]string{"a", "b", "c"})
	g.Over = true
	g.Players[2].Lost = true // Seats 0 and 1 both still in the game.

	failed, msg := runCheck(g, nil, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on a finished game with two survivors")
	}
	if !strings.Contains(msg, "2") {
		t.Errorf("failure message %q does not name the survivor count", msg)
	}
}

// TestCheckInvariantsCatchesNegativeDamage is invariant 4's damage half.
func TestCheckInvariantsCatchesNegativeDamage(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	o := place(g, 0, state.ZBattlefield)
	o.Damage = -1

	failed, msg := runCheck(g, nil, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on an object with negative damage")
	}
	if !strings.Contains(msg, fmt.Sprint(o.ID)) || !strings.Contains(msg, "-1") {
		t.Errorf("failure message %q does not name the culprit object and its damage", msg)
	}
}

// TestCheckInvariantsCatchesANegativeCounter is invariant 4's counter half.
func TestCheckInvariantsCatchesANegativeCounter(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	o := place(g, 0, state.ZBattlefield)
	o.Counters = []state.Counter{{Kind: "+1/+1", N: -2}}

	failed, msg := runCheck(g, nil, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on an object with a negative counter")
	}
	if !strings.Contains(msg, fmt.Sprint(o.ID)) || !strings.Contains(msg, "-2") {
		t.Errorf("failure message %q does not name the culprit object and its counter", msg)
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

// TestCheckInvariantsCatchesADecisionForANonSeat is checkDecisionForLiveOpponent's
// not-a-real-seat guard, independently of the Lost/Life checks below it.
func TestCheckInvariantsCatchesADecisionForANonSeat(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	d := &decision.Decision{Player: 99, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "pass"}}}

	failed, msg := runCheck(g, d, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on a decision pending for a player id that names no real seat")
	}
	if !strings.Contains(msg, "99") || !strings.Contains(msg, "not a real seat") {
		t.Errorf("failure message %q does not name the culprit player id as not a real seat", msg)
	}
}

// TestCheckInvariantsCatchesDuplicateOptionIndices is invariant 5's
// duplicate half.
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

// TestCheckInvariantsCatchesAnOutOfRangeOptionIndex is invariant 5's
// out-of-range half, independently of the duplicate half above.
func TestCheckInvariantsCatchesAnOutOfRangeOptionIndex(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	d := &decision.Decision{Player: 0, Kind: decision.KTarget, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 7, Kind: "player"}}}

	failed, msg := runCheck(g, d, "test")
	if !failed {
		t.Fatal("CheckInvariants did not fail on an out-of-range option index")
	}
	if !strings.Contains(msg, "7") || !strings.Contains(msg, "out of range") {
		t.Errorf("failure message %q does not name the culprit index as out of range", msg)
	}
}

// TestCheckInvariantsAllowsAHealthyGame is the control: a well-formed game
// and a well-formed pending decision must not trip anything above, so the
// fuzz gate's constant CheckInvariants calls are not silently always
// failing (or always passing) regardless of state.
func TestCheckInvariantsAllowsAHealthyGame(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	place(g, 0, state.ZHand)
	place(g, 1, state.ZBattlefield)

	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "pass"}}}

	if failed, msg := runCheck(g, d, "test"); failed {
		t.Fatalf("CheckInvariants failed on a healthy game: %s", msg)
	}
}
