package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// boundCensusHost answers the census/prefix fallback with an incomplete census
// that proves nothing, and counts the prefix searches the fallback attempts.
type boundCensusHost struct {
	prefixCalls int
}

func (h *boundCensusHost) paymentPlanCensusOf(state.PlayerID, *state.Mana) paymentPlanCensus {
	return paymentPlanCensus{}
}

func (h *boundCensusHost) paymentPlanRelaxProof(paymentPlanCensus, func() PaymentPlanOutcome) PaymentPlanOutcome {
	return PaymentPlanOutcome{Reason: pay.ReasonInsufficient}
}

func (h *boundCensusHost) potentialTargetPriced(state.PlayerID, decision.Option) bool { return false }

func (h *boundCensusHost) potentialPrefixScript(state.PlayerID, decision.Option, []state.ObjID, int) ([]ScriptStep, bool) {
	h.prefixCalls++
	return nil, false
}

// TestPotentialCensusFallbackHonoursTheWalkBound pins the may-play fallback's
// script budget to the caller's scriptPlays: EnsurePotentialProofs walks with
// scriptPlays=0 and must run no prefix search, while the offer walk may use its
// budget.
func TestPotentialCensusFallbackHonoursTheWalkBound(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		bound      int
		wantPrefix int
	}{
		{"proof walk has no budget", 0, 0},
		{"offer walk has a budget", potentialScriptPlays, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := &boundCensusHost{}
			censused := false
			scripts := 0
			var census paymentPlanCensus
			verdict := func() PaymentPlanOutcome {
				return PaymentPlanOutcome{Reason: pay.ReasonUnsupported, Detail: pay.DetailWitnessNotOffered}
			}
			got, script := potentialCensusFallback(h, 0, decision.Option{}, state.Mana{},
				verdict(), true, verdict, tc.bound, &census, &censused, &scripts)
			if h.prefixCalls != tc.wantPrefix {
				t.Fatalf("prefix searches = %d, want %d", h.prefixCalls, tc.wantPrefix)
			}
			if got.Reason != pay.ReasonUnsupported || got.Detail != pay.DetailWitnessNotOffered {
				t.Fatalf("verdict = %+v, want unsupported/%s", got, pay.DetailWitnessNotOffered)
			}
			if script != nil {
				t.Fatalf("script = %v, want nil", script)
			}
		})
	}
}
