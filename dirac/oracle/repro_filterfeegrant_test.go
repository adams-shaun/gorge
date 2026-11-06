package oracle

import "testing"

// TestStdTlaFilterFeePaidByGrantedSourceLockstep: Rumble Arena's {1},{T}
// filter fee is funded by Honest Work's granted {C} before a printed Forest,
// as gorge's one prefix search activates every census gap first (CR 605.3a,
// 601.2g-h), so the Agna Qel'a activation taps Turtle-Duck and leaves the
// forests for the cost.
func TestStdTlaFilterFeePaidByGrantedSourceLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-tla", []stdRepro{
		{"filter_fee_paid_by_granted_source", false, false},
	})
}
