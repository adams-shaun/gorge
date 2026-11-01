package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// BenchmarkTransitionBLB is BenchmarkTransitionFDN over 64 BLB pool
// pairings (the BLB comparison of the CR 613 layer walk).
func BenchmarkTransitionBLB(b *testing.B) {
	decks := benchDecks(b, "blb")
	set := cardsettest.Set(b, "blb")
	type game struct {
		tbl    *kop.Table
		da, db []CardID
	}
	games := make([]game, 64)
	for g := range games {
		tbl, da, db, err := set.Matchup(decks[2*g], decks[2*g+1])
		if err != nil {
			b.Fatal(err)
		}
		games[g] = game{tbl, da, db}
		var s State
		s.Init(tbl, da, db, 0, Config{})
	}
	b.Cleanup(func() {
		for _, g := range games {
			ReleaseTable(g.tbl)
		}
	})
	var s State
	var d Decision
	pol := NewRandomPolicy(1)
	g := 0
	cfg := Config{Mulligans: 2}
	s.Init(games[0].tbl, games[0].da, games[0].db, 1, cfg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.StepInto(&d)
		if d.Kind == DkNone {
			g++
			gm := &games[g%len(games)]
			s.Init(gm.tbl, gm.da, gm.db, uint64(g)+1, cfg)
			s.StepInto(&d)
		}
		s.Apply(pol.Pick(&d))
	}
}
