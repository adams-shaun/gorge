package rules

// cr-g64: CR 406.3 -- cards "exiled face down" can't be examined by any
// player except when instructions allow it; a player instructed to look and
// then exile, or allowed to look, may keep looking. The face of a face-down
// exiled card is therefore visible to EXACTLY the seats that may look:
// a recorded looker (WithMayLook$, a looked-at Dig, a search, Hideaway's
// controller -- CR 702.75a -- the foreteller -- CR 702.143a), a seat a live
// MayLookAt$ static names, and the controller of such a seat (CR 723.4) --
// never its owner by default. The board (view.Project) and the seat's event
// stream (view.RedactEventFor + view.Describe) must agree. Before the fix the
// board admitted the exiled card's controller (its owner) whatever the
// instruction said, ignored MayLookAt$, and the Foretell action's transcript
// read "casts".

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

// exiledFaceIn is the Name seat viewer's projection shows for the exiled
// card id (owned by owner): "" when the face is hidden.
func exiledFaceIn(t *testing.T, e *Engine, viewer, owner state.PlayerID, vis view.Visibility, id state.ObjID) string {
	t.Helper()
	v := view.ProjectFor(e.G, e, viewer, vis, nil)
	for _, pv := range v.Players {
		if pv.ID != owner {
			continue
		}
		for _, cv := range pv.Exile {
			if cv.ID == id {
				if !cv.FaceDown {
					t.Fatalf("precondition: exiled card %d is projected face up to seat %d", id, viewer)
				}
				return cv.Name
			}
		}
	}
	t.Fatalf("exiled card %d is not in seat %d's exile in seat %d's projection", id, owner, viewer)
	return ""
}

// assertFaceDownExileLookers checks the board and the stream for ev's card
// against the exact looker set: a looker's board shows name and its stream
// may name it; every other seat and the public spectator see neither.
func assertFaceDownExileLookers(t *testing.T, e *Engine, ev events.Event, name string, lookers map[state.PlayerID]bool, streamLookers map[state.PlayerID]bool) {
	t.Helper()
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Zone != state.ZExile || !o.FaceDown {
		t.Fatalf("precondition: %d is not face down in exile: %+v", ev.Obj, o)
	}
	for s := range e.G.Players {
		seat := state.PlayerID(s)
		face := exiledFaceIn(t, e, seat, o.Owner, view.Seat, ev.Obj)
		if lookers[seat] != (face == name) {
			t.Errorf("board: seat %d (looker=%v) is shown face %q for the face-down exile of %s", seat, lookers[seat], face, name)
		}
		got := view.RedactEventFor(e.G, ev, seat, view.Seat)
		line := view.Describe(e.G, got)
		if named := got.Obj != 0 || strings.Contains(line, name); named && !streamLookers[seat] {
			t.Errorf("stream: seat %d is served Obj %d, line %q for the face-down exile of %s; it may not look", seat, got.Obj, line, name)
		}
	}
	if face := exiledFaceIn(t, e, view.NoSeat, o.Owner, view.Public, ev.Obj); face != "" {
		t.Errorf("board: the public spectator is shown face %q", face)
	}
	pub := view.RedactEventFor(e.G, ev, view.NoSeat, view.Public)
	if line := view.Describe(e.G, pub); pub.Obj != 0 || strings.Contains(line, name) {
		t.Errorf("stream: the public spectator is served Obj %d, line %q", pub.Obj, line)
	}
}

// TestNecropotenceFaceDownExileIsShownToNobody: Necropotence under seat 1
// exiles the top card of seat 1's library face down without anyone looking
// at it. No seat -- its owner included -- may see that card's face, on the
// board or in the transcript.
func TestNecropotenceFaceDownExileIsShownToNobody(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	necro := lookup(t, reg, "Necropotence")
	bears := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, nil, []*cards.Card{necro, bears})
	src := moveByName(t, e, 1, "Necropotence", state.ZBattlefield)
	bear := moveByName(t, e, 1, "Grizzly Bears", state.ZLibrary)
	order := []state.ObjID{bear}
	for _, id := range e.G.Zone(state.ZLibrary, 1) {
		if id != bear {
			order = append(order, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 1, IDs: order, Secret: true})
	passOnce(t, e)
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != 1 {
		t.Fatalf("seat 1 does not hold priority: %+v", d)
	}
	submitChoices(t, e, abilityOptionFor(t, e, src).Index)
	mark := len(e.L.Events)
	passUntilStackEmpty(t, e, 20)
	ev := faceDownExileMoveOf(t, e, mark, bear)
	assertFaceDownExileLookers(t, e, ev, "Grizzly Bears", nil, nil)
	replayCheck(t, e, cfg)
}

// TestPraetorsGraspCasterMayLookAtTheExiledFace: seat 0 casts Praetor's
// Grasp on seat 1 and exiles seat 1's Grizzly Bears face down. Seat 0 looked
// at it (CR 701.23a: to search is to look; CR 406.3: it may keep looking) and
// the spell's Effect grants "You may look at and play that card" (MayLookAt$
// You). Seat 0 is shown the face on the board and the stream; seat 1, its
// owner, is shown neither.
func TestPraetorsGraspCasterMayLookAtTheExiledFace(t *testing.T) {
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
	caster := map[state.PlayerID]bool{0: true}
	assertFaceDownExileLookers(t, e, ev, "Grizzly Bears", caster, caster)
	replayCheck(t, e, cfg)
}

// TestTezzeretsReckoningMayLookAtGrantShowsTheCaster: Tezzeret's Reckoning
// exiles three random cards of seat 0's library with a bare FaceDown$ (no
// look recorded on the move) and its Effect grants MayLookAt$ You over them.
// The live grant alone shows seat 0 the faces; seat 1 sees none, and the
// moves name the cards to nobody.
func TestTezzeretsReckoningMayLookAtGrantShowsTheCaster(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	reck := lookup(t, reg, "Tezzeret's Reckoning")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{reck}, nil)
	moveByName(t, e, 0, "Tezzeret's Reckoning", state.ZHand)
	addMana(t, e, 0, "UU")
	mark := len(e.L.Events)
	castNamed(t, e, "Tezzeret's Reckoning")
	passUntilStackEmpty(t, e, 20)
	n := 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind != events.MoveZone || ev.To != state.ZExile || ev.From != state.ZLibrary {
			continue
		}
		n++
		assertFaceDownExileLookers(t, e, ev, "Mountain", map[state.PlayerID]bool{0: true}, nil)
	}
	if n != 3 {
		t.Fatalf("Tezzeret's Reckoning exiled %d library cards, want 3", n)
	}
	replayCheck(t, e, cfg)
}

// TestForetellActionIsAForetellNotACast: CR 702.143b -- exiling a card with
// its foretell ability is a special action, not a cast, so no transcript line
// reads "casts"; CR 702.143a -- the foreteller (and only the foreteller) may
// look at the exiled card, on the board and in the stream.
func TestForetellActionIsAForetellNotACast(t *testing.T) {
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
		var move *events.Event
		for i := mark; i < len(e.L.Events); i++ {
			ev := e.L.Events[i]
			if ev.Obj != o.ID {
				continue
			}
			if ev.Kind == events.MoveZone && ev.To == state.ZExile {
				move = &e.L.Events[i]
			}
			if ev.Kind != events.CastInfo {
				continue
			}
			for s := range e.G.Players {
				viewer := state.PlayerID(s)
				line := view.Describe(e.G, view.RedactEventFor(e.G, ev, viewer, view.Seat))
				want := "foretells a card"
				if viewer == p {
					want = "foretells Haunting Voyage"
				}
				if strings.Contains(line, "casts") || !strings.Contains(line, want) {
					t.Errorf("foreteller seat %d: seat %d's line for the Foretell action is %q, want %q and no \"casts\" (CR 702.143b)", p, viewer, line, want)
				}
			}
		}
		if move == nil {
			t.Fatalf("seat %d: the foretell action exiled nothing", p)
		}
		assertFaceDownExileLookers(t, e, *move, "Haunting Voyage", map[state.PlayerID]bool{p: true}, map[state.PlayerID]bool{p: true})
	}
}

// TestHideawayControllerStillSeesTheHiddenFace (guard): CR 702.75a's exiled
// card may be looked at by the hideaway permanent's controller -- seat 1,
// who looked -- and by nobody else, on the board and in the stream.
func TestHideawayControllerStillSeesTheHiddenFace(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	clive := lookup(t, reg, "Clive's Hideaway")
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = clive
	}
	cfg := seatZeroStart(Config{Seed: 818, Names: []string{"other", "clive"}, Decks: [][]*cards.Card{deck, deck}})
	e := New(cfg)
	id := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	e.putTriggersOnStack()
	mark := len(e.L.Events)
	kr6ResolveTop(e)
	pick := e.Pending()
	if pick == nil || pick.Kind != decision.KChoose || pick.Player != 1 {
		t.Fatalf("Hideaway pick = %+v, want seat 1's choice", pick)
	}
	chosen := pick.Options[1].Obj
	submitChoices(t, e, pick.Options[1].Index)
	if bottom := e.Pending(); bottom != nil && bottom.Kind == decision.KArrange {
		submitChoices(t, e, 2, 0, 1)
	}
	ev := faceDownExileMoveOf(t, e, mark, chosen)
	lookers := map[state.PlayerID]bool{1: true}
	assertFaceDownExileLookers(t, e, ev, "Clive's Hideaway", lookers, lookers)
	replayCheck(t, e, cfg)
}
