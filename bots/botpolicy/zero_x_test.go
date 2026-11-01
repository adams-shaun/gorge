package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A creature with no toughness of its own dies at X=0 (CR 704.5f), so outside its hand
// the bot casts it only when it can pay for more.
func TestBotHoldsAZeroToughnessCreatureItCantPayXFor(t *testing.T) {
	const spell state.ObjID = 7
	for _, tc := range []struct {
		name string
		card Card
		pool int32
		cast bool
	}{
		{"X X with nothing to pay", Card{Creature: true, ManaCost: "X X"}, 0, false},
		{"X X with one mana", Card{Creature: true, ManaCost: "X X"}, 1, false},
		{"X X with two", Card{Creature: true, ManaCost: "X X"}, 2, true},
		{"X G with one", Card{Creature: true, ManaCost: "X G"}, 1, false},
		{"a printed toughness", Card{Creature: true, Toughness: 1, ManaCost: "X"}, 0, true},
		{"not a creature", Card{ManaCost: "X R"}, 0, true},
		{"from hand", Card{Creature: true, ManaCost: "X X"}, 0, true},
	} {
		mode := "mayplay"
		if tc.name == "from hand" {
			mode = ""
		}
		var b Board
		b.IsMain = true
		b.Pool[5] = tc.pool
		b.Cards.Set(spell, tc.card)
		d := &decision.Decision{Seq: 1, Kind: decision.KPriority, Min: 1, Max: 1, Options: []decision.Option{
			{Index: 0, Kind: "cast", Obj: spell, Mode: mode},
			{Index: 1, Kind: "pass"},
		}}
		if got := b.chooseCast(d) == 0; got != tc.cast {
			t.Errorf("%s: cast %v, want %v", tc.name, got, tc.cast)
		}
	}
}
