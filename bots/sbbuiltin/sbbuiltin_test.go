package sbbuiltin

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/spellbench/builtins"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
)

func TestSBBuiltinRegistrationAndSharedConstructors(t *testing.T) {
	for _, c := range []struct {
		name   string
		policy builtins.Policy
		ctor   func(uint64, builtins.ManaMode) *builtins.Seat
	}{
		{FirstPolicy, builtins.First, NewFirst},
		{UniformPolicy, builtins.Uniform, NewUniform},
		{HeuristicPolicy, builtins.Heuristic, NewHeuristic},
	} {
		t.Run(c.name, func(t *testing.T) { checkBuiltin(t, c.name, c.policy, c.ctor) })
	}
}

func checkBuiltin(t *testing.T, name string, want builtins.Policy, ctor func(uint64, builtins.ManaMode) *builtins.Seat) {
	entry, ok := bots.Lookup(name)
	if !ok || !entry.Env || entry.Search || entry.Caretaker != "bot" {
		t.Fatalf("entry = %+v, registered=%v; want Env non-search policy with bot caretaker", entry, ok)
	}
	seat, err := bots.New(name, bots.Options{Seed: 17, AutoPayMana: true})
	if err != nil {
		t.Fatal(err)
	}
	envSeat, ok := seat.(bots.EnvSeat)
	if !ok {
		t.Fatalf("built %T, want EnvSeat", seat)
	}
	refuser, ok := seat.(bots.RefusalAnswerer)
	if !ok {
		t.Fatalf("built %T, want RefusalAnswerer", seat)
	}
	payment, ok := seat.(interface{ WantsPaymentActions() bool })
	if !ok || !payment.WantsPaymentActions() {
		t.Fatal("AutoPayMana=true did not enable payment actions on the hosted seat")
	}
	if !envSeat.WantsEnv(&decision.Decision{Kind: decision.KPriority}) {
		t.Fatal("priority does not request Env; honest planner would not be installed")
	}
	for _, kind := range decision.Kinds {
		if kind != decision.KPriority && envSeat.WantsEnv(&decision.Decision{Kind: kind}) {
			t.Errorf("WantsEnv(%s) = true; only priority uses the planner", kind)
		}
	}

	shared := ctor(23, builtins.AutoPay)
	if shared.Policy() != want || !shared.WantsPaymentActions() {
		t.Fatalf("NewWithMode returned policy %v, wants payment actions %v", shared.Policy(), shared.WantsPaymentActions())
	}
	// Calling the adapter's refusal rung reaches the wrapped builtin handler.
	refuser.AnswerRefused(view.View{}, decision.Decision{}, decision.Intent{})
	if got := shared.Stats.Refusals; got != 0 {
		t.Fatalf("independent shared constructor unexpectedly has %d refusals", got)
	}
	wrapped := seat.(*hostedSeat)
	if wrapped.bot.Stats.Refusals != 1 {
		t.Fatalf("AnswerRefused reached wrapped handler %d times, want 1", wrapped.bot.Stats.Refusals)
	}
}
