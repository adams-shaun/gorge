package azc3_test

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/engine/seat"
)

// az-c3 and az-c4 (azc4 reuses azc3.ClosedConfig) search the auto-payment
// vocabulary, so the hosted seat must consume the engine's payment plans:
// without them its wrapped bot answers manual mana taps that the search
// cannot see. They are not in bots/all (they need Enable), so the
// enumeration test there cannot reach them.
func TestHostedAZC3ConsumesPaymentPlans(t *testing.T) {
	s, err := azredeal.NewWithModel(bots.Options{Seed: 1, AutoPayMana: true}, azc3.ClosedConfig(nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	pc, ok := s.(seat.PaymentPlanConsumer)
	if !ok || !pc.WantsPaymentActions() {
		t.Fatalf("%T: an az-c3/az-c4 seat must answer WantsPaymentActions true", s)
	}
}
