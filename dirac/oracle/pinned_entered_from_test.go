package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The ThisTurnEnteredFrom_Battlefield filter word (kop.W7EnteredFromBF) reads
// a graveyard card's latest arrival, so each pin below plays recorded games of
// one card that uses it from a synthetic pool, and requires the card or its
// ability to be on the stack at some decision and a card that died earlier
// the same turn to leave the graveyard afterwards. The lockstep must stay
// clean throughout.
//
// A tail pool is a hash-ordered run of the lowered non-Standard cards, so its
// membership moves whenever another card is lowered. The games are recorded
// on the named tail pool but replayed on a table built from each game's own
// decks, so an unrelated lowering cannot take a card out from under them.

func TestPinnedEnteredFromBroughtBack(t *testing.T) {
	t.Parallel()
	pinnedOwnCards(t, "tail-05", Config{GorgeAutopaySurface: true}, onStack("Brought Back"), diedCardLeavesGraveSameTurn)
}

func TestPinnedEnteredFromGrimReturn(t *testing.T) {
	t.Parallel()
	pinnedOwnCards(t, "tail-15", Config{GorgeAutopaySurface: true}, onStack("Grim Return"), diedCardLeavesGraveSameTurn)
}

func TestPinnedEnteredFromContinue(t *testing.T) {
	t.Parallel()
	pinnedOwnCards(t, "tail-63", Config{GorgeAutopaySurface: true}, onStack("Continue?"), diedCardLeavesGraveSameTurn)
}

func TestPinnedEnteredFromSalvagerOfRuin(t *testing.T) {
	t.Parallel()
	pinnedOwnCards(t, "tail-32", Config{GorgeAutopaySurface: true}, onStack("Salvager of Ruin"), diedCardLeavesGraveSameTurn)
}

func TestPinnedEnteredFromMoira(t *testing.T) {
	t.Parallel()
	pinnedOwnCards(t, "tail-15", Config{GorgeAutopaySurface: true}, onStack("Moira, Urborg Haunt"), diedCardLeavesGraveSameTurn)
}

// pinnedOwnCards plays this call's recorded games, each on its own decks'
// table; PINS_RECORD=1 records them again from pool first.
func pinnedOwnCards(t *testing.T, pool string, cfg Config, reached ...func(ref *GorgeSide) bool) {
	t.Helper()
	if os.Getenv("PINS_RECORD") == "1" {
		pinnedGames(t, pool, cfg, reached...)
		return
	}
	path := pinPath(t)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no recorded games at %s (%v): record them with PINS_RECORD=1", path, err)
	}
	var games []*Repro
	if err := json.Unmarshal(b, &games); err != nil || len(games) == 0 {
		t.Fatalf("%s: %v", path, err)
	}
	reg := synthReg(t)
	for i, r := range games {
		res, hit, err := playPinned(reproSet(t, "custom", r), reg, r, RunConfig{Config: cfg}, reached)
		if err != nil {
			t.Fatalf("%s game %d: %v", path, i, err)
		}
		reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
	}
}

// diedCardLeavesGraveSameTurn: some card went from the battlefield to a
// graveyard and, in the same turn, from a graveyard to another zone.
func diedCardLeavesGraveSameTurn(ref *GorgeSide) bool {
	died := map[state.ObjID]bool{}
	for _, e := range ref.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			clear(died)
		case e.Kind != events.MoveZone:
		case e.From == state.ZBattlefield && e.To == state.ZGraveyard:
			died[e.Obj] = true
		case e.From == state.ZGraveyard && e.To != state.ZGraveyard && died[e.Obj]:
			return true
		}
	}
	return false
}
