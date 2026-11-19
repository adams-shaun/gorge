package decision

import "testing"

// TestAttackOneBeltReachesTheCoreAfterTheFold pins the FitRequired belt after
// the repair passes: when the fold's answer spends the budget on unmarked
// picks, the cheapest marked option cannot join it, so the belt re-derives
// from the required core -- where a discharging pick always fits -- instead of
// handing back an answer Validate rejects (a rejected answer sends the
// deterministic bot re-deriving forever).
func TestAttackOneBeltReachesTheCoreAfterTheFold(t *testing.T) {
	d := &Decision{Kind: KAttackers, Min: 0, Max: 3, MaxSum: 7, Options: []Option{
		{Index: 0, Kind: "attacker", Obj: 1, Player: 1, Value: 5, AttackOne: true},
		{Index: 1, Kind: "attacker", Obj: 2, Player: 2, Value: 5},
	}}
	if d.AttackOneSatisfied([]int{1}) {
		t.Fatal("precondition: the unmarked answer must not satisfy the duty")
	}
	if d.AttackOneSatisfied([]int{0}) != true || d.Validate(Intent{Choices: []int{0}}) != nil {
		t.Fatal("precondition: the marked pair alone is not a legal answer")
	}
	got := d.FitRequired([]int{1})
	if !d.AttackOneSatisfied(got) || d.Validate(Intent{Choices: got}) != nil {
		t.Fatalf("FitRequired([1]) = %v, want a legal answer that discharges the duty", got)
	}
}
