package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggeredPlayersAttackedTargetAmountDeclaration drives a real
// declaration in a 4-seat game: a batch "whenever you attack" trigger whose
// body gains X life, X = TriggeredPlayersAttackedTarget$Amount (Commander Liara
// Portyr's SVar:X), must read the number of DISTINCT players attacked across
// the whole declaration, not the one defender of the event that latched it.
func TestTriggeredPlayersAttackedTargetAmountDeclaration(t *testing.T) {
	t.Parallel()
	const watcher = "Name:Watcher\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" +
		"T:Mode$ AttackersDeclared | AttackingPlayer$ You | Execute$ TrigGain | TriggerZones$ Battlefield | TriggerDescription$ Whenever you attack, gain life for each player attacked.\n" +
		"SVar:TrigGain:DB$ GainLife | LifeAmount$ Y | Defined$ You | SpellDescription$ Gain X life.\n" +
		"SVar:Y:TriggeredPlayersAttackedTarget$Amount\n" +
		"Oracle:x\n"
	const bear = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	for _, tc := range []struct {
		name string
		defs []state.PlayerID // defender of watcher, then of each bear
		want int32
	}{
		{"one defender, three attackers", []state.PlayerID{1, 1, 1}, 1},
		{"two defenders", []state.PlayerID{1, 1, 3}, 2},
		{"three defenders", []state.PlayerID{1, 2, 3}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := New(Config{Seed: 717, Names: []string{"a", "b", "c", "d"},
				Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}})
			e.G.Step = state.StepMain1
			e.G.Active, e.G.Priority, e.G.Turn = 0, 0, 1
			w := onBoardReady(t, e, 0, watcher)
			b1 := onBoardReady(t, e, 0, bear)
			b2 := onBoardReady(t, e, 0, bear)
			e.G.Step = state.StepDeclareAttackers
			e.askAttackers()
			var pairs [][2]int
			for i, id := range []state.ObjID{w, b1, b2} {
				pairs = append(pairs, [2]int{int(id), batchAttackerOptionIndex(t, e, id, tc.defs[i])})
			}
			declareAttackersAt(t, e, pairs)
			life := e.G.Players[0].Life
			passUntilStackEmpty(t, e, 20)
			if got := e.G.Players[0].Life - life; got != tc.want {
				t.Fatalf("gained %d life, want %d (distinct players attacked)", got, tc.want)
			}
		})
	}
}
