package main

import (
	"io"
	"strings"
	"testing"
)

func TestRefusesHeldOutSeedRange(t *testing.T) {
	for _, seed := range []string{"1000000", "999999", "1500000"} {
		err := run([]string{"-seed", seed, "-games", "5", "-cards", "/nonexistent"}, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "held-out") {
			t.Fatalf("seed %s: want held-out refusal, got %v", seed, err)
		}
	}
}

// The per-kind split follows every pre-existing line, one census per kind in
// sorted order, with that kind's own fallbacks.
func TestSummaryPerKindLines(t *testing.T) {
	recs := []GameRecord{
		{Pair: "a:b", SearchOver: true, BaseOver: true, Decisions: []DecisionRecord{
			{Kind: "blockers", Covered: true, ChosenDiffersFromBot: true},
			{Kind: "blockers", Covered: true},
			{Kind: "blockers", Fallback: "insufficient worlds/ESS"},
			{Kind: "attackers", Covered: true},
			{Kind: "attackers", Fallback: "teacher error: x"},
		}},
	}
	var b strings.Builder
	summarize(&b, recs, config{kinds: map[string]bool{"attackers": true, "blockers": true}}, 30_000_000, 1, 0)
	out := b.String()
	want := "kind attackers: asked 2, covered 1 (50.0%), overrides 0 (0.0% of covered)\n" +
		"  kind attackers fallback \"teacher error: x\": 1\n" +
		"kind blockers: asked 3, covered 2 (66.7%), overrides 1 (50.0% of covered)\n" +
		"  kind blockers fallback \"insufficient worlds/ESS\": 1\n"
	if !strings.HasSuffix(out, want) {
		t.Fatalf("summary does not end with the per-kind lines %q:\n%s", want, out)
	}
	if !strings.Contains(out, "decisions asked 5, covered 3 (60.0%), overrides 1 (33.3% of covered)\n") {
		t.Fatalf("aggregate line changed:\n%s", out)
	}
}

func TestPairedDeltaSummary(t *testing.T) {
	recs := []GameRecord{
		{Pair: "a:b", SearchOver: true, BaseOver: true, SearchWon: true},
		{Pair: "a:b", SearchOver: true, BaseOver: true, BaseWon: true},
		{Pair: "a:b", SearchOver: true, BaseOver: true, SearchWon: true, BaseWon: true},
		{Pair: "a:b", SearchOver: true, BaseOver: true},
	}
	var b strings.Builder
	summarize(&b, recs, config{kinds: map[string]bool{"attackers": true}}, 30_000_000, 4, 0)
	out := b.String()
	for _, want := range []string{"search seat vs bot:   50.0%", "paired delta:          +0.00pp", "2 games changed outcome"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q:\n%s", want, out)
		}
	}
}
