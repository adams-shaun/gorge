package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Thornbow Archer's attack trigger drains "each opponent who doesn't control
// an Elf" through the generic Forge PlayerProperty spelling
// `Player.Opponent+controlsElf_EQ0`. Before the generic controls<ValidCards>
// rule the dotless qualifier matched no branch and failed closed, so the
// Defined$ player set was empty and the drain acted on nobody.
func TestThornbowArcherDrainsOpponentWithoutElf(t *testing.T) {
	t.Parallel()

	t.Run("opponent has no Elf", func(t *testing.T) {
		e := layerEngine(t)
		archer := onBoardReadyCard(t, e, 0, corpusAlternativeCard(t, "Thornbow Archer"))
		// Precondition: the trigger really carries the generic qualifier and
		// the board has no Elf for the opponent.
		if got := e.G.Obj(archer).Face().Triggers[0].Effect.Params["Defined"]; got != "Player.Opponent+controlsElf_EQ0" {
			t.Fatalf("precondition: Defined$ = %q, want Player.Opponent+controlsElf_EQ0", got)
		}
		if got := e.G.Players[1].Life; got != 20 {
			t.Fatalf("precondition: opponent life = %d, want 20", got)
		}
		if e.G.Obj(archer).Zone != state.ZBattlefield {
			t.Fatalf("precondition: Thornbow Archer is not on the battlefield")
		}

		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{archer}})
		drainTriggers(t, e)

		if got := e.G.Players[1].Life; got != 19 {
			t.Fatalf("opponent life = %d after the attack trigger, want 19", got)
		}
		if got := e.G.Players[0].Life; got != 20 {
			t.Fatalf("controller life = %d, want 20 (Thornbow drains only the opponent)", got)
		}
	})

	t.Run("opponent controls an Elf", func(t *testing.T) {
		e := layerEngine(t)
		archer := onBoardReadyCard(t, e, 0, corpusAlternativeCard(t, "Thornbow Archer"))
		onBoard(t, e, 1, "Name:Elf Witness\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n")
		// Precondition: the opponent does control an Elf, so the EQ0 clause
		// must miss and the drain must not happen.
		if got := e.G.Players[1].Life; got != 20 {
			t.Fatalf("precondition: opponent life = %d, want 20", got)
		}

		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{archer}})
		drainTriggers(t, e)

		if got := e.G.Players[1].Life; got != 20 {
			t.Fatalf("opponent life = %d, want 20 (opponent controls an Elf)", got)
		}
	})
}
