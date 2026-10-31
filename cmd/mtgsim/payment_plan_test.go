package main

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/rules"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	sbseat "github.com/adams-shaun/gorge/server/spellbench/seat"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// planRecordingSeat wraps a SpellBench v2 Agent and counts the priority
// windows the driver built payment plans for and the windows the agent
// answered with a payment intent. It is the discriminating evidence: a seat
// that is offered only the engine's manual cast options can still pick
// cast_spell, but it can never apply a Payment.
type planRecordingSeat struct {
	*sbseat.Agent
	mu         sync.Mutex
	priority   int
	plans      int
	payIntents int
}

var _ engineseat.PaymentPlanConsumer = (*planRecordingSeat)(nil)

func (r *planRecordingSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	r.mu.Lock()
	if d.Kind == decision.KPriority {
		r.priority++
		if len(d.PaymentActions) > 0 {
			r.plans++
		}
	}
	r.mu.Unlock()
	in, err := r.Agent.Decide(ctx, v, d)
	if in.Payment != nil {
		r.mu.Lock()
		r.payIntents++
		r.mu.Unlock()
	}
	return in, err
}

// TestDriveMatchBuildsPaymentPlansForAgentSeat plays the Burn mirror through
// driveMatch, the mtgsim driver, with the neutral gorge agent in seat 0. The
// seat declares itself a PaymentPlanConsumer, so the driver must call
// EnsurePaymentActions before projecting the decision; without that call the
// agent sees no plan candidates and can never apply a payment intent.
func TestDriveMatchBuildsPaymentPlansForAgentSeat(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	dk, ok := catalog.ByID("Burn")
	if !ok {
		t.Fatal("catalog deck Burn is missing")
	}
	burn, err := catalog.Resolve(reg, dk)
	if err != nil {
		t.Fatal(err)
	}
	cfg := rules.Config{
		Names: []string{"Burn", "Burn"}, Decks: [][]*cards.Card{burn, burn},
		Tokens: reg.Tokens, Seed: 7, NameUniverse: reg.Universe(),
	}
	e := rules.New(cfg)
	e.Advance()

	a, err := sbseat.NewInProcess("sb-payment-plan", func(in io.Reader, out io.Writer) error {
		s, err := agent.New("bot")
		if err != nil {
			return err
		}
		s.SetRegistry(reg)
		if err := s.EnableNeutral(); err != nil {
			return err
		}
		return agent.Serve(in, out, s)
	}, bots.Options{Seed: 7, AutoPayMana: true})
	if err != nil {
		t.Fatal(err)
	}
	rec := &planRecordingSeat{Agent: a}
	mc := &bots.MatchContext{
		Context: context.Background(), Game: gorgegame.Game{E: e}, GameID: "g-payment-plan",
		Seat: 0, Seats: 2, Format: "constructed", Seed: []byte("7"),
		OwnDeck: e.OwnDeck(state.PlayerID(0)), Mulligan: "none",
		StartingSeat: int(e.G.Active), Logf: t.Logf, Registry: reg,
	}
	if err := bots.AttachMatch(rec, mc); err != nil {
		t.Fatal(err)
	}
	defer bots.EndMatch(rec, bots.MatchResult{Classification: "halted"})

	if _, err := driveMatch(e, []engineseat.Seat{rec, heuristic.NewBot(8)}); err != nil {
		t.Fatalf("driveMatch: %v", err)
	}
	if f := a.Forfeit(); f != "" {
		t.Fatalf("seat forfeited: %s", f)
	}
	// Precondition: the handler ran on the agent's own priority windows, so
	// a zero plan count below is "no plans were built", not "the seat was
	// never asked".
	if rec.priority == 0 {
		t.Fatal("the agent was handed no priority decision")
	}
	if rec.plans == 0 {
		t.Fatalf("the driver built no payment plan across %d priority windows for a declared consumer", rec.priority)
	}
	if rec.payIntents == 0 {
		t.Fatalf("the agent applied no payment intent across %d planned windows", rec.plans)
	}
}
