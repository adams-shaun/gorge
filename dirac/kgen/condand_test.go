package kgen

import (
	"strings"
	"testing"
)

// condAndCards are the statics that carry two gates (Condition$ plus
// CheckSVar$/IsPresent$). All seven must stop refusing with "two static
// conditions" and show the conjunction in their -lir dump (Pick Up the Pace
// and Theater of Horrors read Card.ExiledWithSource, which lowers as the
// FExiledWith flag; Theater of Horrors still refuses
// later, at emit, on its mixed player/planeswalker target).
var condAndCards = []string{
	"Grond, the Gatebreaker",
	"Incisor Steed",
	"Momo, Friendly Flier",
	"Of One Mind",
	"Pick Up the Pace",
	"Sardian Cliffstomper",
	"Theater of Horrors",
}

func TestStaticCondAnd(t *testing.T) {
	seen := 0
	for _, c := range lowerNamed(t, condAndCards) {
		seen++
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "two static conditions") {
				t.Errorf("%s: still refuses %q", c.Name, r.Reason)
			}
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), "And(") {
			t.Errorf("%s: lowered IR lacks the conjunction:\n%s", c.Name, sb.String())
		}
	}
	if seen != len(condAndCards) {
		t.Errorf("lowered %d of %d cards", seen, len(condAndCards))
	}
}
