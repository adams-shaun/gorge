package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOutcomeGapCastCensus pins two generated games the v2 seat used to lose
// to its native lane: holding a card no tap could cast, the seat passed with a
// planned creature in hand, because the neutral world's decision lacked the
// cast census the native bot's tap gate reads. The lanes now play one game.
// server/spellbench/neutral/cast_census_test.go pins the mechanism without
// depending on the generated decks.
func TestOutcomeGapCastCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 83, 595)
	o := sbOptions{mode: "outcome", policy: "bot", autopay: true, maxTurns: 100, maxIntents: 20000, maxObjects: 20000}
	for _, idx := range []int{178, 594} {
		j := jobs[idx]
		native := playSBLane(reg, j.decks, j.seed, o, []bool{false, false})
		v2 := playSBLane(reg, j.decks, j.seed, o, []bool{true, false})
		if !native.done || !v2.done {
			t.Fatalf("pair %d did not finish: native %s, v2 %s", idx, native, v2)
		}
		if native.winner != v2.winner || native.turns != v2.turns || native.intents != v2.intents {
			t.Fatalf("pair %d: the lanes played different games: native %s, v2 %s", idx, native, v2)
		}
	}
}
