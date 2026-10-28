package host

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	sbseat "github.com/adams-shaun/gorge/server/spellbench/seat"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// sbPlanRecordingSeat wraps a SpellBench v2 Agent and counts the priority
// windows the host published payment plans for and the windows the agent
// answered with a payment intent.
type sbPlanRecordingSeat struct {
	*sbseat.Agent
	mu         sync.Mutex
	priority   int
	plans      int
	payIntents int
}

var _ seat.PaymentPlanConsumer = (*sbPlanRecordingSeat)(nil)

func (r *sbPlanRecordingSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
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

// TestHostBuildsPaymentPlansForV2AgentSeat pins the gorged half of the
// contract: server/host/match.go builds the lazy payment extension for a
// PaymentPlanConsumer seat before cloning the decision, and the real v2 agent
// then applies a payment intent. It fails if that guard is lost or the seat
// stops declaring PaymentPlanConsumer.
func TestHostBuildsPaymentPlansForV2AgentSeat(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const seed = 7

	var builtMu sync.Mutex
	var built []*sbPlanRecordingSeat
	o := testOptions(t)
	o.LoadDeck = repoDeckLoader(t)
	o.Seats = func(names []string, gotSeed uint64) []seat.Seat {
		a, err := sbseat.NewInProcess("sb-host-payment-plan", func(in io.Reader, out io.Writer) error {
			s, err := agent.New("bot")
			if err != nil {
				return err
			}
			s.SetRegistry(reg)
			if err := s.EnableNeutral(); err != nil {
				return err
			}
			return agent.Serve(in, out, s)
		}, bots.Options{Seed: gotSeed, AutoPayMana: true})
		if err != nil {
			t.Errorf("build in-process seat: %v", err)
			return nil
		}
		rec := &sbPlanRecordingSeat{Agent: a}
		builtMu.Lock()
		built = append(built, rec)
		builtMu.Unlock()
		return []seat.Seat{rec, heuristic.NewBot(gotSeed + 1)}
	}

	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(TableConfig{
		ID: "t1", Seats: 2, Decks: []string{"death-n-taxes", "dimir-tempo"},
		Seed: seed, Pace: 0, Spectator: view.Omniscient,
		AutoMana: true, BotAutoPayMana: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	builtMu.Lock()
	seats := append([]*sbPlanRecordingSeat(nil), built...)
	builtMu.Unlock()
	if len(seats) != 1 {
		t.Fatalf("built %d v2 agent seats, want 1", len(seats))
	}
	rec := seats[0]
	if f := rec.Agent.Forfeit(); f != "" {
		t.Fatalf("seat forfeited: %s", f)
	}
	// Precondition: the handler ran on the agent's own priority windows.
	if rec.priority == 0 {
		t.Fatal("the agent was handed no priority decision")
	}
	if rec.plans == 0 {
		t.Fatalf("the host built no payment plan across %d priority windows for a declared consumer", rec.priority)
	}
	if rec.payIntents == 0 {
		t.Fatalf("the agent applied no payment intent across %d planned windows", rec.plans)
	}
}
