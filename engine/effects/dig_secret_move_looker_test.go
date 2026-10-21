package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDigOpponentLibraryMovesAreTheChoosersSecret (cr-g47): a Dig resolved
// by seat 0 over seat 1's library records BOTH of its piles' Secret moves --
// the taken card to exile and the rest to the graveyard -- with Player 0,
// the seat that looked (the chooser), never seat 1, the library's owner, who
// did not see the window (CR 401.2). Secret's Player is the one seat
// view.RedactEvent hands the payload to.
func TestDigOpponentLibraryMovesAreTheChoosersSecret(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	ids := []state.ObjID{
		h.g.AddObject(land, 1).ID,
		h.g.AddObject(land, 1).ID,
		h.g.AddObject(land, 1).ID,
		h.g.AddObject(land, 1).ID,
	}
	h.g.SetZone(state.ZLibrary, 1, ids)
	Resolve(h, &Ctx{Controller: 0},
		sa(t, "SP$ Dig | Defined$ Opponent | DigNum$ 3 | ChangeNum$ 1 | DestinationZone$ Exile | DestinationZone2$ Graveyard"))
	moves := map[state.Zone]int{}
	for _, e := range h.log {
		if e.Kind != events.MoveZone {
			continue
		}
		if !e.Secret || e.Player != 0 {
			t.Fatalf("dig move %+v: want Secret with Player 0 (the chooser who looked), not the library's owner", e)
		}
		moves[e.To]++
	}
	if moves[state.ZExile] != 1 || moves[state.ZGraveyard] != 2 {
		t.Fatalf("dig moves by destination = %v, want 1 exile + 2 graveyard", moves)
	}
	if lib := h.g.Zone(state.ZLibrary, 1); len(lib) != 1 || lib[0] != ids[3] {
		t.Fatalf("seat 1 library = %v, want [%d]", lib, ids[3])
	}
}

// digSecretMoves returns every MoveZone the Dig logged, fatal on a non-Secret
// one (every Dig library move is Secret).
func digSecretMoves(t *testing.T, log []events.Event) []events.Event {
	t.Helper()
	var out []events.Event
	for _, e := range log {
		if e.Kind != events.MoveZone {
			continue
		}
		if !e.Secret {
			t.Fatalf("dig move %+v is not Secret", e)
		}
		out = append(out, e)
	}
	return out
}

// TestDigMayLookExileIsTheLookersSecret (cr-g47): Ixhel, Scion of Atraxa's
// shape -- a no-choice WithMayLook$ face-down exile of seat 0's top card,
// resolved by seat 1 -- records no look Note, but the may-look grant names
// seat 1 as the one player who may see the face (Amount), so the Secret
// move is seat 1's too, never seat 0's (the owner, whom the projection
// already refuses the face).
func TestDigMayLookExileIsTheLookersSecret(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	ids := []state.ObjID{h.g.AddObject(land, 0).ID, h.g.AddObject(land, 0).ID}
	h.g.SetZone(state.ZLibrary, 0, ids)
	Resolve(h, &Ctx{Controller: 1},
		sa(t, "SP$ Dig | Defined$ Opponent | DigNum$ 1 | ChangeNum$ All | DestinationZone$ Exile | ExileFaceDown$ True | WithMayLook$ True"))
	moves := digSecretMoves(t, h.log)
	if len(moves) != 1 || moves[0].Obj != ids[0] || moves[0].Amount != 1 {
		t.Fatalf("may-look exile moves = %+v, want one Secret move of %d granting the look to seat 1 (Amount 1)", moves, ids[0])
	}
	if moves[0].Player != 1 {
		t.Fatalf("may-look exile move %+v: Player %d, want 1 (the looker the grant names), not the library's owner 0", moves[0], moves[0].Player)
	}
}

// TestDigBlindFaceDownExileStaysTheOwnersSecret (cr-g47 guard): Pyxis of
// Pandemonium's shape -- "each player exiles the top card of their library
// face down", no look, no may-look grant -- shows the resolving player
// nothing, so the move of seat 1's card stays seat 1's Secret exactly as
// before cr-g47 (the activator must not gain the card).
func TestDigBlindFaceDownExileStaysTheOwnersSecret(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	own := h.g.AddObject(land, 0).ID
	opp := h.g.AddObject(land, 1).ID
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{own})
	h.g.SetZone(state.ZLibrary, 1, []state.ObjID{opp})
	Resolve(h, &Ctx{Controller: 0},
		sa(t, "SP$ Dig | Defined$ Player | DigNum$ 1 | ChangeNum$ All | DestinationZone$ Exile | ExileFaceDown$ True"))
	moves := digSecretMoves(t, h.log)
	if len(moves) != 2 {
		t.Fatalf("blind exile moves = %+v, want one per library", moves)
	}
	for _, e := range moves {
		want := h.g.Obj(e.Obj).Owner
		if e.Player != want {
			t.Fatalf("blind face-down exile %+v: Player %d, want the card's owner %d (nobody looked)", e, e.Player, want)
		}
	}
}
