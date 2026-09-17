package searchprobe

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
)

func TestExperimentRealDeckRootReplayAndDeterminism(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{"death-n-taxes", "dimir-tempo"}
	decks := make([][]*cards.Card, 2)
	for i, n := range names {
		var err error
		decks[i], err = testutil.LoadRepoDeck(reg, n)
		if err != nil {
			t.Fatal(err)
		}
	}
	setup := PublicGame{Names: names, Decks: decks, Tokens: reg.Tokens}
	opts := ExperimentOptions{Seed: 10000, SampleSeed: 54321, Attempts: 4, Worlds: 4, MaxSubmits: 5000}
	a := RunExperiment(setup, opts)
	if a.Error != "" || !a.BaselineReplay || a.RootAt < 0 || len(a.Candidates) < 2 || len(a.Outcomes) != 4 {
		t.Fatalf("experiment %+v", a)
	}
	b := RunExperiment(setup, opts)
	a.SampleNS = 0
	b.SampleNS = 0
	a.SearchNS = 0
	b.SearchNS = 0
	if !reflect.DeepEqual(a, b) {
		t.Fatal("experiment depends on ambient state")
	}
}

func TestExperimentClockChangesOnlyDiagnostics(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{"death-n-taxes", "dimir-tempo"}
	decks := make([][]*cards.Card, 2)
	for i, n := range names {
		var err error
		decks[i], err = testutil.LoadRepoDeck(reg, n)
		if err != nil {
			t.Fatal(err)
		}
	}
	setup := PublicGame{Names: names, Decks: decks, Tokens: reg.Tokens}
	opts := ExperimentOptions{Seed: 10000, SampleSeed: 54321, Attempts: 4, Worlds: 4, MaxSubmits: 5000}
	want := RunExperiment(setup, opts)
	ticks := []int64{10, 30, 100, 145}
	next := 0
	opts.Clock = func() int64 {
		if next >= len(ticks) {
			t.Fatal("clock called more than four times")
		}
		v := ticks[next]
		next++
		return v
	}
	got := RunExperiment(setup, opts)
	if next != 4 || got.SampleNS != 20 || got.SearchNS != 45 {
		t.Fatalf("clock diagnostics: calls=%d sample=%d search=%d", next, got.SampleNS, got.SearchNS)
	}
	got.SampleNS, got.SearchNS = 0, 0
	if !reflect.DeepEqual(got, want) {
		t.Fatal("diagnostic clock changed experiment behavior")
	}
}
