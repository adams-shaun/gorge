package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiscardSelfDescription: Discard<1/CARDNAME/this card> (the description
// is prompt text only, gorge cost.parse) lowers to the same hand-zone
// CostDiscardSelf as the two-part spelling; on the battlefield it still refuses.
func TestDiscardSelfDescription(t *testing.T) {
	carriers := []string{"Gideon's Memorial", "Sureshot Sower", "The Masters of Evil", "Action News Crew"}
	// a Plan card in the pool makes "Plan" a known subtype for Masters of Evil's search
	plan := "Villainous Syndication"
	lir, ref := lowerCarriers(t, append([]string{plan}, carriers...))
	for _, n := range carriers {
		for _, r := range ref[n] {
			t.Errorf("%s: refused: %s", n, r)
		}
	}
	if got := strings.Count(lir, "DiscardSelf"); got < len(carriers) {
		t.Errorf("lowered IR has %d DiscardSelf costs, want at least %d:\n%s", got, len(carriers), lir)
	}

	dir := t.TempDir()
	sub := filepath.Join(dir, "cardsfolder", "k")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "Name:Kchanneldesc Probe\nManaCost:1\nTypes:Artifact\n" +
		"A:AB$ Draw | Cost$ Discard<1/CARDNAME/this card> | NumCards$ 1 | SpellDescription$ Draw a card.\n" +
		"Oracle:Discard this card: Draw a card.\n"
	if err := os.WriteFile(filepath.Join(sub, "kchanneldesc_probe.txt"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte("Kchanneldesc Probe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, dir)
	if err != nil {
		t.Fatal(err)
	}
	hit := false
	for _, c := range lowerPool(p).cards {
		for _, r := range c.Refusals {
			hit = hit || strings.Contains(r.Reason, "outside a hand ability")
		}
	}
	if !hit {
		t.Error("battlefield Discard<1/CARDNAME/desc> did not refuse with \"outside a hand ability\"")
	}
}
