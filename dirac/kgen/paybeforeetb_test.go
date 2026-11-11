package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMoxDiamondLowers: Mox Diamond's pay-before-entering replacement lowers
// with no refusal, as a ReplPayBeforeETB ability whose Prog is the one
// optional, remembering discard of a land card from the hand.
func TestMoxDiamondLowers(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Mox Diamond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if len(lw.cards) != 1 || lw.cards[0].Name != "Mox Diamond" {
		t.Fatalf("lowered %d cards, want Mox Diamond", len(lw.cards))
	}
	var reasons []string
	for _, r := range lw.cards[0].Refusals {
		reasons = append(reasons, r.Node+": "+r.Reason)
	}
	if len(reasons) != 0 {
		t.Fatalf("Mox Diamond refused: %s", strings.Join(reasons, "; "))
	}
	n := 0
	for _, r := range lw.cards[0].Repls {
		if r.Kind != "PayBeforeETB" {
			continue
		}
		n++
		if len(r.Prog) != 1 || r.Prog[0].Op != "Discard" || r.Prog[0].C&int(kop.OptAsk) == 0 || r.Prog[0].Note != noteRememberDiscarded {
			t.Fatalf("PayBeforeETB prog %+v, want one optional remembering Discard", r.Prog)
		}
	}
	if n != 1 {
		t.Fatalf("%d PayBeforeETB replacements, want 1", n)
	}
}
