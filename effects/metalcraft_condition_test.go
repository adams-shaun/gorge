package effects

import "testing"

// TestConditionBareMetalcraftReportsTheCensus proves the bare gate resolves
// both below and above the shared rules-side census threshold.
func TestConditionBareMetalcraftReportsTheCensus(t *testing.T) {
	h, ids := conditionBoard(t)
	gate := sa(t, "DB$ Draw | Condition$ Metalcraft")
	ctx := &Ctx{Controller: 0, Source: ids[3]}
	if met, resolved := conditionMet(h, ctx, gate); !resolved || met {
		t.Fatalf("no metalcraft: met=%v resolved=%v, want false true", met, resolved)
	}
	h.metalcraft = true
	if met, resolved := conditionMet(h, ctx, gate); !resolved || !met {
		t.Fatalf("metalcraft: met=%v resolved=%v, want true true", met, resolved)
	}
}
