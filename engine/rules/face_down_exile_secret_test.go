package rules

// cr-g62/cr-g64: a ChangeZone face-down exile out of a hidden zone (CR
// 406.3: a card "exiled face down" can't be examined by any player except
// when instructions allow it) must not name the card on any seat's
// transcript but a seat that may look at it -- the recorded looker, or
// nobody for a blind exile (engine/view/card.go: projector.mayLookFaceDown).
// Before cr-g62 the move was not Secret, so view.RedactEvent's rule (2) kept
// its Obj (exile is not a hidden zone) and view.Describe named the card to
// every seat and to the public spectator.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// faceDownExileMoveOf is the one MoveZone after mark that put id into exile.
func faceDownExileMoveOf(t *testing.T, e *Engine, mark int, id state.ObjID) events.Event {
	t.Helper()
	var found []events.Event
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.To == state.ZExile {
			found = append(found, ev)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one MoveZone of %d into exile after event %d, got %+v", id, mark, found)
	}
	return found[0]
}

// TestForetellActionNamesNoCardToTheOpponent: CR 702.143a's {2} action
// exiles a card from its owner's hand face down. Run for each seat as the
// foreteller: the action's CastInfo and MoveZone must name the card to the
// foreteller only -- never to the other seat or the spectator. Before the
// fix both events were public with Player 0.
func TestForetellActionNamesNoCardToTheOpponent(t *testing.T) {
	t.Parallel()
	voyage := corpusAlternativeCard(t, "Haunting Voyage")
	names := []string{"a", "b"}
	for _, p := range []state.PlayerID{0, 1} {
		e := New(Config{Seed: 1, Names: []string{"a", "b"},
			Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
		for q := state.PlayerID(0); q < 2; q++ {
			e.G.SetZone(state.ZHand, q, nil)
		}
		o := e.G.AddObject(voyage, p)
		o.Zone = state.ZHand
		e.G.SetZone(state.ZHand, p, []state.ObjID{o.ID})
		e.G.Step, e.G.Turn = state.StepMain1, 1
		e.G.Active, e.G.Priority = p, p
		e.G.Players[p].Pool[state.MC] = 2
		mark := len(e.L.Events)
		e.beginCast(p, optionByMode(t, e.legalActions(p), "foretell"))
		if got := e.G.Obj(o.ID); got.Zone != state.ZExile || !got.FaceDown {
			t.Fatalf("seat %d: precondition: the foretell action did not exile Haunting Voyage face down: %+v", p, got)
		}
		seen := 0
		for _, ev := range e.L.Events[mark:] {
			if ev.Obj != o.ID || (ev.Kind != events.CastInfo && ev.Kind != events.MoveZone) {
				continue
			}
			seen++
			for s := range e.G.Players {
				viewer := state.PlayerID(s)
				got := view.RedactEventFor(e.G, ev, viewer, view.Seat)
				line := view.Describe(e.G, got)
				named := got.Obj == o.ID || strings.Contains(line, "Haunting Voyage")
				if named != (viewer == p) {
					t.Fatalf("foreteller seat %d: seat %d is served %v Obj %d, line %q (raw %+v); want the card named to the foreteller only", p, viewer, ev.Kind, got.Obj, line, ev)
				}
				// The redacted CastInfo keeps the foreteller in Player, so
				// its line must attribute the public action to the
				// foreteller, not to seat 0 (review MAJOR, cr-g62).
				if ev.Kind == events.CastInfo && viewer != p {
					if want := names[p] + " foretells"; !strings.HasPrefix(line, want) {
						t.Fatalf("foreteller seat %d: seat %d is served %q, want it to name the foreteller %q", p, viewer, line, want)
					}
				}
			}
			pub := view.RedactEventFor(e.G, ev, view.NoSeat, view.Public)
			if line := view.Describe(e.G, pub); pub.Obj != 0 || strings.Contains(line, "Haunting Voyage") {
				t.Fatalf("foreteller seat %d: the spectator is served %v Obj %d, line %q", p, ev.Kind, pub.Obj, line)
			} else if ev.Kind == events.CastInfo {
				if want := names[p] + " foretells"; !strings.HasPrefix(line, want) {
					t.Fatalf("foreteller seat %d: the spectator is served %q, want it to name the foreteller %q", p, line, want)
				}
			}
		}
		if seen != 2 {
			t.Fatalf("foreteller seat %d: want the action's CastInfo and MoveZone, saw %d events naming the card", p, seen)
		}
	}
}
