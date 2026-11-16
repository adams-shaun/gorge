package kgen

import (
	"strings"
	"testing"
)

// TestEffectCantBlockByUntilYourNextTurn: Sly Instigator's targeted Effect
// (Duration$ UntilYourNextTurn) lowers to the next-turn unblockable override
// record, not the end-of-turn GrantKeyword (CR 611.2b).
func TestEffectCantBlockByUntilYourNextTurn(t *testing.T) {
	cs := lowerNamed(t, []string{"Sly Instigator"})
	if len(cs) != 1 {
		t.Fatalf("got %d cards", len(cs))
	}
	for _, r := range cs[0].Refusals {
		t.Errorf("refused: %s: %s", r.Node, r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, cs[0])
	if !strings.Contains(sb.String(), "Override obj=Target0 CS=UntilYourNextTurn") {
		t.Errorf("no next-turn override:\n%s", sb.String())
	}
	if strings.Contains(sb.String(), "CS=Unblockable") {
		t.Errorf("lowered as the end-of-turn grant:\n%s", sb.String())
	}
}
