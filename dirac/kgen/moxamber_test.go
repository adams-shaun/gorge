package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMoxAmberLowers: Mox Amber's lone reflected mana ability over its
// legendary creatures and planeswalkers lowers with no refusal.
func TestMoxAmberLowers(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Mox Amber\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if len(lw.cards) != 1 || lw.cards[0].Name != "Mox Amber" {
		t.Fatalf("lowered %d cards, want Mox Amber", len(lw.cards))
	}
	var reasons []string
	for _, r := range lw.cards[0].Refusals {
		reasons = append(reasons, r.Node+": "+r.Reason)
	}
	if len(reasons) != 0 {
		t.Fatalf("Mox Amber refused: %s", strings.Join(reasons, "; "))
	}
}
