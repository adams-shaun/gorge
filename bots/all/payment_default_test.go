package all_test

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/seat"
)

// paymentOptOut names the registered policies that may answer
// WantsPaymentActions false under Options.AutoPayMana, each with its reason.
// Every other registered bot must consume the engine's payment plans
// (seat.PaymentPlanConsumer's bot default): a bot that does not silently
// plays its fallback's manual mana taps.
var paymentOptOut = map[string]string{
	"az-redeal": "the generation-0 hosted config (azredeal.Hosted) searches the manual option list (Search.AutoPayment off); a checkpoint-backed successor searches the payment vocabulary and consumes plans",
	"search":    "the searchteacher seat searches the manual option list over a manual-tap default bot; neither reads a payment plan",
}

// TestRegisteredBotsConsumePaymentPlans builds every registered policy with
// auto-pay on and asserts it is a seat.PaymentPlanConsumer answering true,
// unless paymentOptOut names it.
func TestRegisteredBotsConsumePaymentPlans(t *testing.T) {
	for _, e := range bots.Entries() {
		name := e.Info.Name
		s, err := bots.New(name, bots.Options{Seed: 1, AutoPayMana: true, Deps: bots.Deps{Cards: &cards.Registry{}}})
		if err != nil {
			t.Errorf("%s: build: %v", name, err)
			continue
		}
		pc, ok := s.(seat.PaymentPlanConsumer)
		if !ok {
			t.Errorf("%s (%T) does not implement seat.PaymentPlanConsumer; a bot must answer it (forward to what it wraps, or true)", name, s)
			continue
		}
		if got := pc.WantsPaymentActions(); !got {
			if why, ok := paymentOptOut[name]; ok {
				t.Logf("%s opts out: %s", name, why)
				continue
			}
			t.Errorf("%s (%T).WantsPaymentActions() = false under AutoPayMana; bots consume payment plans unless paymentOptOut names a reason", name, s)
		}
	}
}
