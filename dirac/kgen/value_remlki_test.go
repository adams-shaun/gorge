package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRemLKIPTRefusals: a RememberedLKI$ read the ability program does not
// carry would read an accumulator no Sacrifice fills, so it is refused:
// Reign of the Pit's and Soul Separator's TokenPower$/TokenToughness$
// overrides. The program reads (Tribute to Hunger, Twisted Justice, Xathrid
// Demon) lower.
func TestRemLKIPTRefusals(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "remlki.txt")
	if err := os.WriteFile(list, []byte("Reign of the Pit\nSoul Separator\nTribute to Hunger\nTwisted Justice\nXathrid Demon\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	const want = "(a read outside an ability program)"
	for i, name := range p.names {
		refused := false
		for _, r := range lw.cards[i].Refusals {
			refused = refused || strings.Contains(r.Reason, want)
		}
		outside := name == "Reign of the Pit" || name == "Soul Separator"
		switch {
		case outside && !refused:
			t.Errorf("%s: want a refusal containing %q, got %v", name, want, lw.cards[i].Refusals)
		case !outside && len(lw.cards[i].Refusals) != 0:
			t.Errorf("%s: refused %v", name, lw.cards[i].Refusals)
		}
	}
}
