package lab_test

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/lab"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Example is the page docs/site/engines/lab.md shows, on the vanilla table so it
// needs no corpus (the page names a real set instead).
func Example() {
	cfg := lab.Config{Table: v.Table, IDs: [][]kop.CardID{v.Decks[0], v.Decks[1]}}

	// 100 games, hbot against random
	var seeds []uint64
	for i := uint64(1); i <= 100; i++ {
		seeds = append(seeds, i)
	}
	_, sum, _ := lab.Batch(cfg, []lab.Policy{lab.HBot(), lab.Random(1)}, seeds, 4, 20000)
	fmt.Println("games:", sum.Games, "stalled:", sum.Stalls, "hbot beat random:", sum.WinRate(0).Lo > 0.5)

	// fork a position, play it out with two policies, the original does not move
	g, _ := lab.NewGame(lab.Config{Table: v.Table, IDs: cfg.IDs, Seed: 7})
	defer g.Close()
	for i := 0; i < 30 && !g.Done(); i++ {
		_ = g.Apply(g.Decision().Options[0])
	}
	before := g.Hash()
	a := lab.Play(g.Fork(), []lab.Policy{lab.HBot(), lab.HBot()}, 20000)
	b := lab.Play(g.Fork(), []lab.Policy{lab.Random(1), lab.Random(2)}, 20000)
	fmt.Println("original unchanged:", g.Hash() == before, a.Outcome, b.Outcome)

	// a search seat against random
	r := lab.Play(g.Fork(), []lab.Policy{lab.Search(lab.SearchOptions(16)), lab.Random(3)}, 20000)
	fmt.Println("search game ended:", r.Outcome != lab.Truncated)
	// Output:
	// games: 100 stalled: 0 hbot beat random: true
	// original unchanged: true won won
	// search game ended: true
}
