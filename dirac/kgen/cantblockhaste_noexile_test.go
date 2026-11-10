package kgen

import (
	"strings"
	"testing"
)

// TestEffectCantBlockByHasteNoExile: Speed, Young Avenger's targeted Effect
// (ValidBlocker$ Creature.withoutHaste, no ExileOnMoved$) lowers to ExHaste on
// the target, as the exile form does.
func TestEffectCantBlockByHasteNoExile(t *testing.T) {
	cs := lowerNamed(t, []string{"Speed, Young Avenger"})
	if len(cs) != 1 {
		t.Fatalf("got %d cards", len(cs))
	}
	c := cs[0]
	for _, r := range c.Refusals {
		t.Errorf("refused: %s: %s", r.Node, r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, c)
	if !strings.Contains(sb.String(), "ExHaste obj=Target") {
		t.Errorf("no ExHaste:\n%s", sb.String())
	}
	if strings.Contains(sb.String(), "CS=Unblockable") {
		t.Errorf("lowered as plain Unblockable:\n%s", sb.String())
	}
}
