package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOutcomePair134NoForfeit pins outcome pair 134 of the seed-61 run: the
// v2 lane's search over a compound Library,Graveyard origin offered a visible
// graveyard card, which the mapping asked for a look id and the seat forfeited
// as agent_error. The pair must reach its result with both lanes finished.
func TestOutcomePair134NoForfeit(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 61, 200)
	if len(jobs) <= 134 {
		t.Fatalf("seed 61 has %d pairs, want at least 135", len(jobs))
	}
	j := jobs[134]
	if j.seed != 5887574459431277129 {
		t.Fatalf("pair 134 seed = %d, want 5887574459431277129", j.seed)
	}
	if len(j.decks) != 2 || len(j.decks[0].Cards) == 0 || len(j.decks[1].Cards) == 0 {
		t.Fatalf("pair 134 decks are empty: %+v", j.decks)
	}
	o := sbOptions{mode: "outcome", policy: "bot", autopay: true, maxTurns: 100, maxIntents: 20000, maxObjects: 20000}
	native := playSBLane(reg, j.decks, j.seed, o, []bool{false, false})
	if native.err != "" || !native.done {
		t.Fatalf("native lane did not finish: err=%q done=%v stall=%q", native.err, native.done, native.stall)
	}
	v2 := playSBLane(reg, j.decks, j.seed, o, []bool{true, false})
	if v2.err != "" {
		t.Fatalf("v2 lane error: %s", v2.err)
	}
	if v2.forfeit[0] != "" {
		t.Fatalf("v2 seat 0 forfeited %s on the game native finishes", v2.forfeit[0])
	}
	if !v2.done {
		t.Fatalf("v2 lane did not finish: stall=%q", v2.stall)
	}
}
