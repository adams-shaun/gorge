package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClashTwoSeat: a Clash lowering's opponent is gorge's first living
// opponent in APNAP order from seat 0 (engine/effects/clash.go
// clashParticipants: g.AliveFrom(0)), which the kernel's clashOpp reads at
// any seat count, so no clash carrier is marked two-seat only for it.
func TestClashTwoSeat(t *testing.T) {
	heavyParallel(t)
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	var names []string
	for _, f := range clashForms {
		names = append(names, f.card)
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	for _, c := range lw.cards {
		if len(c.Refusals) != 0 {
			t.Errorf("%s: refused, so the clash lowering never ran: %v", c.Name, c.Refusals)
			continue
		}
		for _, w := range c.TwoSeatWhy {
			if strings.Contains(w, "Clash") {
				t.Errorf("%s: still marked two-seat for its clash: %s", c.Name, w)
			}
		}
	}
}
