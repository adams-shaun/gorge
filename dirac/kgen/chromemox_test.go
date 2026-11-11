package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestChromeMoxLowers: Chrome Mox's imprint trigger, its reflected mana
// ability over Defined.Imprinted and its two bookkeeping static triggers
// lower with no refusal.
func TestChromeMoxLowers(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Chrome Mox\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if len(lw.cards) != 1 || lw.cards[0].Name != "Chrome Mox" {
		t.Fatalf("lowered %d cards, want Chrome Mox", len(lw.cards))
	}
	c := lw.cards[0]
	var reasons []string
	for _, r := range c.Refusals {
		reasons = append(reasons, r.Node+": "+r.Reason)
	}
	if len(reasons) != 0 {
		t.Fatalf("Chrome Mox refused: %s", strings.Join(reasons, "; "))
	}
}
