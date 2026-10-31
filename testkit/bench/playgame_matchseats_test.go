package bench_test

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

type attachSpy struct {
	inner    seat.Seat
	attached bool
	ended    bool
	mc       *bots.MatchContext
	result   bots.MatchResult
}

func (s *attachSpy) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	return s.inner.Decide(ctx, v, d)
}

func (s *attachSpy) AttachMatch(mc *bots.MatchContext) error {
	s.attached = true
	s.mc = mc
	return nil
}

func (s *attachSpy) EndMatch(result bots.MatchResult) {
	s.ended = true
	s.result = result
}

func TestPlayGameAttachesMatchSeats(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	da, err := testutil.LoadRepoDeck(reg, "mono-black-aggro")
	if err != nil {
		t.Fatal(err)
	}
	db, err := testutil.LoadRepoDeck(reg, "mono-red-prowess")
	if err != nil {
		t.Fatal(err)
	}
	const seed = 424242
	cfg := rules.Config{
		Seed: seed, Names: []string{"mono-black-aggro", "mono-red-prowess"},
		Decks: [][]*cards.Card{da, db}, Tokens: reg.Tokens,
	}
	seats := []seat.Seat{
		&attachSpy{inner: heuristic.NewBot(seed ^ 1)},
		&attachSpy{inner: heuristic.NewBot(seed ^ 2)},
	}
	_, e, err := bench.PlayGame(cfg, seats, 60, 4000, bench.Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	if e == nil || !e.G.Over {
		t.Fatalf("game did not finish (engine=%v)", e != nil)
	}
	for i, raw := range seats {
		s := raw.(*attachSpy)
		if !s.attached {
			t.Fatalf("seat %d was never attached", i)
		}
		if s.mc == nil || s.mc.Engine != e {
			t.Fatalf("seat %d got no live engine", i)
		}
		if s.mc.OwnDeck == nil {
			t.Fatalf("seat %d got no own deck", i)
		}
		if s.mc.Seats != 2 || s.mc.Seat != i {
			t.Fatalf("seat %d got seat=%d seats=%d", i, s.mc.Seat, s.mc.Seats)
		}
		if s.mc.DeadlineMS != 0 {
			t.Fatalf("seat %d deadline = %d, want bench unbounded", i, s.mc.DeadlineMS)
		}
		if !s.ended {
			t.Fatalf("seat %d was never ended", i)
		}
	}
}
