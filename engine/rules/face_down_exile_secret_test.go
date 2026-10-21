package rules

// cr-g62: a ChangeZone face-down exile out of a hidden zone (CR 406.3:
// a card "exiled face down" can't be examined by any player except when
// instructions allow it) must not name the card on any seat's transcript
// but the one seat the projection admits to its face (engine/view/card.go:
// the exiled card's controller, i.e. its owner). Before the fix the move was
// not Secret, so view.RedactEvent's rule (2) kept its Obj (exile is not a
// hidden zone) and view.Describe named the card to every seat and to the
// public spectator.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
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

// assertFaceDownExileNamedOnlyTo checks every seat's and the public
// spectator's served form of ev: only owner is served the card (its Obj and
// a transcript line naming it); every other viewer is served Obj 0 and a
// line that does not name it. The owner arm is the projection's own rule (a
// face-down exiled card's face is its controller's, engine/view/card.go), so
// the transcript and the board agree for every seat.
func assertFaceDownExileNamedOnlyTo(t *testing.T, e *Engine, ev events.Event, name string, owner state.PlayerID) {
	t.Helper()
	if o := e.G.Obj(ev.Obj); o == nil || o.Zone != state.ZExile || !o.FaceDown || o.Owner != owner {
		t.Fatalf("precondition: %d is not a face-down exiled card owned by seat %d: %+v", ev.Obj, owner, o)
	}
	for s := range e.G.Players {
		seat := state.PlayerID(s)
		got := view.RedactEventFor(e.G, ev, seat, view.Seat)
		line := view.Describe(e.G, got)
		if seat == owner {
			if got.Obj != ev.Obj || !strings.Contains(line, name) {
				t.Fatalf("the owner (seat %d) is served Obj %d, line %q; want the card %d (%s): the projection shows it its face", seat, got.Obj, line, ev.Obj, name)
			}
			continue
		}
		if got.Obj != 0 || strings.Contains(line, name) {
			t.Fatalf("seat %d is served Obj %d, line %q for a face-down exile of %s (raw %+v); want no card", seat, got.Obj, line, name, ev)
		}
	}
	pub := view.RedactEventFor(e.G, ev, view.NoSeat, view.Public)
	if line := view.Describe(e.G, pub); pub.Obj != 0 || strings.Contains(line, name) {
		t.Fatalf("the public spectator is served Obj %d, line %q for a face-down exile of %s; want no card", pub.Obj, line, name)
	}
}

// TestPraetorsGraspFaceDownExileNamesNoCardToOtherSeats: seat 0 casts
// Praetor's Grasp ("Search target opponent's library for a card and exile it
// face down") on seat 1 and takes seat 1's Grizzly Bears. The exiled card
// must not be named on seat 0's or the spectator's transcript.
func TestPraetorsGraspFaceDownExileNamesNoCardToOtherSeats(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	grasp := lookup(t, reg, "Praetor's Grasp")
	bears := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{grasp}, []*cards.Card{bears})
	bear := moveByName(t, e, 1, "Grizzly Bears", state.ZLibrary)
	moveByName(t, e, 0, "Praetor's Grasp", state.ZHand)
	addMana(t, e, 0, "BBB")
	castNamed(t, e, "Praetor's Grasp")
	targetPlayer(t, e, 1)
	mark := len(e.L.Events)
	var d *decision.Decision
	for i := 0; i < 20; i++ {
		if d = e.Pending(); d == nil || d.Kind != decision.KPriority {
			break
		}
		passOnce(t, e)
	}
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "search" || d.Player != 0 {
		t.Fatalf("no Praetor's Grasp search ask for seat 0: %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == bear {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("seat 1's Grizzly Bears %d is not offered by the search: %+v", bear, d.Options)
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 20)
	ev := faceDownExileMoveOf(t, e, mark, bear)
	assertFaceDownExileNamedOnlyTo(t, e, ev, "Grizzly Bears", 1)
	replayCheck(t, e, cfg)
}

// TestNecropotenceFaceDownExileIsSeatOnesSecret: Necropotence (repo deck
// hearthhull-worldseed-landfall) under SEAT 1 exiles the top card of seat
// 1's library face down. The move rides effChangeZone's object path, which
// stamps no Player, so the fix must name the card's owner, not seat 0.
func TestNecropotenceFaceDownExileIsSeatOnesSecret(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	necro := lookup(t, reg, "Necropotence")
	bears := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, nil, []*cards.Card{necro, bears})
	src := moveByName(t, e, 1, "Necropotence", state.ZBattlefield)
	bear := moveByName(t, e, 1, "Grizzly Bears", state.ZLibrary)
	// moveByName's library move appends at the bottom; lift it to the top.
	order := []state.ObjID{bear}
	for _, id := range e.G.Zone(state.ZLibrary, 1) {
		if id != bear {
			order = append(order, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 1, IDs: order, Secret: true})
	if top := e.G.Zone(state.ZLibrary, 1)[0]; top != bear {
		t.Fatalf("precondition: seat 1's top card is %d, want Grizzly Bears %d", top, bear)
	}
	passOnce(t, e) // seat 0 passes: seat 1 holds priority on the empty stack
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != 1 {
		t.Fatalf("seat 1 does not hold priority: %+v", d)
	}
	submitChoices(t, e, abilityOptionFor(t, e, src).Index)
	mark := len(e.L.Events)
	passUntilStackEmpty(t, e, 20)
	ev := faceDownExileMoveOf(t, e, mark, bear)
	assertFaceDownExileNamedOnlyTo(t, e, ev, "Grizzly Bears", 1)
	replayCheck(t, e, cfg)
}

// TestForetellActionNamesNoCardToTheOpponent: CR 702.143a's {2} action
// exiles a card from its owner's hand face down. Run for each seat as the
// foreteller: the action's CastInfo and MoveZone must name the card to the
// foreteller only -- never to the other seat or the spectator. Before the
// fix both events were public with Player 0.
func TestForetellActionNamesNoCardToTheOpponent(t *testing.T) {
	t.Parallel()
	voyage := corpusAlternativeCard(t, "Haunting Voyage")
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
			}
			pub := view.RedactEventFor(e.G, ev, view.NoSeat, view.Public)
			if line := view.Describe(e.G, pub); pub.Obj != 0 || strings.Contains(line, "Haunting Voyage") {
				t.Fatalf("foreteller seat %d: the spectator is served %v Obj %d, line %q", p, ev.Kind, pub.Obj, line)
			}
		}
		if seen != 2 {
			t.Fatalf("foreteller seat %d: want the action's CastInfo and MoveZone, saw %d events naming the card", p, seen)
		}
	}
}
