package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFiendHunterRemPersist: Fiend Hunter's leaves trigger returns the
// enters trigger's exiled card through Defined$ DirectRemembered (gorge's
// alias of RememberedLKI); it lowers as the remembered-persist pair, as
// Journey to Nowhere's Defined$ Remembered does.
func TestFiendHunterRemPersist(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Fiend Hunter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if len(lw.cards) != 1 || lw.cards[0].Name != "Fiend Hunter" {
		t.Fatalf("lowered %d cards, want Fiend Hunter", len(lw.cards))
	}
	c := lw.cards[0]
	var reasons []string
	for _, r := range c.Refusals {
		reasons = append(reasons, r.Node+": "+r.Reason)
	}
	if len(reasons) != 0 {
		t.Fatalf("Fiend Hunter refused: %s", strings.Join(reasons, "; "))
	}
	if !c.RemPersist {
		t.Fatal("Fiend Hunter lowered without CardDef.RemPersist: the leaves trigger would read an empty set")
	}
}
