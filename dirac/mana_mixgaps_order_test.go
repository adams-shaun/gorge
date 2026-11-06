package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAnyRStepsOrder pins the single-colour anyR gap's answer order against
// gorge's decisions: the "choose a mana ability" ask (answer 0, the printed
// first mana ability) comes before the colour ask, so it is not reordered by
// scriptOrderAnswers; the colour ask's node is charged after it, and only
// then do the play's announced colours (G here) precede the engine order.
// The previously wrong order charged the node first and tried G before
// answer 0.
func TestAnyRStepsOrder(t *testing.T) {
	var need [kop.NumManaKinds]bool
	need[kop.G] = true
	var got []int
	found := anyRSteps(need,
		func(ans uint8) bool {
			got = append(got, int(ans))
			return false
		},
		func() bool {
			got = append(got, -1) // the colour ask's node
			return true
		})
	if found {
		t.Fatal("anyRSteps reported a found script though every answer failed")
	}
	want := []int{0, -1, 5, 1, 2, 3, 4} // first ability, node, G, W, U, B, R
	if len(got) != len(want) {
		t.Fatalf("answer order %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("answer order %v, want %v", got, want)
		}
	}
}

// TestAnyRStepsTickGate: the colour ask's node is spent before the first
// colour answer, and a spent node tries none of them.
func TestAnyRStepsTickGate(t *testing.T) {
	var need [kop.NumManaKinds]bool
	var got []int
	found := anyRSteps(need,
		func(ans uint8) bool {
			got = append(got, int(ans))
			return false
		},
		func() bool {
			got = append(got, -1)
			return false
		})
	if found {
		t.Fatal("anyRSteps found a script after the node was spent")
	}
	if len(got) != 2 || got[0] != 0 || got[1] != -1 {
		t.Fatalf("answer order %v, want [0 -1]: the first ability, then the spent node", got)
	}
}
