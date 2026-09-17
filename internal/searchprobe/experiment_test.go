package searchprobe

import (
	"reflect"
	"strings"
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

func TestExperimentContradictionRetainsBaselineAndFallbackOutcomes(t *testing.T) {
	// The rules can run this public fixture, but its declared nameless card
	// contradicts the sampler's observable card-fact contract at the first draw.
	card := syntheticCard(t, "Name:Fixture\nManaCost:0\nTypes:Sorcery\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:Fixture.\n")
	card.Faces[0].Name = ""
	setup := PublicGame{Names: []string{"a", "b"}, Decks: [][]*cards.Card{repeatCard(card, 12), repeatCard(card, 12)}}
	got := RunExperiment(setup, ExperimentOptions{Seed: 10000, SampleSeed: 54321, Attempts: 4, Worlds: 4, MaxSubmits: 500})
	if got.Error != "" || !got.BaselineReplay || got.RootAt < 0 || len(got.Outcomes) != 4 {
		t.Fatalf("contradiction discarded baseline/outcomes: error=%q replay=%v root=%d outcomes=%d", got.Error, got.BaselineReplay, got.RootAt, len(got.Outcomes))
	}
	if len(got.Sampling.Worlds) != 0 || got.Sampling.Accepted != 0 || !strings.Contains(got.FourWorld.Fallback, "contradictory") || got.OneWorld.Fallback != got.FourWorld.Fallback {
		t.Fatalf("contradiction fallback not distinct: one=%q four=%q", got.OneWorld.Fallback, got.FourWorld.Fallback)
	}
	for _, outcome := range got.Outcomes {
		if !outcome.Replay || !outcome.Terminal {
			t.Fatalf("unverified fallback outcome: %+v", outcome)
		}
	}
	// Invalid sampling configuration is still fatal, not a population fallback.
	card.Faces[0].Name = "Fixture"
	bad := RunExperiment(setup, ExperimentOptions{Seed: 10000, SampleSeed: 54321, Attempts: 0, Worlds: 4, MaxSubmits: 500})
	if !strings.Contains(bad.Error, "invariant") || bad.BaselineReplay {
		t.Fatalf("invariant swallowed as fallback: %+v", bad)
	}
}
