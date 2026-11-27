package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEquipFlashLowers: the Activated.Equip arm of Mode$ CastWithFlash
// (static_equipflash.go) lowers Leonin Shikari with no refusals. Forge Anew's
// CastWithFlash static lowers too (its PlayerTurn gate on Cond2); the card stays
// refused by its independent AlternativeCost Count$ThisTurnActivated blocker.
func TestEquipFlashLowers(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Leonin Shikari\nForge Anew\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]*lCard{}
	for _, c := range lowerPool(p).cards {
		by[c.Name] = c
	}
	shikari, anew := by["Leonin Shikari"], by["Forge Anew"]
	if shikari == nil || anew == nil {
		t.Fatalf("not lowered: shikari=%v anew=%v", shikari != nil, anew != nil)
	}
	if r := refusalsOf(shikari); len(r) != 0 {
		t.Errorf("Leonin Shikari refused: %v", r)
	}
	var lir strings.Builder
	for _, s := range shikari.Statics {
		lir.WriteString(s.Kind + ";")
	}
	if !strings.Contains(lir.String(), equipFlashKind) {
		t.Errorf("Leonin Shikari statics = %q, want an %s static", lir.String(), equipFlashKind)
	}
	r := refusalsOf(anew)
	if len(r) == 0 {
		t.Fatal("Forge Anew lowers with no refusal; its AlternativeCost blocker is gone")
	}
	for _, reason := range r {
		if strings.Contains(reason, "CastWithFlash shape") {
			t.Errorf("Forge Anew still refuses the CastWithFlash shape: %q", reason)
		}
		if !strings.Contains(reason, "ThisTurnActivated") {
			t.Errorf("Forge Anew refusal %q is not the AlternativeCost blocker", reason)
		}
	}
}
