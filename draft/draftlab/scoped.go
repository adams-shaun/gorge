package draftlab

import "github.com/adams-shaun/gorge/engine/seat"

// GameToken identifies one game for GameScoped seats; only its address means
// anything. It is not zero-sized, so two tokens never share an address.
type GameToken struct{ _ byte }

// GameScoped is an optional seat interface for seats whose answers come from
// outside the process and are batched across concurrent games (draft/draftlab/extern).
// playGame calls BeginGame once the game's seats are built and its engine
// configured, and EndGame exactly once afterwards — right after the engine
// stops, and on every other return path — so a batching barrier can flush
// when every live game is waiting on its agent and never hold a finished
// game's answers.
type GameScoped interface {
	BeginGame(t *GameToken)
	EndGame(t *GameToken)
}

// beginGameScoped notifies the game's two seats that their game begins and
// returns the token plus the one-shot end function: EndGame on the same
// seats in reverse order, exactly once per BeginGame (playGame invokes it
// right after the engine stops and defers it for the error paths). With no
// GameScoped seat the token is nil and end is a no-op, so a game with no
// external seat pays nothing.
func beginGameScoped(seats [2]seat.Seat) (*GameToken, func()) {
	var scoped [2]GameScoped
	for i, s := range seats {
		if g, ok := s.(GameScoped); ok {
			scoped[i] = g
		}
	}
	end := func() {}
	if scoped[0] == nil && scoped[1] == nil {
		return nil, end
	}
	tok := &GameToken{}
	var ended bool
	end = func() {
		if ended {
			return
		}
		ended = true
		for i := 1; i >= 0; i-- {
			if scoped[i] != nil {
				scoped[i].EndGame(tok)
			}
		}
	}
	if scoped[0] != nil {
		scoped[0].BeginGame(tok)
	}
	if scoped[1] != nil {
		scoped[1].BeginGame(tok)
	}
	return tok, end
}
