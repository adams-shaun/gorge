package dirac_test

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// plainsGame deals two 20-Plains decks from the hand-written vanilla table and
// starts a game with seed 1. A generated set plays the same way: its Matchup gives
// the table and decks (dirac/README.md), but it needs make dirac-tables, and an
// Example cannot skip when they are missing.
func plainsGame() *dirac.State {
	deck := make([]kop.CardID, 20)
	for i := range deck {
		deck[i] = vanilla.Plains
	}
	return dirac.NewConfig(vanilla.NewTable(), deck, deck, 1, dirac.Config{})
}

// Example plays a game to its end, answering every decision with its first
// option. The decks are only Plains, so every decision is a pass and the
// game ends when a player's library is empty.
func Example() {
	s := plainsGame()
	for {
		d := s.Step()
		if d.Kind == dirac.DkNone {
			break
		}
		s.Apply(d.Options[0])
	}
	over, winner := s.Terminal()
	fmt.Println("over:", over)
	fmt.Println("winner:", winner)
	// Output:
	// over: true
	// winner: 1
}

// ExampleState_Project shows the canonical Projection of the opening
// position: the objects dealt, both life totals and the turn number.
func ExampleState_Project() {
	s := plainsGame()
	var p dirac.Projection
	s.Project(&p)
	fmt.Println("objects:", p.NObj)
	fmt.Println("life:", p.Life[0], p.Life[1])
	fmt.Println("turn:", p.Turn)
	// Output:
	// objects: 40
	// life: 20 20
	// turn: 1
}
