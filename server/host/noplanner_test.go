package host

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// noPlanner promotes only game.Game's methods, so it hides the PaymentPlanner
// of the game it wraps.
type noPlanner struct{ game.Game }

// planCountingSeat wants payment plans and counts the decisions and the plans
// it is handed. Embedding the interface hides the bot's board path, so every
// decision reaches it through a projected view.
type planCountingSeat struct {
	seat.Seat
	decisions, withPlans, views int
}

func (s *planCountingSeat) WantsPaymentActions() bool { return true }

func (s *planCountingSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	s.decisions++
	if len(d.PaymentActions) > 0 {
		s.withPlans++
	}
	if len(v.Players) > 0 {
		s.views++
	}
	return s.Seat.Decide(ctx, v, d)
}

func TestNoPaymentPlannerSkipsEnsure(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	play := func(wrap func(game.Game) game.Game) *planCountingSeat {
		t.Helper()
		s := &planCountingSeat{Seat: heuristic.NewBot(1)}
		o := o
		o.Seats = func(_ []string, seed uint64) []seat.Seat {
			return []seat.Seat{s, heuristic.NewBot(seed + 1)}
		}
		r, err := New(o)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		cfg := TableConfig{ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Seed: 7, Spectator: view.Omniscient}
		if err := r.AddTable(cfg); err != nil {
			t.Fatal(err)
		}
		r.mu.RLock()
		tab := r.tables["t1"]
		r.mu.RUnlock()
		m, err := r.newMatch(tab, 1)
		if err != nil {
			t.Fatal(err)
		}
		m.g = wrap(m.g)
		if final := r.play(context.Background(), tab, m); final != protocol.MatchFinished {
			t.Fatalf("match ended %s: %s", final, m.reason)
		}
		return s
	}

	planned := play(func(g game.Game) game.Game { return g })
	if planned.withPlans == 0 {
		t.Fatalf("the planner offered no plan in %d decisions, so this game proves nothing", planned.decisions)
	}
	bare := play(func(g game.Game) game.Game { return noPlanner{g} })
	if bare.decisions == 0 || bare.views != bare.decisions {
		t.Fatalf("%d of %d decisions were projected", bare.views, bare.decisions)
	}
	if bare.withPlans != 0 {
		t.Fatalf("%d decisions carried plans without a planner", bare.withPlans)
	}
}
