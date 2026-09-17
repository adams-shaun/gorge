package searchprobe

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/view"
)

func samplingHistory(t *testing.T) (PublicGame, History) {
	t.Helper()
	e := observationEngine(t, 17)
	c := NewCollector(0)
	h := History{Actor: 0, Answers: make(map[int][]Action)}
	r := rand.New(rand.NewPCG(7, 19))
	start := 0
	for i := 0; i < 50; i++ {
		frame, err := c.Capture(e, e.L.Events[start:])
		if err != nil {
			t.Fatal(err)
		}
		h.Frames = append(h.Frames, frame)
		d := e.Pending()
		if i >= 20 && d.Player == 0 {
			break
		}
		in := botpolicy.Decide(botpolicy.BoardFromGame(e.G, e, d.Player), d, r)
		if d.Player == 0 {
			h.Answers[i], err = c.Actions(d, in)
			if err != nil {
				t.Fatal(err)
			}
		}
		start = len(e.L.Events)
		if err := e.SubmitHypothetical(in); err != nil {
			t.Fatal(err)
		}
	}
	if h.Frames[len(h.Frames)-1].Decision == nil {
		t.Fatal("fixture failed to select actor root")
	}
	// This is the fixture's PUBLIC deck composition, not an extraction of its
	// hidden zones. All cards are identical in the synthetic experiment.
	definition, ds := cards.ParseBytes("public-fixture", []byte("Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n"))
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	definition.Link()
	for _, f := range definition.Faces {
		f.ApplyIntrinsics()
	}
	deck := make([]*cards.Card, 20)
	for i := range deck {
		deck[i] = definition
	}
	return PublicGame{Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, deck}}, h
}

func TestSamplerClassifiesContradictoryAndUnsupportedInput(t *testing.T) {
	setup, h := samplingHistory(t)
	h.Frames[0].Identities[0].Name = "Not In Public Deck"
	_, err := Sample(setup, h, SampleOptions{Seed: 1, Attempts: 4, Worlds: 4, MaxSubmits: 5000})
	var failure *Failure
	if !errors.As(err, &failure) || failure.Kind != "contradictory" {
		t.Fatalf("contradiction: %v", err)
	}
	setup, h = samplingHistory(t)
	setup.Decks[0] = setup.Decks[0][:6]
	_, err = Sample(setup, h, SampleOptions{Seed: 1, Attempts: 4, Worlds: 4, MaxSubmits: 5000})
	if !errors.As(err, &failure) || failure.Kind != "unsupported" {
		t.Fatalf("unsupported: %v", err)
	}
}

func TestSamplerReconstructsFullPrefixAndReplaysWorlds(t *testing.T) {
	setup, h := samplingHistory(t)
	result, err := Sample(setup, h, SampleOptions{Seed: 991, Attempts: 8, Worlds: 4, MaxSubmits: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Worlds) != 4 || result.Accepted != 8 || result.ESS < 7.99 {
		t.Fatalf("sampling result: %+v", result)
	}
	for _, w := range result.Worlds {
		e, err := rules.NewHypothetical(w.Config, w.Engine.ChanceTranscript())
		if err != nil {
			t.Fatal(err)
		}
		if err := e.AdvanceHypothetical(); err != nil {
			t.Fatal(err)
		}
		for _, in := range w.Engine.L.Intents {
			if err := e.SubmitHypothetical(in); err != nil {
				t.Fatal(err)
			}
		}
		if e.L.Head() != w.Engine.L.Head() || e.RNGDraws() != w.Engine.RNGDraws() {
			t.Fatal("selected world replay failed")
		}
	}
}

func TestSamplerRejectsChangedPrefixAndHonorsSubmitBudget(t *testing.T) {
	setup, h := samplingHistory(t)
	var board view.View
	if err := json.Unmarshal(h.Frames[3].Board, &board); err != nil {
		t.Fatal(err)
	}
	board.Players[0].Life = 999
	h.Frames[3].Board, _ = json.Marshal(board)
	result, err := Sample(setup, h, SampleOptions{Seed: 991, Attempts: 8, Worlds: 4, MaxSubmits: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Worlds) != 0 || result.Accepted != 0 || result.PrefixRejected != 8 {
		t.Fatalf("accepted impossible prefix: %+v", result)
	}
	_, h = samplingHistory(t)
	result, err = Sample(setup, h, SampleOptions{Seed: 991, Attempts: 8, Worlds: 4, MaxSubmits: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Worlds) != 0 || result.BudgetExhausted != 8 || result.Submits != 8 {
		t.Fatalf("ignored work cap: %+v", result)
	}
}

func TestSamplerUsesOnlyHistoryAndExplicitSeed(t *testing.T) {
	setup, h := samplingHistory(t)
	before, _ := json.Marshal(h)
	opts := SampleOptions{Seed: 991, Attempts: 8, Worlds: 4, MaxSubmits: 5000}
	a, err := Sample(setup, h, opts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Sample(setup, h, opts)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(h)
	if string(before) != string(after) {
		t.Fatal("sampler modified history")
	}
	if len(a.Worlds) != len(b.Worlds) || len(a.Worlds) != 4 {
		t.Fatal("nondeterministic or empty sampling")
	}
	for i := range a.Worlds {
		if a.Worlds[i].Engine.L.Head() != b.Worlds[i].Engine.L.Head() || !reflect.DeepEqual(a.Worlds[i].Engine.ChanceTranscript(), b.Worlds[i].Engine.ChanceTranscript()) {
			t.Fatal("sample not deterministic")
		}
	}
}

func TestResampledDuplicateWorldsAreIndependent(t *testing.T) {
	setup, h := samplingHistory(t)
	for seed := uint64(0); seed < 20; seed++ {
		result, err := Sample(setup, h, SampleOptions{Seed: seed, Attempts: 4, Worlds: 4, MaxSubmits: 5000})
		if err != nil {
			t.Fatal(err)
		}
		if result.Duplicates == 0 {
			continue
		}
		for i, a := range result.Worlds {
			for j, b := range result.Worlds {
				if i != j && (a.Engine == b.Engine || a.Observer == b.Observer) {
					t.Fatal("resampled worlds alias")
				}
			}
		}
		return
	}
	t.Fatal("fixture never selected a duplicate")
}
