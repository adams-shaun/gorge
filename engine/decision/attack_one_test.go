package decision

import "testing"

// TestAttackOneRepairSwapsSameCreaturePair pins repairAttackOne's swap: a
// declaration that attacks with the only creature the wrong way, at a full
// Max, is repaired by moving that creature's attack to its marked pair (one
// pair per creature), and a duty no option carries binds nothing.
func TestAttackOneRepairSwapsSameCreaturePair(t *testing.T) {
	d := &Decision{Kind: KAttackers, Min: 0, Max: 1, Options: []Option{
		{Index: 0, Kind: "attacker", Obj: 1, Player: 2},
		{Index: 1, Kind: "attacker", Obj: 1, Player: 1, AttackOne: true},
	}}
	if d.AttackOneSatisfied([]int{0}) || d.AttackOneSatisfied(nil) {
		t.Fatal("an unmarked or empty declaration satisfied the duty")
	}
	if got := d.FitRequired([]int{0}); len(got) != 1 || got[0] != 1 {
		t.Fatalf("FitRequired([0]) = %v, want the marked pair [1]", got)
	}
	if err := d.Validate(Intent{Choices: d.FitRequired(nil)}); err != nil {
		t.Fatalf("repaired empty declaration invalid: %v", err)
	}
	free := &Decision{Kind: KAttackers, Min: 0, Max: 1, Options: []Option{{Index: 0, Kind: "attacker", Obj: 1, Player: 2}}}
	if !free.AttackOneSatisfied(nil) {
		t.Fatal("a duty no option carries must bind nothing")
	}
}
