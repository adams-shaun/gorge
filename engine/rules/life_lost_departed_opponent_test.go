package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLifeOppsLostCountsADepartedOpponent: the life an opponent lost this turn
// is still life the opponents lost after that player leaves the game (Neheb,
// the Eternal's {R} for the damage that killed a defender). Forge sums every
// registered opponent (Player.getOpponentLostLifeThisTurn).
func TestLifeOppsLostCountsADepartedOpponent(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 1})
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.emit(events.Event{Kind: events.Damage, Player: 2, Amount: 20})
	e.emit(events.Event{Kind: events.PlayerLost, Player: 2})
	if alive := e.G.AliveFrom(0); slices.Contains(alive, state.PlayerID(2)) || len(alive) != 2 {
		t.Fatalf("living seats = %v, want seat 2 gone", alive)
	}
	if n := evalHead(t, e, 0, 0, "Count$LifeOppsLostThisTurn"); n != 22 {
		t.Fatalf("Count$LifeOppsLostThisTurn = %d, want 22 (2 from the living opponent, 20 from the departed one)", n)
	}
}
