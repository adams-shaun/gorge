package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func TestNoteCardsForFeedsRepeatPlayersAndClearsLoopRemembered(t *testing.T) {
	h := newHost(t, 3)
	c := &Ctx{Controller: 0, Remembered: []state.Target{
		{Player: 1, IsPlayer: true}, {Player: 2, IsPlayer: true},
	}}
	pump := &cards.SA{API: "Pump", Params: map[string]string{
		"NoteCards": "Self", "NoteCardsFor": "Fame",
	}}
	effPump(h, c, pump)
	if got := c.NotedFor["Fame"]; len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("notation = %v, want [1 2]", got)
	}
	players, ok := repeatPlayers(h, c, "Player.NotedForFame")
	if !ok || len(players) != 2 || players[0] != 1 || players[1] != 2 {
		t.Fatalf("repeat players = %v, %v", players, ok)
	}

	// The loop selector consumes the notation before clearing the temporary
	// chooser remembered set. The body itself is intentionally empty.
	c.SVars = map[string]string{"Body": "DB$ Draw | NumCards$ 0"}
	effRepeatEach(h, c, &cards.SA{API: "RepeatEach", Params: map[string]string{
		"RepeatPlayers": "Player.NotedForFame", "RepeatSubAbility": "Body",
		"ClearRememberedBeforeLoop": "True",
	}})
	if len(c.Remembered) != 0 {
		t.Fatalf("remembered after loop = %v, want empty", c.Remembered)
	}
}
