package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTrigOwnExileGraveN: a printed trigger's own ExileFromGrave<N/Spec> cost
// with N > 1 lowers to an IfPaid ExileGrave of N.
func TestTrigOwnExileGraveN(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Aegis Sculptor\nEmeritus of Ideation\nSkywarp Skaab\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if len(lw.cards) != 3 {
		t.Fatalf("%d lowered cards", len(lw.cards))
	}
	for _, c := range lw.cards {
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %v", c.Name, c.Refusals)
			continue
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		out := sb.String()
		t.Logf("%s:\n%s", c.Name, out)
		want := map[string]string{"Aegis Sculptor": "A=2", "Emeritus of Ideation": "A=8", "Skywarp Skaab": "A=2"}[c.Name]
		ok := false
		for _, ln := range strings.Split(out, "\n") {
			if strings.Contains(ln, "ExileGrave") && strings.Contains(ln, want) {
				ok = true
			}
		}
		if !ok {
			t.Errorf("%s: no ExileGrave %s in the lowered IR", c.Name, want)
		}
	}
}
