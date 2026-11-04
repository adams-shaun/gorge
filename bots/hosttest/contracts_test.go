package hosttest_test

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/seat"
)

// TestLinkedPoliciesKeepTheRegistryContracts runs bots/all's two
// registry-wide contracts (TestRegisteredBotsConsumePaymentPlans and
// TestRegisteredSearchAndNetBotsExplain) over the policies this package links
// on top of bots/all: each consumes the engine's payment plans under auto-pay
// unless named below, and each entry that declares Info.Search hands a
// driver's SetExplain to its search through its outermost seat.
func TestLinkedPoliciesKeepTheRegistryContracts(t *testing.T) {
	optOut := map[string]string{
		"search": "the teacher seat searches the manual option list over a manual-tap default bot; neither reads a payment plan",
	}
	for _, name := range linkedPolicies {
		e, ok := bots.Lookup(name)
		if !ok {
			t.Errorf("%s is not registered", name)
			continue
		}
		s, err := bots.New(name, bots.Options{Seed: 1, AutoPayMana: true, Deps: bots.Deps{Cards: &cards.Registry{}}})
		if err != nil {
			t.Errorf("%s: build: %v", name, err)
			continue
		}
		pc, ok := s.(seat.PaymentPlanConsumer)
		switch {
		case !ok:
			t.Errorf("%s (%T) does not implement seat.PaymentPlanConsumer", name, s)
		case !pc.WantsPaymentActions() && optOut[name] == "":
			t.Errorf("%s (%T).WantsPaymentActions() = false under AutoPayMana with no stated reason", name, s)
		}
		if _, ok := s.(explain.Emitter); e.Search && !ok {
			t.Errorf("%s (%T) declares Info.Search but does not implement explain.Emitter", name, s)
		}
	}
}
