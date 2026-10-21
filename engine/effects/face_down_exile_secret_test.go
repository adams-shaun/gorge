package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g62/cr-g64: a face-down exile out of a hidden zone is Secret, addressed
// to the recorded looker when the instruction names one and to the card's
// owner for a blind move (whose payload the view then serves to nobody); one
// out of a public zone stays public; a move its emitter already made Secret
// keeps the emitter's Player.

// faceDownExileBoard seeds a 2-seat game with one plain card in each seat's
// zone z and returns the host and the two ids (seat 0's, seat 1's).
func faceDownExileBoard(t *testing.T, z state.Zone) (*fakeHost, state.ObjID, state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	card := mkCard(t, "Name:Plain Card\nTypes:Creature\nPT:1/1\nOracle:x\n")
	var ids [2]state.ObjID
	for p := range ids {
		o := h.g.AddObject(card, state.PlayerID(p))
		o.Zone = z
		h.g.SetZone(z, state.PlayerID(p), []state.ObjID{o.ID})
		ids[p] = o.ID
	}
	return h, ids[0], ids[1]
}

// exileMoves is every MoveZone into exile h logged.
func exileMoves(h *fakeHost) []events.Event {
	var out []events.Event
	for _, ev := range h.log {
		if ev.Kind == events.MoveZone && ev.To == state.ZExile {
			out = append(out, ev)
		}
	}
	return out
}

// TestChangeZoneAllHandFaceDownExileIsEachOwnersSecret: Memory Jar's shape
// ("each player exiles all cards from their hand face down"), resolved by
// seat 0. ChangeZoneAll stamps no Player, so before the fix both moves were
// public and Player 0; each must be Secret to the card's own owner.
func TestChangeZoneAllHandFaceDownExileIsEachOwnersSecret(t *testing.T) {
	h, mine, theirs := faceDownExileBoard(t, state.ZHand)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ ChangeZoneAll | ChangeType$ Card | Origin$ Hand | Destination$ Exile | ExileFaceDown$ True"))
	moves := exileMoves(h)
	if len(moves) != 2 {
		t.Fatalf("want both hand cards exiled, got %+v", moves)
	}
	for _, ev := range moves {
		want := map[state.ObjID]state.PlayerID{mine: 0, theirs: 1}[ev.Obj]
		if !ev.Secret || ev.Player != want || ev.Counter != "exiled_with_face_down" {
			t.Fatalf("face-down hand exile of %d = %+v, want Secret with Player %d (its owner)", ev.Obj, ev, want)
		}
		if o := h.g.Obj(ev.Obj); o == nil || o.Zone != state.ZExile || !o.FaceDown {
			t.Fatalf("precondition: %d is not face down in exile: %+v", ev.Obj, o)
		}
	}
}

// TestGraveyardFaceDownExileStaysPublic is the guard: a face-down exile out
// of a public zone (Ghastly Conscription's graveyard origin) was seen by every
// seat before it moved, so the move stays public and keeps its Player.
func TestGraveyardFaceDownExileStaysPublic(t *testing.T) {
	h, _, _ := faceDownExileBoard(t, state.ZGraveyard)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ ChangeZoneAll | ChangeType$ Card | Origin$ Graveyard | Destination$ Exile | ExileFaceDown$ True"))
	moves := exileMoves(h)
	if len(moves) != 2 {
		t.Fatalf("want both graveyard cards exiled, got %+v", moves)
	}
	for _, ev := range moves {
		if ev.Secret || ev.Player != 0 {
			t.Fatalf("a graveyard-origin face-down exile became %+v, want it public with Player 0 unchanged", ev)
		}
	}
}

// TestFaceDownExileKeepsAnEmittersSecretPlayer is the guard for an emitter
// that has already made its move Secret and named its seat before the
// face-down marker runs: the marker stamps its Counter/Amount but must not
// re-address the move. (effDig's take runs the marker first and then sets
// its own Player via digSecretTo, so it never reaches this branch.)
func TestFaceDownExileKeepsAnEmittersSecretPlayer(t *testing.T) {
	h, _, theirs := faceDownExileBoard(t, state.ZLibrary)
	ev := events.Event{Kind: events.MoveZone, Obj: theirs, From: state.ZLibrary, To: state.ZExile, Player: 1, Secret: true}
	applyMoveFaceDown(h, &Ctx{Controller: 0}, &FaceDownRiders{ExileFaceDown: true, WithMayLook: true}, &ev, state.ZExile)
	if !ev.Secret || ev.Player != 1 || ev.Counter != "exiled_with_face_down_maylook" || ev.Amount != 0 {
		t.Fatalf("an already-Secret face-down exile became %+v, want Player 1 kept (the marker still stamps its Counter/Amount)", ev)
	}
}

// TestHandFaceDownBattlefieldEntryIsEachOwnersSecret: the face-down
// battlefield arm (Tezzeret, Cruel Machinist's "put any number of cards from
// your hand onto the battlefield face down") was already Secret, but the
// ChangeZoneAll and object movers stamp no Player, so seat 1's entry was
// seat 0's secret. Each entry must be Secret to its card's owner.
func TestHandFaceDownBattlefieldEntryIsEachOwnersSecret(t *testing.T) {
	h, mine, theirs := faceDownExileBoard(t, state.ZHand)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ ChangeZoneAll | ChangeType$ Card | Origin$ Hand | Destination$ Battlefield | FaceDown$ True"))
	n := 0
	for _, ev := range h.log {
		if ev.Kind != events.MoveZone || ev.To != state.ZBattlefield {
			continue
		}
		n++
		want := map[state.ObjID]state.PlayerID{mine: 0, theirs: 1}[ev.Obj]
		if !ev.Secret || ev.Player != want {
			t.Fatalf("face-down hand entry of %d = %+v, want Secret with Player %d (its owner)", ev.Obj, ev, want)
		}
	}
	if n != 2 {
		t.Fatalf("want both hand cards entered face down, got %d moves: %+v", n, h.log)
	}
}
