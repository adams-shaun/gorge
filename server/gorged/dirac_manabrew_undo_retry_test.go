package gorged

import "testing"

func TestPlayLandUndoGamesReplaysPreLandLosses(t *testing.T) {
	var calls []int
	playLandUndoGames(t, 3, 12, func(game int) bool {
		calls = append(calls, game)
		return game != 0 && game != 2 // games 0 and 2 end before a land
	})
	if len(calls) != 5 {
		t.Fatalf("play ran %v; want 5 games (2 pre-land losses replayed, 3 played)", calls)
	}
}
