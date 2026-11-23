package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// cmdCastCarriers are the corpus cards whose sole value blocker was
// Count$TotalCommanderCastFromCommandZone; each now lowers end to end.
var cmdCastCarriers = []string{
	"Captain Vargus Wrath", "Commander's Insignia", "Echo Storm", "Empyrial Storm",
	"Font of Magic", "Fury Storm", "Hatut Zeraze Strike Force", "Jirina Kudro",
	"Jyoti, Moag Ancient", "The Swarmlord", "Vanguard of the Restless",
}

// TestCommanderCastCountLowers pins the Count$TotalCommanderCastFromCommandZone
// lowering (dirac/kgen/value_cmdcast.go): every paper carrier reads
// ExTotalCommanderCastFromCommandZone with no refusal, and the head builds into
// the pool tables. Count$CommanderCastFromCommandZone (one commander) refuses:
// gorge's Count$ table maps both spellings to the controller's total, so
// lowering the per-commander reading would diverge.
func TestCommanderCastCountLowers(t *testing.T) {
	lir, ref := lowerCarriers(t, cmdCastCarriers)
	for _, n := range cmdCastCarriers {
		for _, r := range ref[n] {
			t.Errorf("%s: refused: %s", n, r)
		}
		sec := section(lir, n)
		if !strings.Contains(sec, "CmdCastTotal") {
			t.Errorf("%s: no CmdCastTotal read lowered:\n%s", n, sec)
		}
	}

	// The eleven also build: the new value kind reaches the pool tables.
	kp := kfhBuild(t, cmdCastCarriers, nil)
	for _, n := range cmdCastCarriers {
		if got := refusalText(kp.cards[n]); got != "" {
			t.Errorf("%s: refused: %s", n, got)
		}
	}

	// The lowered kind carries the manifest's op: the kernel reads the
	// controller's per-slot cast counters (dirac/value_cmdcast.go).
	if exprKinds[vCmdCastTotal] != kop.ExTotalCommanderCastFromCommandZone {
		t.Errorf("exprKinds[vCmdCastTotal] = %v, want %v", exprKinds[vCmdCastTotal], kop.ExTotalCommanderCastFromCommandZone)
	}

	// The one-commander spelling refuses by name.
	mu := lowerCorpusCard(t, "Myth Unbound")
	if mu == nil {
		t.Fatal("Myth Unbound did not lower")
	}
	var got []string
	for _, r := range mu.Refusals {
		got = append(got, r.Reason)
	}
	joined := strings.Join(got, "; ")
	if !strings.Contains(joined, "one-commander count is not modelled") {
		t.Errorf("Myth Unbound: refusal does not name the unmodelled one-commander count: %s", joined)
	}
}
