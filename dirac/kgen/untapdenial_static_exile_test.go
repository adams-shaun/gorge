package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUntapDenialEmitsNoDelayedTrigger: Exhaustion's Phase$ Untap self-exile
// is Static$ True, which gorge resolves inline (CR 611.2a duration
// bookkeeping, not a CR 603.7 delayed trigger on the stack), so the lowering
// must not pose a delayed program that puts a stack object at the target's
// upkeep.
func TestUntapDenialEmitsNoDelayedTrigger(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Exhaustion\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if len(lw.cards) != 1 || len(lw.cards[0].Refusals) != 0 {
		t.Fatalf("precondition: Exhaustion lowers unrefused, got %+v", lw.cards)
	}
	var sb strings.Builder
	dumpLIR(&sb, lw.cards[0])
	ir := sb.String()
	if !strings.Contains(ir, "SkipUntap") {
		t.Fatalf("precondition: lowered IR lacks SkipUntap:\n%s", ir)
	}
	if strings.Contains(ir, "DelayedTrigger") {
		t.Fatalf("lowered IR poses a delayed trigger for the Static$ True self-exile:\n%s", ir)
	}
}
