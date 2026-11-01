package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// BenchmarkTransitionFDNPool is BenchmarkTransitionFDN over the same 64 FDN
// pairings, played on ONE table over the whole FDN pool (set.Pool), the
// table self-play plays on by default (set.Pool). Every pool card is
// prepared, including ones neither deck uses, so per-table work that scales
// with the pool (a pool-wide flag one unused card trips, a walk over every
// static source) shows here and not in the two-deck benches.
func BenchmarkTransitionFDNPool(b *testing.B) {
	decks := benchDecks(b, "fdn")
	set := cardsettest.Set(b, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		b.Fatal(err)
	}
	ids := func(es []cardset.Entry) []CardID {
		var out []CardID
		for _, e := range es {
			i, ok := set.Lookup(e.Name)
			if !ok || set.InertID(set.PoolID(i)) {
				b.Fatalf("%s: not a supported pool card", e.Name)
			}
			for k := 0; k < e.Count; k++ {
				out = append(out, set.PoolID(i))
			}
		}
		return out
	}
	type game struct{ da, db []CardID }
	games := make([]game, 64)
	for g := range games {
		games[g] = game{ids(decks[2*g]), ids(decks[2*g+1])}
	}
	Register(tbl)
	b.Cleanup(func() { ReleaseTable(tbl) })
	var s State
	var d Decision
	pol := NewRandomPolicy(1)
	g := 0
	cfg := Config{Mulligans: 2}
	s.Init(tbl, games[0].da, games[0].db, 1, cfg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.StepInto(&d)
		if d.Kind == DkNone {
			g++
			gm := &games[g%len(games)]
			s.Init(tbl, gm.da, gm.db, uint64(g)+1, cfg)
			s.StepInto(&d)
		}
		s.Apply(pol.Pick(&d))
	}
}
