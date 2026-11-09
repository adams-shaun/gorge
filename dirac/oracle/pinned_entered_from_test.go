package oracle

import (
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

func TestPinnedEnteredFromBroughtBack(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "tail-05", Config{GorgeAutopaySurface: true}, onStack("Brought Back"), diedCardLeavesGraveSameTurn)
}

func TestPinnedEnteredFromGrimReturn(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "tail-15", Config{GorgeAutopaySurface: true}, onStack("Grim Return"), diedCardLeavesGraveSameTurn)
}

func TestPinnedEnteredFromContinue(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "tail-63", Config{GorgeAutopaySurface: true}, onStack("Continue?"), diedCardLeavesGraveSameTurn)
}

func TestPinnedEnteredFromSalvagerOfRuin(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "tail-32", Config{GorgeAutopaySurface: true}, onStack("Salvager of Ruin"), diedCardLeavesGraveSameTurn)
}

func TestPinnedEnteredFromMoira(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "tail-15", Config{GorgeAutopaySurface: true}, onStack("Moira, Urborg Haunt"), diedCardLeavesGraveSameTurn)
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
