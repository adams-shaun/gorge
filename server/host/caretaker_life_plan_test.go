package host

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/sanity"
)

// The host's human-seat caretaker is built with SkipLifePlans (spec §6): for
// every hosted policy, offered only a life-paying plan, it answers from the
// ordinary options, while the same policy's hosted bot submits the plan
// (RunCaretakerLifePlanContract).
func TestCaretakerNeverSelectsLifePayingPlan(t *testing.T) {
	for _, policy := range []string{BotPolicy, LethalPressurePolicy} {
		t.Run(policy, func(t *testing.T) { RunCaretakerLifePlanContract(t, policy) })
	}
	// Without auto-pay the caretaker is the plain policy bot.
	s, err := newCaretakerSeat(BotPolicy, 19, false, false, bots.Deps{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if b, ok := s.(*heuristic.Bot); !ok || b.WantsPaymentActions() {
		t.Fatalf("caretaker without auto-pay = %T wants payment actions", s)
	}
}

// The guard decorator must not hide the *heuristic.Bot from the caretaker's
// SkipLifePlans arming (it exposes UnwrapSeat).
func TestCaretakerSkipLifePlansSeesThroughGuard(t *testing.T) {
	b := heuristic.NewBot(19).EnableAutoPayMana()
	skipLifePlans(sanity.Wrap(b), true)
	d, v := lifeOnlyDecision()
	in, err := b.Decide(context.Background(), v, d)
	if err != nil {
		t.Fatal(err)
	}
	if in.Payment != nil {
		t.Fatalf("guarded caretaker submitted the life-paying plan: %+v", in.Payment)
	}
}
