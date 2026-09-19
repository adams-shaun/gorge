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
