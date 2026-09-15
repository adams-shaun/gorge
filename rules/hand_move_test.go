package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The handmove1 end-to-end leaves: a Burgeoning-shaped "choose N cards
// matching ChangeType$ from Origin$ Hand" ChangeZone reached through the real
// engine (opponent plays a land, the optional trigger accepted) poses the
// hand-choice KChoose, the answer moves exactly the picked card, and the
// whole game replays from the log. The effects-package halves (the ask shape,
// the re-entry contract, the R-9 no-host fallback, the fx42 scoping) live in
// effects/hand_move_test.go.

// burgeoningSrc is Burgeoning's own script shape: an optional trigger on an
// opponent's land play whose Execute$ is the broken ChangeZone shape --
// Origin$ Hand, a ChangeType$ filter, no Defined$/DefinedPlayer$/ValidTgts$.
const burgeoningSrc = "Name:Burgeoning\nTypes:Enchantment\n" +
	"T:Mode$ LandPlayed | ValidCard$ Land.OppCtrl | TriggerZones$ Battlefield | OptionalDecider$ You | Execute$ TrigDropLand\n" +
	"SVar:TrigDropLand:DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land\nOracle:x\n"

// burgeoningFixture seats Burgeoning on seat 0's battlefield and arranges
// seat 0's hand to hold EXACTLY handLands Mountains: the mountain-deck filler
// leaves the opening hand full of them, so handLands == 0 first bridges every
// hand Mountain back to the library, then exactly handLands are bridged out
// of the library. All zone changes go through the event path.
func burgeoningFixture(t *testing.T, seed uint64, handLands int) (*Engine, Config) {
	t.Helper()
	e, cfg, id := newFixtureDeck(t, seed, burgeoningSrc)
	// Empty the hand of Mountains first, whatever the seeded shuffle drew.
	for _, oid := range append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...) {
		if o := e.G.Obj(oid); o != nil && o.Face() != nil && o.Face().Name == "Mountain" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: oid, From: state.ZHand, To: state.ZLibrary, Player: 0})
		}
	}
	// Bridge exactly handLands Mountains from the library into the hand.
	moved := 0
	for _, oid := range e.G.Zone(state.ZLibrary, 0) {
		if moved >= handLands {
			break
		}
		if o := e.G.Obj(oid); o != nil && o.Face() != nil && o.Face().Name == "Mountain" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: oid, From: state.ZLibrary, To: state.ZHand, Player: 0})
			moved++
		}
	}
	if moved != handLands {
		t.Fatalf("fixture library lacks Mountains: bridged %d, want %d", moved, handLands)
	}
	// Seat the enchantment (came back from the fixture's own hand/library
	// bridge; find it wherever it is now) on the battlefield.
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield})
	}
	// The setup ran after Advance built the upkeep priority snapshot; the
	// snapshot's options are stale. Clear and re-drive (the same net state
	// the live Submit loop reaches).
	e.pending = nil
	e.Advance()
	driveToStep(t, e, e.G.Turn, e.G.Active, state.StepMain1)
	return e, cfg
}

// opponentPlaysALand drives to seat 1's own turn (a land can be played only
// during its controller's own main phase, CR 305.1) and has seat 1 play one
// Mountain, returning the pending decision after the play -- for a Burgeoning
// game that is seat 0's trigger_optional ask.
func opponentPlaysALand(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	driveToStep(t, e, 2, 1, state.StepMain1)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 1 {
		t.Fatalf("expected seat 1's priority on their own main1, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "play_land" {
			idx = o.Index
			break
		}
	}
	if idx < 0 {
		t.Fatalf("seat 1 has no play_land option: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	// The land play puts the trigger on the stack; drain the priority rounds
	// until the trigger reaches its resolution ask.
	return passUntilNonPriority(t, e, 20)
}

// handToBattlefieldMoves counts seat 0 hand->battlefield MoveZone events for
// one object id anywhere in the log.
func handToBattlefieldMoves(log []events.Event, id state.ObjID) int {
	n := 0
	for _, ev := range log {
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.From == state.ZHand && ev.To == state.ZBattlefield {
			n++
		}
	}
	return n
}

// TestBurgeoningAsksForTheHandLandAndHonoursTheAnswer is the core leaf: with
// two lands in hand (a strict superset of ChangeNum 1), answering "yes" to
// the optional trigger poses a real KChoose over the hand lands, and the
// answer -- here the SECOND option, proving the choice is honoured, not a
// first-eligible default -- moves exactly that card onto the battlefield.
func TestBurgeoningAsksForTheHandLandAndHonoursTheAnswer(t *testing.T) {
	e, cfg := burgeoningFixture(t, 71, 2)
	handBefore := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)

	d := opponentPlaysALand(t, e)
	if d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("expected the trigger_optional ask, got %+v", d)
	}
	submitChoices(t, e, 0) // yes

	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "hand_move" {
		t.Fatalf("expected a pending hand_move KChoose, got %+v", d)
	}
	if d.Player != 0 || d.Min != 1 || d.Max != 1 {
		t.Fatalf("Min/Max/Player = %d/%d/%d, want 1/1/0 (take one, the hand's owner)", d.Min, d.Max, d.Player)
	}
	if d.ResumeKind != "hand_move" {
		t.Fatalf("ResumeKind = %q, want \"hand_move\"", d.ResumeKind)
	}
	if len(d.Options) != 2 {
		t.Fatalf("options = %d, want 2 (the two Mountains; nothing else is pickable)", len(d.Options))
	}
	for _, o := range d.Options {
		if o.Label != "Mountain" {
			t.Fatalf("option label = %q, want the face name", o.Label)
		}
	}
	picked := d.Options[1].Obj
	submitChoices(t, e, 1)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(picked); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the picked land is not on the battlefield: %+v", o)
	}
	if n := handToBattlefieldMoves(e.L.Events, picked); n != 1 {
		t.Fatalf("picked land moved hand->battlefield %d times, want exactly 1", n)
	}
	handAfter := e.G.Zone(state.ZHand, 0)
	if len(handAfter) != len(handBefore)-1 {
		t.Fatalf("hand size %d, want %d", len(handAfter), len(handBefore)-1)
	}
	for _, oid := range handAfter {
		if oid == picked {
			t.Fatalf("the picked land is still in hand: %v", handAfter)
		}
	}
	replayCheck(t, e, cfg)
}

// TestBurgeoningNoLandInHandResolvesSilently is the zero-eligible leaf:
// answering "yes" with no land in hand resolves the trigger doing nothing --
// no ask, no move, no wedge. Seat 1's own land play is a legitimate
// hand->battlefield move; the guard is that none of SEAT 0's hand objects
// moved.
func TestBurgeoningNoLandInHandResolvesSilently(t *testing.T) {
	e, cfg := burgeoningFixture(t, 72, 0)
	handIDs := make(map[state.ObjID]bool)
	for _, oid := range e.G.Zone(state.ZHand, 0) {
		handIDs[oid] = true
	}

	d := opponentPlaysALand(t, e)
	if d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("expected the trigger_optional ask, got %+v", d)
	}
	submitChoices(t, e, 0) // yes

	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected the resolution to complete back at priority, got %+v", d)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.From == state.ZHand && ev.To == state.ZBattlefield && handIDs[ev.Obj] {
			t.Fatalf("a seat 0 hand object moved hand->battlefield with an empty-land hand: %+v", ev)
		}
	}
	replayCheck(t, e, cfg)
}

// TestBurgeoningSingleLandMovesWithoutAsk is the no-choice leaf: with exactly
// ChangeNum eligible cards the take is deterministic (there is no decision
// anybody could answer differently), so the land moves without an ask.
func TestBurgeoningSingleLandMovesWithoutAsk(t *testing.T) {
	e, cfg := burgeoningFixture(t, 73, 1)
	handBefore := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)

	d := opponentPlaysALand(t, e)
	if d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("expected the trigger_optional ask, got %+v", d)
	}
	submitChoices(t, e, 0) // yes

	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected the resolution to complete back at priority, got %+v", d)
	}
	land := handBefore[0]
	if o := e.G.Obj(land); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the single hand land did not move: %+v", o)
	}
	if n := handToBattlefieldMoves(e.L.Events, land); n != 1 {
		t.Fatalf("land moved hand->battlefield %d times, want exactly 1", n)
	}
	replayCheck(t, e, cfg)
}
