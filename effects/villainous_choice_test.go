package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// TestVillainousChoiceVisitsEveryDefinedOpponent proves the primitive is not
// the old single-target modal shortcut: every Defined$ Opponent is processed
// and the chosen body sees that victim through Remembered.
func TestVillainousChoiceVisitsEveryDefinedOpponent(t *testing.T) {
	g := &state.Game{Players: []state.Player{{ID: 0, Life: 20}, {ID: 1, Life: 20}, {ID: 2, Life: 20}}}
	h := &fakeHost{g: g}
	root := &cards.SA{Kind: "DB", API: "VillainousChoice", Params: map[string]string{
		"Defined": "Opponent", "Choices": "DBLoseLife",
	}}
	c := &Ctx{Source: 0, Controller: 0, SVars: map[string]string{}}
	// ResolveSVar reads the compiled table, so use the direct helper's table
	// representation expected by the parser and link the body explicitly.
	c.SVars = map[string]string{"DBLoseLife": "DB$ LoseLife | Defined$ Player.IsRemembered | LifeAmount$ 2"}
	Resolve(h, c, root)
	if g.Players[1].Life != 18 || g.Players[2].Life != 18 {
		t.Fatalf("victims' life = %d/%d, want 18/18", g.Players[1].Life, g.Players[2].Life)
	}
}
