package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFDCPodRowsPlay: pair fdc seats four FDC precons in a Commander game at
// 40 life with each seat's commander named, rotates the pods, and the random
// and bot rows each play a whole 4-seat game.
func TestFDCPodRowsPlay(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	w := workload{reg: reg, paidManaCensus: true}
	cfgOf, err := w.games("fdc", 1)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for g := 0; g < podSeats*len(fdcDecks); g++ {
		cfg := cfgOf(g)
		if len(cfg.Decks) != podSeats || len(cfg.Commanders) != podSeats || cfg.Format != rules.FormatCommander || cfg.StartingLife != 40 {
			t.Fatalf("game %d: %d decks, %d commanders, format %v, life %d", g, len(cfg.Decks), len(cfg.Commanders), cfg.Format, cfg.StartingLife)
		}
		key := ""
		for _, n := range cfg.Names {
			key += n + "|"
		}
		seen[key] = true
	}
	if len(seen) != podSeats*len(fdcDecks) {
		t.Fatalf("%d distinct seatings in 20 games, want 20", len(seen))
	}
	if _, err := w.games("fdc-sub", 1); err == nil {
		t.Fatal("pair fdc-sub without -pod-dir was accepted")
	}
	var rs randomStats
	playRandomGame(cfgOf(0), &rs, nil, true)
	var bs botStats
	if err := playBotWithPool(cfgOf(1), true, &bs, nil); err != nil {
		t.Fatal(err)
	}
	if rs.Games != 1 || bs.Games != 1 || rs.Turns == 0 || bs.Turns == 0 {
		t.Fatalf("random %+v, bot %+v", rs, bs)
	}
	t.Logf("random: %d turns %d decisions %d stalls; bot: %d turns %d intents %d stalls", rs.Turns, rs.Decisions, rs.Stalls, bs.Turns, bs.Intents, bs.Stalls)
}
