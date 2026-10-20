package azmcts

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchseat"
)

// BenchmarkSeatDecideSearch is one az seat decision end to end
// (Seat.DecideSearch: the wrapped bot's answer, then a 25-simulation
// clairvoyant Search, heuristic leaf) at a fixed early position, with no
// note sink installed: the seat path every bench and hosted game runs. Its
// allocs/op and B/op must not move when a diagnostic lands on the seat (the
// decision-note emitter's zero-cost-off check, ticket T3); compare it against
// the same file at the base revision.
func BenchmarkSeatDecideSearch(b *testing.B) {
	e, d, _ := stagePosition(b, 3)
	sc := DefaultSeatConfig()
	sc.Search.Sims = 25
	sc.Source = testSeatSource
	s, err := NewSeat(1, nil, sc)
	if err != nil {
		b.Fatal(err)
	}
	env := searchseat.Env{Engine: e, Board: botpolicy.BoardFromGame(e.G, e, d.Player)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.DecideSearch(context.Background(), env, *d); err != nil {
			b.Fatal(err)
		}
	}
}
