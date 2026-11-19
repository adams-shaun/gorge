package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClashTwoSeat: a Clash lowering reads "the one opponent" (clashOpp ->
// State.oppsFrom), which is gorge's first living opponent in APNAP order from
// seat 0 (engine/effects/clash.go clashParticipants: g.AliveFrom(0)). That is
// the same player only in a two-player game, so every clash carrier must be
// marked two-seat only (kop.CardDef.TwoSeat, refused at three or more seats).
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
		if !c.TwoSeat {
			t.Errorf("%s: Clash reads the one opponent but the lowering is not marked two-seat", c.Name)
		}
		if len(c.TwoSeatWhy) == 0 {
			t.Errorf("%s: two-seat only with no reason", c.Name)
		}
	}
}
