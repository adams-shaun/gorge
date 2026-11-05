package rules

// CR 406.3 and CR 701.20 -- Vulcan Mind-Meld: the target opponent looks at
// the top card of the caster's library and exiles it face down; the caster
// looks at the top two cards of that player's library and exiles them face
// down; the player who exiled each card may play it. Each face-down card is
// shown to exactly the player who exiled it, on the board and in the stream;
// its owner is shown neither, and no public reveal names the cards.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// setLibraryTop puts the named cards on top of seat p's library, in order.
func setLibraryTop(t *testing.T, e *Engine, p state.PlayerID, names ...string) []state.ObjID {
	t.Helper()
	var top []state.ObjID
	for _, n := range names {
		id := findNamedInZone(e, p, state.ZLibrary, n)
		if id == 0 {
			// the copy was drawn into the opening hand: put it back
			if id = findNamedInZone(e, p, state.ZHand, n); id == 0 {
				t.Fatalf("%q is in neither zone of seat %d", n, p)
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
		}
		top = append(top, id)
	}
	order := append([]state.ObjID(nil), top...)
	for _, id := range e.G.Zone(state.ZLibrary, p) {
		if id != top[0] && (len(top) < 2 || id != top[1]) {
			order = append(order, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: p, IDs: order, Secret: true})
	return top
}

func TestVulcanMindMeldShowsEachCardToItsExiler(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	meld := lookup(t, reg, "Vulcan Mind-Meld")
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{meld, lookup(t, reg, "Hill Giant")},
		[]*cards.Card{lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Ornithopter")})
	mine := setLibraryTop(t, e, 0, "Hill Giant")
	theirs := setLibraryTop(t, e, 1, "Grizzly Bears", "Ornithopter")
	moveByName(t, e, 0, "Vulcan Mind-Meld", state.ZHand)
	addMana(t, e, 0, "U")
	castNamed(t, e, "Vulcan Mind-Meld")
	targetPlayer(t, e, 1)
	mark := len(e.L.Events)
	passUntilStackEmpty(t, e, 30)
	// the opponent (seat 1) exiled and may look at the caster's card
	assertFaceDownExileLookers(t, e, faceDownExileMoveOf(t, e, mark, mine[0]), "Hill Giant",
		map[state.PlayerID]bool{1: true}, map[state.PlayerID]bool{1: true})
	// the caster (seat 0) looked at the opponent's two cards and exiled them:
	// the board and the stream show seat 0 the faces, and no seat is told
	// the cards by a public reveal
	for i, name := range []string{"Grizzly Bears", "Ornithopter"} {
		assertFaceDownExileLookers(t, e, faceDownExileMoveOf(t, e, mark, theirs[i]), name,
			map[state.PlayerID]bool{0: true}, map[state.PlayerID]bool{0: true})
	}
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind != events.Note || ev.Secret {
			continue
		}
		for _, id := range ev.IDs {
			if id == theirs[0] || id == theirs[1] {
				t.Errorf("a public note names the face-down exiled card %d: %+v", id, ev)
			}
		}
	}
	replayCheck(t, e, cfg)
}
