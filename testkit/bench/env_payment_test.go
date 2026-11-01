package bench_test

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// planEnvSeat answers its priority decisions with a payment plan chosen on
// the honest root (a seat that plans payments on its own root and opts out
// of PaymentPlanConsumer, so the driver never builds the plans on
// the live decision for them). It counts the plan answers and the Env
// decisions that arrived with no feed after one -- a feed the driver stopped.
type planEnvSeat struct {
	*heuristic.Bot
	plans, fedAfter, unfedAfter int
}

// WantsPaymentActions shadows the embedded auto-pay bot's: the driver must
// not pre-build the live decision's plans for this seat.
func (s *planEnvSeat) WantsPaymentActions() bool { return false }

func (s *planEnvSeat) WantsEnv(d *decision.Decision) bool { return d.Kind == decision.KPriority }

func (s *planEnvSeat) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	if s.plans > 0 {
		if env.Search.Feed == nil {
			s.unfedAfter++
		} else {
			s.fedAfter++
		}
	}
	root := env.Search.Engine
	if root == nil || root.Pending() == nil || root.Pending().Seq != d.Seq {
		return s.Bot.DecideBoard(ctx, env.Board, d)
	}
	root.EnsurePaymentActions()
	rd := root.Pending().Clone()
	in, err := s.Bot.DecideBoard(ctx, env.Board, *rd)
	if err == nil && (in.Payment != nil || in.Announce != nil) {
		s.plans++
	}
	return in, err
}

// TestEnvSeatPaymentPlanAnswerKeepsItsFeed: a payment-plan answer from an
// EnvSeat is recorded on its feed without stopping it. If the live decision's
// PaymentActions are not built for such a seat, the recorded answer does not
// translate and the seat is fed nothing (no honest root, its search silently
// off) for the rest of the game.
func TestEnvSeatPaymentPlanAnswerKeepsItsFeed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	deckA, deckB := fdnPair(t, reg)
	const seed = 7717
	s := &planEnvSeat{Bot: heuristic.NewBot(seed ^ 1).EnableAutoPayMana()}
	o, _, err := bench.PlayGame(envCfg(seed, reg, deckA, deckB),
		[]seat.Seat{s, heuristic.NewBot(seed ^ 2)}, envTestTurns, envTestIntents,
		bench.Hooks{EnvSeats: true, HonestRoot: true})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if o.IsStalled() {
		t.Fatalf("stalled on %s", o.StallOn)
	}
	if s.plans == 0 {
		t.Fatal("the seat never answered with a payment plan; the test exercises nothing")
	}
	if s.unfedAfter > 0 {
		t.Fatalf("after a payment-plan answer %d Env decisions had no feed (%d had one): the answer stopped the feed", s.unfedAfter, s.fedAfter)
	}
}
