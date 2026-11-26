package codeshape

import "testing"

// TestGrowthRuleAdmitsOneStringsCall pins the scenario that the stored
// ceiling-plus-slack design failed: engine/rules at about 1700 strings.*
// calls, a PR adding one. The rule compares with the value at the merge base,
// so one added call is admitted with room to spare (limit 1699 + 33 = 1732).
func TestGrowthRuleAdmitsOneStringsCall(t *testing.T) {
	const base, head, wantLimit = 1699, 1700, 1732
	if head <= base {
		t.Fatalf("precondition: head %d must exceed base %d", head, base)
	}
	if got := base + CountAllowance(base, CountGrowthPercent, CountGrowthFloor); got != wantLimit {
		t.Fatalf("limit = %d, want %d (CountGrowthPercent %d, CountGrowthFloor %d)",
			got, wantLimit, CountGrowthPercent, CountGrowthFloor)
	}
	fails, oks := CheckCountGrowth([]CountSample{{Name: "engine/rules: strings.* calls", Base: base, Head: head}},
		CountGrowthPercent, CountGrowthFloor)
	if len(fails) != 0 {
		t.Errorf("adding one strings.* call at base %d failed: %+v", base, fails)
	}
	if len(oks) != 1 || oks[0].Limit != wantLimit {
		t.Errorf("oks = %+v, want one ok with limit %d", oks, wantLimit)
	}
}
